package migrator

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	_ "modernc.org/sqlite"
)

type MigrationStatus int64

const (
	StatusPending MigrationStatus = iota
	StatusMigrating
	StatusMigrated
	StatusFailed
)

var ErrNoPendingEmails = errors.New("no pending emails")
var ErrClaimLimitReached = errors.New("claim limit reached")

type MigrationManager struct {
	db             *sql.DB
	targetAddress  string
	destination    string
	MaxAttempts    int
	migrationFlags MigrationFlag
	limit          int64
	claimCount     atomic.Int64
	logger         *slog.Logger
	runID          string
}

type Email struct {
	ID                     int64
	MessageID              string
	Filename               string
	FileHash               []byte
	Sender                 string
	Destination            string
	Date                   time.Time
	MigrationTargetAddress string
}

func NewMigrationManager(targetAddress string, destination string, maxAttempts int, logger *slog.Logger) (*MigrationManager, error) {
	db, err := sql.Open("sqlite", utils.MANAGER_DB_PATH)
	if err != nil {
		return nil, fmt.Errorf("failed to setup database (open): %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(
		context.Background(),
		"PRAGMA busy_timeout = 5000;",
	); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure SQLite busy timeout: %w", err)
	}

	if strings.TrimSpace(targetAddress) == "" {
		return nil, errors.New("target address cannot be empty")
	}

	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	return &MigrationManager{
		db:             db,
		MaxAttempts:    maxAttempts,
		targetAddress:  strings.TrimSpace(targetAddress),
		destination:    strings.TrimSpace(destination),
		migrationFlags: MigrationFlagEmpty,
		logger:         logger.With("component", "migration-manager"),
		runID:          utils.GetRunID(),
	}, nil
}

// Setters for Migration Manager
func (m *MigrationManager) SetFromFlags(fields MigrationFlag) error {
	if err := m.migrationFlags.Validate(); err != nil {
		return fmt.Errorf("flag validation: %w", err)
	}
	m.migrationFlags = fields
	return nil
}

func (m *MigrationManager) SetLimit(l int) error {
	if l < 0 {
		return errors.New("limit cannot be negative")
	}

	m.limit = int64(l)
	m.claimCount.Store(0)
	return nil
}

// Type Methods
func (m *MigrationManager) CountEligible(ctx context.Context) (int64, error) {
	const query = `
		SELECT COUNT(*)
		FROM emails
		WHERE migration_target_address = @target
		  	AND (@destination = '' OR dest = @destination)
			AND (
				NOT @local_enforce
				OR (
					migration_status in (@pending, @failed)
					AND retry_count < @max_attempts
				)
			)
			AND (
				last_claim_run_id IS NULL
				OR last_claim_run_id <> @run_id
			)
	`

	var count int64
	err := m.db.QueryRowContext(
		ctx,
		query,
		sql.Named("target", m.targetAddress),
		sql.Named("destination", m.destination),
		sql.Named("local_enforce", m.migrationFlags.Has(MigrationFlagLocalEnforce)),
		sql.Named("pending", StatusPending),
		sql.Named("failed", StatusFailed),
		sql.Named("max_attempts", m.MaxAttempts),
		sql.Named("run_id", m.runID),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count eligible emails: %w", err)
	}

	if m.limit > 0 && count > m.limit {
		count = m.limit
	}

	return count, nil
}

func (m *MigrationManager) ClaimNext(ctx context.Context) (Email, error) {
	// claim limit guard
	if !m.reserveClaim() {
		return Email{}, ErrClaimLimitReached
	}

	orderDirection := m.migrationFlags.GetOrderString()

	query := fmt.Sprintf(`
	UPDATE emails
	SET 
		migration_status = @migrating,
		last_claim_run_id = @run_id
	WHERE id = (
		SELECT id
		FROM emails
		WHERE migration_target_address = @target
			AND (@destination = '' OR dest = @destination)
			AND (
				NOT @local_enforce
				OR (
					migration_status in (@pending, @failed)
					AND retry_count < @max_attempts
				)
			)
			AND (
				last_claim_run_id IS NULL
				OR last_claim_run_id <> @run_id
			)
		ORDER BY date %s, id %s
		LIMIT 1
	)
	RETURNING
		id,
		message_id,
		filename,
		file_hash,
		sender,
		dest,
		migration_target_address;
	`, orderDirection, orderDirection)

	var email Email

	err := m.db.QueryRowContext(
		ctx,
		query,
		sql.Named("migrating", StatusMigrating),
		sql.Named("pending", StatusPending),
		sql.Named("failed", StatusFailed),
		sql.Named("max_attempts", m.MaxAttempts),
		sql.Named("target", m.targetAddress),
		sql.Named("destination", m.destination),
		sql.Named("run_id", m.runID),
		sql.Named("local_enforce", m.migrationFlags.Has(MigrationFlagLocalEnforce)),
	).Scan(
		&email.ID,
		&email.MessageID,
		&email.Filename,
		&email.FileHash,
		&email.Sender,
		&email.Destination,
		&email.MigrationTargetAddress,
	)

	if err != nil {
		// claim release on error
		m.releaseClaim()

		if errors.Is(err, sql.ErrNoRows) {
			return Email{}, ErrNoPendingEmails
		}

		return Email{}, fmt.Errorf("claim next email: %w", err)
	}

	return email, nil
}

func (m *MigrationManager) MarkMigrated(ctx context.Context, id int64) error {
	const query = `
	UPDATE emails
	SET migration_status = ?
	WHERE id = ?
	  AND migration_status = ?;
	`

	result, err := m.db.ExecContext(ctx, query, StatusMigrated, id, StatusMigrating)
	if err != nil {
		return fmt.Errorf("mark email %d migrated: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"check mark-migrated result for email %d: %w",
			id,
			err,
		)
	}

	if affected != 1 {
		return fmt.Errorf(
			"cannot mark email %d migrated: email is not migrating (affected: %d)",
			id,
			affected,
		)
	}

	return nil
}

func (m *MigrationManager) MarkFailed(
	ctx context.Context,
	id int64,
) error {
	const query = `
		UPDATE emails
		SET migration_status = ?,
		    retry_count = retry_count + 1
		WHERE id = ?
		  AND migration_status = ?;
	`

	result, err := m.db.ExecContext(
		ctx,
		query,
		StatusFailed,
		id,
		StatusMigrating,
	)
	if err != nil {
		return fmt.Errorf("mark email %d failed: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"check mark-failed result for email %d: %w",
			id,
			err,
		)
	}

	if affected != 1 {
		return fmt.Errorf(
			"cannot mark email %d failed: email is not migrating",
			id,
		)
	}

	return nil
}

var ErrDuplicateEmail = errors.New(
	"email already exists for this destination and migration target",
)

func (m *MigrationManager) AddEmail(
	ctx context.Context,
	email Email,
) (int64, error) {
	if len(email.FileHash) != sha256.Size {
		return 0, fmt.Errorf(
			"invalid SHA-256 hash length: got %d bytes, expected %d",
			len(email.FileHash),
			sha256.Size,
		)
	}

	const query = `
		INSERT INTO emails (
			message_id,
			filename,
			file_hash,
			sender,
			dest,
			date,
			migration_target_address,
			migration_status,
			retry_count
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT (
			file_hash,
			dest,
			migration_target_address
		)
		DO NOTHING
		RETURNING id;
	`

	var id int64

	err := m.db.QueryRowContext(
		ctx,
		query,
		email.MessageID,
		email.Filename,
		email.FileHash,
		email.Sender,
		email.Destination,
		email.Date.Format(time.RFC3339Nano),
		email.MigrationTargetAddress,
		StatusPending,
	).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrDuplicateEmail
	}
	if err != nil {
		return 0, fmt.Errorf(
			"insert email %q: %w",
			email.Filename,
			err,
		)
	}

	return id, nil
}

func (m *MigrationManager) RecoverInterrupted(
	ctx context.Context,
) (int64, error) {
	const query = `
		UPDATE emails
		SET migration_status = ?,
		    retry_count = retry_count + 1
		WHERE migration_status = ?;
	`

	result, err := m.db.ExecContext(
		ctx,
		query,
		StatusFailed,
		StatusMigrating,
	)
	if err != nil {
		return 0, fmt.Errorf("recover interrupted migrations: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get recovered migration count: %w", err)
	}

	return affected, nil
}

func (m *MigrationManager) Close() error {
	return m.db.Close()
}

func (m *MigrationManager) reserveClaim() bool {
	if m.limit == 0 {
		return true
	}

	// compare-and-swap retry pattern
	for {
		current := m.claimCount.Load()
		if current >= m.limit {
			return false
		}

		if m.claimCount.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (m *MigrationManager) releaseClaim() {
	if m.limit > 0 {
		m.claimCount.Add(-1)
	}
}

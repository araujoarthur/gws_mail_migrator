package migrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/araujoarthur/gws_mail_migrator/lib/mailinserter"
)

type EmailInserter interface {
	InsertRawEML(ctx context.Context, content io.Reader) (mailinserter.InsertResult, error)
}

type EmailExistenceChecker interface {
	EmailExists(ctx context.Context, messageID string) (bool, error)
}

type ProgressReporter interface {
	Add(int) error
}

type Migrator struct {
	manager            *MigrationManager
	inserter           EmailInserter
	checker            EmailExistenceChecker
	emlFolder          string
	logger             *slog.Logger
	progress           ProgressReporter
	insertedCount      atomic.Int64
	alreadyExistsCount atomic.Int64
}

// NewMigrator returns a pointer to a Migrator allocation. The inserter and checker separated fields exist to account for groups vs. users insertions.
// in the scenario of an user migration, the checker should be the same instance as the inserter.
func NewMigrator(
	manager *MigrationManager,
	inserter EmailInserter,
	checker EmailExistenceChecker,
	emlFolder string,
	logger *slog.Logger,
	progress ProgressReporter,
) *Migrator {
	return &Migrator{
		manager:   manager,
		inserter:  inserter,
		checker:   checker,
		emlFolder: emlFolder,
		logger:    logger.With("component", "migrator"),
		progress:  progress,
	}
}

func (m *Migrator) Close() error {
	return m.manager.Close()
}

func (m *Migrator) InsertedCount() int64 {
	return m.insertedCount.Load()
}

func (m *Migrator) AlreadyExistsCount() int64 {
	return m.alreadyExistsCount.Load()
}

func (m *Migrator) MigratedCount() int64 {
	return m.InsertedCount() + m.AlreadyExistsCount()
}

// reportOutcome provides the means to report progress after a successful operation
func (m *Migrator) reportOutcome(outcome MigrationOutcome) error {
	switch outcome {
	case MigrationOutcomeInserted:
		m.insertedCount.Add(1)

	case MigrationOutcomeAlreadyExists:
		m.alreadyExistsCount.Add(1)

	default:
		return fmt.Errorf("cannot report migration outcome %d", outcome)
	}

	if m.progress != nil {
		if err := m.progress.Add(1); err != nil {
			m.logger.Warn("failed to update progress bar", "error", err)
		}
	}

	return nil
}

// migrateEmail provides the logic for the unit of work of migrating one claimed email.
func (m *Migrator) migrateEmail(ctx context.Context, email Email) (MigrationOutcome, error) {
	if m.manager.migrationFlags.Has(MigrationFlagModeDelta) {
		exists, err := m.checker.EmailExists(ctx, email.MessageID)
		if err != nil {
			m.logger.Error("failed to migrate email", "step", "check_email_existence", "error", err)
			return MigrationOutcomeUnknown, fmt.Errorf("check whether email exists remotely: %w", err)
		}

		if exists {
			m.logger.Info("email already exists on the mailbox", "email_id", email.ID, "message_id", email.MessageID)

			return MigrationOutcomeAlreadyExists, nil
		}
	}

	// Reaches here if either there's no delta import flag or the email in fact does not exist in the mailbox.
	filename := filepath.Clean(email.Filename)
	root := filepath.Clean(m.emlFolder)

	prefix := root + string(filepath.Separator)

	filename, _ = strings.CutPrefix(filename, prefix)

	if !filepath.IsLocal(filename) {
		return MigrationOutcomeUnknown, fmt.Errorf("unsafe email filename %q", email.Filename)
	}

	path := filepath.Join(root, filename)

	file, err := os.Open(path)
	if err != nil {
		return MigrationOutcomeUnknown, fmt.Errorf("open email %q: %w", email.Filename, err)
	}
	defer file.Close()

	result, err := m.inserter.InsertRawEML(ctx, file)
	if err != nil {
		return MigrationOutcomeUnknown, fmt.Errorf("insert email %q: %w", email.Filename, err)
	}

	m.logger.Info(
		"inserted local email",
		"email_id", email.ID,
		"result_id", result.ID,
		"result_thread_id", result.ThreadID,
		"label_ids", result.LabelIDs,
		"target", email.MigrationTargetAddress,
	)

	return MigrationOutcomeInserted, nil
}

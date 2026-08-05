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

	"github.com/araujoarthur/gws_mail_migrator/lib/impersonator"
)

type EmailInserter interface {
	InsertRawEML(ctx context.Context, content io.Reader) (impersonator.InsertResult, error)
}

type ProgressReporter interface {
	Add(int) error
}

type Migrator struct {
	manager      *MigrationManager
	inserter     EmailInserter
	emlFolder    string
	logger       *slog.Logger
	progress     ProgressReporter
	successCount atomic.Int64
}

func NewMigrator(
	manager *MigrationManager,
	inserter EmailInserter,
	emlFolder string,
	logger *slog.Logger,
	progress ProgressReporter,
) *Migrator {
	return &Migrator{
		manager:   manager,
		inserter:  inserter,
		emlFolder: emlFolder,
		logger:    logger.With("component", "migrator"),
		progress:  progress,
	}
}

func (m *Migrator) Close() error {
	return m.manager.Close()
}

// reportProgress provides the means to report progress after a successful operation
func (m *Migrator) reportSuccess() {
	m.successCount.Add(1)

	if m.progress != nil {
		m.progress.Add(1)
	}
}

// migrateEmail provides the logic for the unit of work of migrating one claimed email.
func (m *Migrator) migrateEmail(ctx context.Context, email Email) error {
	filename := filepath.Clean(email.Filename)
	root := filepath.Clean(m.emlFolder)

	prefix := root + string(filepath.Separator)

	filename, _ = strings.CutPrefix(filename, prefix)

	if !filepath.IsLocal(filename) {
		return fmt.Errorf("unsafe email filename %q", email.Filename)
	}

	path := filepath.Join(root, filename)

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open email %q: %w", email.Filename, err)
	}
	defer file.Close()

	result, err := m.inserter.InsertRawEML(ctx, file)
	if err != nil {
		return fmt.Errorf("insert email %q: %w", email.Filename, err)
	}

	m.logger.Info(
		"gmail inserted local email",
		"email_id", email.ID,
		"result_id", result.ID,
		"result_thread_id", result.ThreadID,
		"label_ids", result.LabelIDs,
		"target", email.MigrationTargetAddress,
	)

	return nil
}

func (m *Migrator) SuccessCount() int64 {
	return m.successCount.Load()
}

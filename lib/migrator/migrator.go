package migrator

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/impersonator"
)

type EmailInserter interface {
	InsertRawEML(ctx context.Context, content io.Reader) (impersonator.InsertResult, error)
}

type Migrator struct {
	manager   *MigrationManager
	inserter  EmailInserter
	emlFolder string
	logger    *log.Logger
}

func NewMigrator(
	manager *MigrationManager,
	inserter EmailInserter,
	emlFolder string,
	logger *log.Logger,
) *Migrator {
	return &Migrator{
		manager:   manager,
		inserter:  inserter,
		emlFolder: emlFolder,
		logger:    logger,
	}
}

func (m *Migrator) Close() error {
	return m.manager.Close()
}

// Migrate ONE claimed email logic
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

	m.logger.Printf(
		"Gmail inserted local email %d: gmail_id=%s thread_id=%s labels=%v target=%s",
		email.ID,
		result.ID,
		result.ThreadID,
		result.LabelIDs,
		email.MigrationTargetAddress,
	)

	return nil
}

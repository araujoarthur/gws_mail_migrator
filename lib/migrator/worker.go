package migrator

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

func (m *Migrator) runWorker(ctx context.Context, workerID int) error {
	for {
		email, err := m.manager.ClaimNext(ctx)

		if errors.Is(err, ErrNoPendingEmails) || errors.Is(err, ErrClaimLimitReached) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("worker %d claim email: %w", workerID, err)
		}

		outcome, err := m.migrateEmail(ctx, email)
		if err != nil {
			markErr := m.manager.MarkFailed(ctx, email.ID)
			if markErr != nil {
				return errors.Join(
					err,
					fmt.Errorf(
						"worker %d mark email %d failed: %w",
						workerID,
						email.ID,
						markErr,
					),
				)
			}

			if m.logger != nil {
				m.logger.Warn(
					"worker failed to insert email",
					"worker_id", workerID,
					"email_id", email.ID,
					"error", err,
				)
			}

			// Continue so another eligible email can be processed.
			continue
		}

		m.logger.Info("insertion exited successfully", "outcome", outcome, "outcome_string", outcome.StringRepr())

		// Successfully migrated, marking on database
		if err := m.manager.MarkMigrated(ctx, email.ID); err != nil {
			return fmt.Errorf(
				"worker %d mark email %d migrated: %w",
				workerID,
				email.ID,
				err,
			)
		}

		if err := m.reportOutcome(outcome); err != nil {
			return fmt.Errorf("report outcome for email %d: %w", email.ID, err)
		}

		if m.logger != nil {
			m.logger.Info(
				"worker migrated email",
				"worker_id", workerID,
				"email_id", email.ID,
				"email_filename", email.Filename,
			)
		}
	}
}

func (m *Migrator) Run(ctx context.Context, workerCount int) error {
	if workerCount < 1 {
		return fmt.Errorf("worker count must be at least 1")
	}

	group, workerCtx := errgroup.WithContext(ctx)

	for workerID := 1; workerID <= workerCount; workerID++ {
		id := workerID
		group.Go(func() error {
			return m.runWorker(workerCtx, id)
		})
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("migration workers: %w", err)
	}

	return nil
}

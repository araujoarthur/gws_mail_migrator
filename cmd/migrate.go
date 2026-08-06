package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/araujoarthur/gws_mail_migrator/lib/impersonator"
	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var ErrNoEligibleEmails = errors.New("no eligible emails")

var successCount atomic.Int64

type migrateOptions struct {
	targetAddress string
	destination   string
	maxAttempts   int
	limit         int
	limitSet      bool
	order         migrator.DateOrder
	orderRaw      string
	workers       int
	verbosity     bool
}

func newMigrateCommand() *cobra.Command {
	var options migrateOptions

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate pending emails to a target mailbox",
		Args:  cobra.NoArgs,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			options.targetAddress = strings.TrimSpace(
				options.targetAddress,
			)
			options.destination = strings.TrimSpace(
				options.destination,
			)

			// target validation
			if options.targetAddress == "" {
				return fmt.Errorf("target address cannot be empty")
			}

			// worker count validation
			if options.workers < 1 {
				return fmt.Errorf("workers must be at least 1")
			}

			// max attempts validation
			if options.maxAttempts < 1 {
				return fmt.Errorf("max attempts must be at least 1")
			}

			// limit validation
			options.limitSet = cmd.Flags().Changed("limit")

			if options.limitSet && options.limit < 1 {
				return fmt.Errorf("limit must be at least 1")
			}

			// order validation
			options.orderRaw = strings.ToLower(
				strings.TrimSpace(options.orderRaw),
			)

			switch options.orderRaw {
			case "oldest", "newest":
				dateOrder := migrator.DateOrderOldestFirst
				if options.orderRaw == "newest" {
					dateOrder = migrator.DateOrderNewestFirst
				}

				options.order = dateOrder
			default:
				return fmt.Errorf(
					"invalid order %q: expected oldest or newest",
					options.orderRaw,
				)
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigration(cmd.Context(), options)
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&options.targetAddress,
		"target",
		"",
		"mailbox that will receive the migrated emails",
	)

	flags.StringVar(
		&options.destination,
		"dest",
		"",
		"only migrate emails with this original destination",
	)

	flags.StringVar(
		&options.orderRaw,
		"order",
		"newest",
		"migration order: 'newest' or 'oldest'",
	)

	flags.IntVarP(
		&options.workers,
		"workers",
		"w",
		4,
		"number of concurrent migration workers",
	)

	flags.IntVar(
		&options.maxAttempts,
		"max-attempts",
		5,
		"maximum number of attempts for each entry",
	)

	flags.IntVar(
		&options.limit,
		"limit",
		0,
		"maximum number of emails to process",
	)

	flags.BoolVarP(
		&options.verbosity,
		"verbosity",
		"v",
		false,
		"enable logging verbosity",
	)

	if err := cmd.MarkFlagRequired("target"); err != nil {
		panic(err)
	}

	return cmd
}

func runMigration(ctx context.Context, options migrateOptions) error {
	logger, loggerCloser, err := logging.New(utils.LOG_FILE_PATH, options.verbosity)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer loggerCloser()

	commandLogger := logger.With("command", "migrate")
	commandLogger.Debug("migrate ran with options",
		"destination", options.destination,
		"limit", options.limit,
		"limit_set", options.limitSet,
		"max_attempts", options.maxAttempts,
		"order", options.order,
		"order_raw", options.orderRaw,
		"target_address", options.targetAddress,
		"verbosity", options.verbosity,
		"workers", options.workers,
	)

	manager, err := migrator.NewMigrationManager(options.targetAddress, options.destination, options.maxAttempts, commandLogger)
	if err != nil {
		return fmt.Errorf("create migration manager: %w", err)
	}
	defer manager.Close()

	// Setting manager options
	manager.SetDateOrder(options.order)

	if err := manager.SetLimit(options.limit); err != nil {
		return fmt.Errorf("set limit: %w", err)
	}

	total, err := manager.CountEligible(ctx)
	if err != nil {
		return fmt.Errorf("counting eligibles: %w", err)
	}

	if total == 0 {
		return ErrNoEligibleEmails
	}

	// Creating impersonator
	imp, err := impersonator.NewImpersonator(
		utils.CREDENTIALS_PATH,
		options.targetAddress,
		impersonator.GmailInsertScope+" "+impersonator.GmailReadOnlyScope,
		commandLogger,
	)
	if err != nil {
		return fmt.Errorf("create the impersonator: %w", err)
	}

	var reporter migrator.ProgressReporter
	var bar *progressbar.ProgressBar
	if !options.verbosity {
		bar = progressbar.NewOptions64(
			total,
			progressbar.OptionSetDescription("Migrating emails"),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(30),
			progressbar.OptionSetWriter(os.Stdout),
		)

		reporter = bar
	}

	migr := migrator.NewMigrator(manager, imp, utils.EMAILS_ROOT_PATH, logger, reporter)

	runErr := migr.Run(
		ctx,
		options.workers,
	)

	succeeded := migr.SuccessCount()
	// removes the progress display before printing the summary.
	if bar != nil {
		if succeeded == total {
			_ = bar.Finish()
		}

		// Move subsequent output onto a clean line.
		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintf(
		os.Stdout,
		"Migration complete: %d of %d emails succeeded\n",
		succeeded,
		total,
	)

	if runErr != nil {
		commandLogger.Error("failed during migration run", "error", err)
		return fmt.Errorf("run migration: %w", err)
	}

	commandLogger.Info("migration completed", "success_count", migr.SuccessCount(), "total", total)
	return nil
}

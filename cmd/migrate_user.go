package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/mailinserter"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type migrateUserOptions struct {
	targetAddress  string
	destination    string
	maxAttempts    int
	limit          int
	limitSet       bool
	orderRaw       string
	workers        int
	verbosity      bool
	deltaMigration bool
	localEnforce   bool
	dryRun         bool
	testRun        bool
	migrationFlags migrator.MigrationFlag
}

func newMigrateUserCommand() *cobra.Command {
	var options migrateUserOptions

	cmd := &cobra.Command{
		Use:   "user",
		Short: "Migrate pending emails to a target user's mailbox",
		Long: `
			This command is used to migrate multiple emails conditionally to a user's mailbox. The mais difference between this and the 'group' command is that groups do not support parallel insertion.
		`,
		Args: cobra.NoArgs,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			options.targetAddress = strings.TrimSpace(
				strings.ToLower(options.targetAddress),
			)
			options.destination = strings.TrimSpace(
				strings.ToLower(options.destination),
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

			// Flags Build Up
			options.migrationFlags = migrator.MigrationFlagEmpty

			options.migrationFlags.Set(migrator.MigrationFlagTargetUser)
			options.migrationFlags.Unset(migrator.MigrationFlagTargetGroup)

			if options.dryRun {
				options.migrationFlags.Set(migrator.MigrationFlagDryRun)
			}

			if options.deltaMigration {
				options.migrationFlags.Set(migrator.MigrationFlagModeDelta)
				if options.localEnforce {
					options.migrationFlags.Set(migrator.MigrationFlagLocalEnforce)
				} else {
					options.migrationFlags.Unset(migrator.MigrationFlagLocalEnforce)
				}
			} else {
				options.migrationFlags.SetN(migrator.MigrationFlagModeStandard, migrator.MigrationFlagLocalEnforce)
			}

			// order validation
			options.orderRaw = strings.ToLower(
				strings.TrimSpace(options.orderRaw),
			)

			switch options.orderRaw {
			case "oldest":
				options.migrationFlags = options.migrationFlags | migrator.MigrationFlagOrderOldestFirst
			case "newest":
				options.migrationFlags = options.migrationFlags | migrator.MigrationFlagOrderNewestFirst

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

	flags.StringVarP(
		&options.targetAddress,
		"target",
		"t",
		"",
		"mailbox that will receive the migrated emails",
	)

	flags.StringVarP(
		&options.destination,
		"dest",
		"d",
		"",
		"only migrate emails with this original destination",
	)

	flags.StringVarP(
		&options.orderRaw,
		"order",
		"o",
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

	flags.BoolVar(
		&options.deltaMigration,
		"delta-migration",
		false,
		"enables migration in delta mode",
	)

	flags.BoolVar(
		&options.localEnforce,
		"local-enforce",
		false,
		"respect migration status stored in the local database",
	)

	flags.BoolVar(
		&options.dryRun,
		"dry-run",
		false,
		"performs a dry run",
	)

	cmd.Flags().BoolVarP(
		&options.testRun,
		"test",
		"T",
		false,
		"displays a textual message explaining the migration to be executed and exits",
	)

	if err := cmd.MarkFlagRequired("target"); err != nil {
		panic(err)
	}

	return cmd
}

func runMigration(ctx context.Context, options migrateUserOptions) error {
	logger, loggerCloser, err := logging.New(utils.LOG_FILE_PATH, utils.GetRunID(), options.verbosity)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer loggerCloser()

	commandLogger := logger.With("command", "migrate-user")
	commandLogger.Debug("migrate ran with options",
		"destination", options.destination,
		"limit", options.limit,
		"limit_set", options.limitSet,
		"max_attempts", options.maxAttempts,
		"order_raw", options.orderRaw,
		"target_address", options.targetAddress,
		"verbosity", options.verbosity,
		"workers", options.workers,
		"deltaMigration", options.deltaMigration,
		"dryRun", options.dryRun,
	)

	manager, err := migrator.NewMigrationManager(options.targetAddress, options.destination, options.maxAttempts, commandLogger)
	if err != nil && !errors.Is(err, utils.ErrDryRun) {
		return fmt.Errorf("create migration manager: %w", err)
	}
	defer manager.Close()

	// Setting manager options
	if err := manager.SetFromFlags(options.migrationFlags); err != nil {
		commandLogger.Error("failed to set options from flags", "error", err)
		return fmt.Errorf("set from flags: %w", err)
	}

	if err := manager.SetLimit(options.limit); err != nil {
		commandLogger.Error("failed to set limit", "error", err)
		return fmt.Errorf("set limit: %w", err)
	}

	if options.testRun {
		runDescTemplate := `
		---- RUN DESCRIPTION ----
		You are running an USER migration.
		This run would migrate Emails with:
			- original destination (DEST) set to '%s' in the database
			- to the account (TARGET) '%s' in the workspace
		

		To be included in the migration, the email's entry in the database should have the field DEST as '%s' and TARGET as '%s'.
		The user email address used for impersonation/token request is %s
		The operation would migrate the %s emails first.
		Each eligible entry would have up to %d attempts.

		ENTRY LIMIT: %s
		DELTA MODE: %t
		ORDER: %s
		FLAGS: %X (%b) (%d)
		STRING REPRESENTATION OF FLAGS SET: %s
		`

		var entryLimitText string
		if options.limit > 0 {
			entryLimitText = fmt.Sprintf("The operation is limited to %d entries", options.limit)
		} else {
			entryLimitText = "the operation has NO entry limit"
		}

		resultingText := fmt.Sprintf(
			runDescTemplate,
			options.destination,
			options.targetAddress,
			options.destination,
			options.targetAddress,
			options.targetAddress,
			options.orderRaw,
			options.maxAttempts,
			entryLimitText,
			options.migrationFlags.Has(migrator.MigrationFlagModeDelta),
			options.orderRaw,
			options.migrationFlags,
			options.migrationFlags,
			options.migrationFlags,
			options.migrationFlags.NamesString(),
		)

		commandLogger.Info("ran a test migration", "text", resultingText)
		fmt.Println(resultingText)
	}

	total, err := manager.CountEligible(ctx)
	if err != nil {
		commandLogger.Error("failed to count eligible", "error", err)
		return fmt.Errorf("counting eligibles: %w", err)
	}

	logger.Info("counted eligibles", "eligibles", total)
	if total == 0 && !options.testRun {
		return ErrNoEligibleEmails
	}

	if options.testRun {
		commandLogger.Info("ran eligible count on test migration", "total", total)
		fmt.Printf("There would be %d eligible elements\n", total)
		return nil
	}

	// Creating inserter/checker
	inserter, err := mailinserter.NewUserMailInserter(
		options.targetAddress,
		commandLogger,
	)
	if err != nil {
		return fmt.Errorf("create the inserter/checker: %w", err)
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

	migr := migrator.NewMigrator(manager, inserter, inserter, utils.EMAILS_ROOT_PATH, logger, reporter)

	if options.dryRun {
		return utils.ErrDryRun
	}

	runErr := migr.Run(
		ctx,
		options.workers,
	)

	succeeded := migr.MigratedCount()
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
		commandLogger.Error("failed during migration run", "error", runErr)
		return fmt.Errorf("run migration: %w", runErr)
	}

	commandLogger.Info("migration completed", "migrated_count", migr.MigratedCount(), "total", total, "inserted_count", migr.InsertedCount(), "already_existed_count", migr.AlreadyExistsCount())
	return nil
}

package cmd

import (
	"context"
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

type migrateGroupOptions struct {
	adminAddress   string
	destAddress    string
	groupAddress   string
	maxAttempts    int
	verbosity      bool
	limit          int
	orderRaw       string
	migrationFlags migrator.MigrationFlag
}

func newMigrateGroupCommand() *cobra.Command {
	var options migrateGroupOptions

	cmd := &cobra.Command{
		Use:   "group",
		Short: "Migrate pending emails to a target group's archive",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			options.migrationFlags = migrator.MigrationFlagEmpty
			options.migrationFlags.Set(migrator.MigrationFlagTargetGroup)
	
			options.orderRaw = strings.TrimSpace(strings.ToLower(options.orderRaw))
			switch options.orderRaw {
			case "newest":
				options.migrationFlags.Set(migrator.MigrationFlagOrderNewestFirst)
			case "oldest":
				options.migrationFlags.Set(migrator.MigrationFlagOrderOldestFirst)
			default:
				return fmt.Errorf("malformed order value: %s", options.orderRaw)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateGroup(cmd.Context(), options)
		},
	}

	cmd.Flags().StringVarP(
		&options.adminAddress,
		"system-address",
		"s",
		"",
		"the system's administrator address for token request signing",
	)

	cmd.Flags().StringVarP(
		&options.destAddress,
		"dest",
		"d",
		"",
		"only migrate emails with this original destination",
	)

	cmd.Flags().StringVarP(
		&options.groupAddress,
		"target",
		"t",
		"",
		"target group's email address",
	)

	cmd.Flags().IntVar(
		&options.maxAttempts,
		"max-attempts",
		5,
		"maximum times the insertion of an entry will be tried",
	)

	cmd.Flags().BoolVarP(
		&options.verbosity,
		"verbosity",
		"v",
		false,
		"enables log verbosity",
	)

	cmd.Flags().IntVar(
		&options.limit,
		"limit",
		0,
		"limits the amount of ordered emails to be attempted",
	)

	cmd.Flags().StringVar(
		&options.orderRaw,
		"order",
		"newest",
		"order of email insertion ('newest', 'oldest')",
	)
	return cmd
}

func runMigrateGroup(ctx context.Context, options migrateGroupOptions) error {
	logger, loggerCloser, err := logging.New(utils.LOG_FILE_PATH, utils.GetRunID(), options.verbosity)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer loggerCloser()

	commandLogger := logger.With("command", "migrate-group")

	if options.migrationFlags.Has(migrator.MigrationFlagModeDelta) {
		return fmt.Errorf("unable to start group migration with delta mode")
	}

	manager, err := migrator.NewMigrationManager(
		options.groupAddress,
		options.destAddress,
		options.maxAttempts,
		commandLogger,
	)
	if err != nil {
		commandLogger.Error("failed to create migration manager for group migration", "error", err)
		return fmt.Errorf("create migration manager: %w", err)
	}
	defer manager.Close()

	if err := manager.SetFromFlags(options.migrationFlags); err != nil {
		commandLogger.Error("failed to set options from flags", "error", err)
		return fmt.Errorf("set from flags: %w", err)
	}

	if err := manager.SetLimit(options.limit); err != nil {
		commandLogger.Error("failed to set limit", "error", err)
		return fmt.Errorf("set limit: %w", err)
	}

	total, err := manager.CountEligible(ctx)
	if err != nil {
		commandLogger.Error("failed to count eligible", "error", err)
		return fmt.Errorf("counting eligibles: %w", err)
	}

	logger.Info("counted eligibles", "eligibles", total)
	if total == 0 {
		return ErrNoEligibleEmails
	}

	// Creating only an inserter
	inserter, err := mailinserter.NewGroupMailInserter(
		options.groupAddress,
		options.adminAddress,
		commandLogger,
	)
	if err != nil {
		return fmt.Errorf("create the inserter: %w", err)
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

	migr := migrator.NewMigrator(manager, inserter, nil, utils.EMAILS_ROOT_PATH, commandLogger, reporter)

	runErr := migr.Run(
		ctx,
		1,
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
		"Group migration complete: %d of %d emails succeeded\n",
		succeeded,
		total,
	)

	if runErr != nil {
		commandLogger.Error("failed during group migration run", "error", runErr)
		return fmt.Errorf("run group migration: %w", runErr)
	}

	commandLogger.Info("group migration completed", "migrated_count", migr.MigratedCount(), "total", total, "inserted_count", migr.InsertedCount(), "already_existed_count", migr.AlreadyExistsCount())
	return nil
}

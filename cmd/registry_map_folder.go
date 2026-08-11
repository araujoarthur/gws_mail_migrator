package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

type mapFolderOptions struct {
	folder       string
	listOnly     bool
	target       string
	targetType   string
	dest         string
	verbosity    bool
	dryRun       bool
	optionsFlags migrator.MigrationFlag
}

func newRegistryMapFolderCommand() *cobra.Command {
	var options mapFolderOptions

	comd := &cobra.Command{
		Use:   "folder",
		Short: "maps all contets of a folder into database entities",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if options.listOnly {
				return nil
			}

			var missing []string

			// Presence validation
			if options.target == "" {
				missing = append(missing, "target")
			}

			if options.dest == "" {
				missing = append(missing, "dest")
			}

			if options.folder == "" {
				missing = append(missing, "in")
			}

			if options.targetType == "" {
				missing = append(missing, "target-type")
			}

			if len(missing) > 0 {
				return fmt.Errorf("required options not set: %s", strings.Join(missing, ","))
			}

			// flags.targetType validation
			options.targetType = strings.TrimSpace(strings.ToLower(options.targetType))
			switch options.targetType {
			case "group":
				options.optionsFlags.Set(migrator.MigrationFlagTargetGroup)
			case "user":
				options.optionsFlags.Set(migrator.MigrationFlagTargetUser)
			default:
				return fmt.Errorf("%w: 'target-type' value '%s' not recognized. Must be either 'user' or 'group'", ErrInvalidFlagValue, options.targetType)
			}

			if options.dryRun {
				options.optionsFlags.Set(migrator.MigrationFlagDryRun)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMapFolder(cmd.Context(), options)
		},
	}

	comd.Flags().StringVar(
		&options.folder,
		"in",
		"",
		"folder in emls/ to map",
	)

	comd.Flags().BoolVar(
		&options.listOnly,
		"list-only",
		false,
		"only enumerate the emails on the given folder",
	)

	comd.Flags().StringVar(
		&options.target,
		"target",
		"",
		"the target user email address in the final migration space",
	)

	comd.Flags().StringVar(
		&options.dest,
		"dest",
		"",
		"the single user email address this email's were addressed to in the original migration space",
	)

	comd.Flags().StringVar(
		&options.targetType,
		"target-type",
		"",
		"the type of target for insertion ('group', 'user')",
	)

	comd.Flags().BoolVarP(
		&options.verbosity,
		"verbosity",
		"v",
		false,
		"enable logging verbosity",
	)

	comd.Flags().BoolVar(
		&options.dryRun,
		"dry-run",
		false,
		"performs a dry run",
	)

	return comd
}

func runMapFolder(ctx context.Context, options mapFolderOptions) error {
	logger, logCloser, err := logging.New("./migration.log", utils.GetRunID(), options.verbosity)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer logCloser()

	commandLogger := logger.With("command", "map-folder")

	if options.dryRun {
		commandLogger.Debug("started a dry run",
			"opts_folder", options.folder,
			"opts_listOnly", options.listOnly,
			"opts_target", options.target,
			"opts_target_type", options.targetType,
			"opts_dest", options.dest,
			"opts_verbosity", options.verbosity,
			"opts_dry_run", options.dryRun,
		)
		commandLogger = commandLogger.WithGroup("dry_run")

		absolutePath, err := filepath.Abs(options.folder)
		if err != nil {
			commandLogger.Error("failed to get absolute path", "error", err)
			return fmt.Errorf("unable to get absolute path in a dry run: %w", err)
		}

		commandLogger.Debug("found an absolute path", "absolute_path", absolutePath)
	}

	mailList, err := utils.ListEmailFiles(filepath.Join(utils.EMAILS_ROOT_PATH, options.folder))
	if err != nil {
		return err
	}

	if options.listOnly {
		for idx, mail := range mailList {
			fmt.Printf("[%d] - %s\n", idx, mail)
		}

		return nil
	}

	manager, err := migrator.NewMigrationManager(options.target, options.dest, 5, commandLogger)
	if err != nil {
		commandLogger.Error("failed to create migration manager", "error", err)
		return err
	}

	if err := manager.SetFromFlags(options.optionsFlags); err != nil {
		commandLogger.Error("failed to set migration manager flags", "error", err)
		return err
	}

	for idx, fpath := range mailList {
		message, err := utils.ReadEmailFile(fpath)
		if err != nil {
			commandLogger.Error("failed to read file", "index", idx, "filepath", fpath, "error", err)
			continue
		}

		mailMessage := migrator.Email{
			MessageID:              message.MessageID,
			Filename:               message.Filename,
			Sender:                 message.Sender,
			FileHash:               message.FileHash,
			Destination:            options.dest,
			Date:                   message.Date,
			MigrationTargetAddress: options.target,
		}

		id, err := manager.AddEmail(ctx, mailMessage)
		if err != nil {
			if !errors.Is(err, utils.ErrDryRun) {
				commandLogger.Error("failed to add entry", "id", id, "index", idx, "filepath", fpath, "error", err)
			}
			continue
		}

		commandLogger.Info("successfully inserted", "id", id, "index", idx, "filepath", fpath)
	}

	return nil
}

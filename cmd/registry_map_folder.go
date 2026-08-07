package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

type mapFolderCommandFlags struct {
	folder    string
	listOnly  bool
	target    string
	dest      string
	verbosity bool
}

func newRegistryMapFolderCommand() *cobra.Command {
	var flags mapFolderCommandFlags

	comd := &cobra.Command{
		Use:   "folder",
		Short: "maps all contets of a folder into database entities",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if flags.listOnly {
				return nil
			}

			var missing []string

			if flags.target == "" {
				missing = append(missing, "target")
			}

			if flags.dest == "" {
				missing = append(missing, "dest")
			}

			if flags.folder == "" {
				missing = append(missing, "in")
			}

			if len(missing) > 0 {
				return fmt.Errorf("required flags not set: %s", strings.Join(missing, ","))
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMapFolder(cmd.Context(), flags)
		},
	}

	comd.Flags().StringVar(
		&flags.folder,
		"in",
		"",
		"folder in emls/ to map",
	)

	comd.Flags().BoolVar(
		&flags.listOnly,
		"list-only",
		false,
		"only enumerate the emails on the given folder",
	)

	comd.Flags().StringVar(
		&flags.target,
		"target",
		"",
		"the target user email address in the final migration space",
	)

	comd.Flags().StringVar(
		&flags.dest,
		"dest",
		"",
		"the single user email address this email's were addressed to in the original migration space",
	)

	comd.Flags().BoolVarP(
		&flags.verbosity,
		"verbosity",
		"v",
		false,
		"enable logging verbosity",
	)

	return comd
}

func runMapFolder(ctx context.Context, flags mapFolderCommandFlags) error {

	logger, logCloser, err := logging.New("./migration.log", flags.verbosity)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer logCloser()

	commandLogger := logger.With("command", "map-folder")

	mailList, err := utils.ListEmailFiles(filepath.Join(utils.EMAILS_ROOT_PATH, flags.folder))
	if err != nil {
		return nil
	}

	if flags.listOnly {
		for idx, mail := range mailList {
			fmt.Printf("[%d] - %s\n", idx, mail)
		}

		return nil
	}

	manager, err := migrator.NewMigrationManager(flags.target, flags.dest, 5, commandLogger)
	if err != nil {
		commandLogger.Error("failed to create migration manager", "error", err)
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
			Destination:            flags.dest,
			Date:                   message.Date,
			MigrationTargetAddress: flags.target,
		}

		id, err := manager.AddEmail(ctx, mailMessage)
		if err != nil {
			commandLogger.Error("failed to add entry", "id", id, "index", idx, "filepath", fpath, "error", err)
			continue
		}

		commandLogger.Info("successfully inserted", "id", id, "index", idx, "filepath", fpath)
	}

	return nil
}

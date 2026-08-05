package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

type mapFolderCommandFlags struct {
	folder   string
	listOnly bool
	target   string
	dest     string
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

	return comd
}

func runMapFolder(ctx context.Context, flags mapFolderCommandFlags) error {

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

	manager, err := migrator.NewMigrationManager(flags.target, flags.dest, 5)
	if err != nil {
		return err
	}

	for idx, fpath := range mailList {
		message, err := utils.ReadEmailFile(fpath)
		if err != nil {
			log.Printf("failed to read file [%d - %s]: %v\n", idx, fpath, err)
			continue
		}

		mailMessage := migrator.Email{
			Filename:               message.Filename,
			Sender:                 message.Sender,
			FileHash:               message.FileHash,
			Destination:            flags.dest,
			Date:                   message.Date,
			MigrationTargetAddress: flags.target,
		}

		id, err := manager.AddEmail(ctx, mailMessage)
		if err != nil {
			log.Printf("failed to add entry id:%d [%d - %s]: %v\n", id, idx, fpath, err)
			continue
		}

		log.Printf("New File Added (%d)\n", id)
	}

	return nil
}

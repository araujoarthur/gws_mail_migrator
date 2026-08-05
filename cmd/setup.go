package cmd

import (
	"fmt"

	"github.com/araujoarthur/gws_mail_migrator/lib/impersonator"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

func newSetupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Initializes the migration dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup()
		},
	}
}

func runSetup() error {
	// Verify Credentials.json existence
	if exists, err := utils.FileExists(utils.CREDENTIALS_PATH); !exists || err != nil {
		if err != nil {
			return fmt.Errorf("checking credentials.json existence: %w", err)
		}

		return fmt.Errorf("credentials.json is missing")
	}

	// Verify Credentials.json fields
	if _, err := impersonator.LoadCredentialsFile(utils.CREDENTIALS_PATH); err != nil {
		return fmt.Errorf("loading and verifying the credentials file: %w", err)
	}

	// Verify emls folder existence (create if not exists)
	if exists, err := utils.FolderExists(utils.EMAILS_ROOT_PATH); !exists || err != nil {
		if err != nil {
			return fmt.Errorf("veryfing root emails folder existence: %w", err)
		}
	}

	// Run DB Setup
	if err := migrator.InitialDBSetup(); err != nil {
		return fmt.Errorf("initial db setup: %w", err)
	}

	// Run Folder Setup
	if err := migrator.InitialFolderSetup(); err != nil {
		return fmt.Errorf("initial folder setup: %w", err)
	}

	return nil
}

package cmd

import (
	"fmt"
	"os"

	"github.com/araujoarthur/gws_mail_migrator/lib/impersonator"
	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

type setupCommandOptions struct {
	verbosity bool
}

func newSetupCommand() *cobra.Command {
	var options setupCommandOptions

	comd := &cobra.Command{
		Use:   "setup",
		Short: "Initializes the migration dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(options)
		},
	}

	comd.Flags().BoolVarP(
		&options.verbosity,
		"verbosity",
		"v",
		false,
		"enable logging verbosity",
	)

	return comd

}

func runSetup(options setupCommandOptions) error {

	logger, logCloser, err := logging.New("./migration.log", options.verbosity)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer logCloser()

	commandLogger := logger.With("command", "setup")

	// Verify Credentials.json existence
	if exists, err := utils.FileExists(utils.CREDENTIALS_PATH); !exists || err != nil {
		if err != nil {
			commandLogger.Error("failed to check credentials.json existence", "error", err)
			return fmt.Errorf("checking credentials.json existence: %w", err)
		}

		commandLogger.Error("credentials.json is missing")
		return fmt.Errorf("credentials.json is missing")
	}

	// Verify Credentials.json fields
	if _, err := impersonator.LoadCredentialsFile(utils.CREDENTIALS_PATH); err != nil {
		commandLogger.Error("failed to load and verify credentials file", "error", err)
		return fmt.Errorf("loading and verifying the credentials file: %w", err)
	}

	// Verify emls folder existence (create if not exists)
	if exists, err := utils.FolderExists(utils.EMAILS_ROOT_PATH); !exists || err != nil {
		if err != nil {
			commandLogger.Error("failed to verify root emails folder existence", "error", err)
			return fmt.Errorf("veryfing root emails folder existence: %w", err)
		}

		if err := os.Mkdir(utils.EMAILS_ROOT_PATH, 0o755); err != nil {
			commandLogger.Error("failed create root folder", "error", err)
			return fmt.Errorf("creating root folder: %w", err)
		}
	}

	// Run DB Setup
	if err := migrator.InitialDBSetup(commandLogger.With("exec_branch", "initial-db-setup")); err != nil {
		commandLogger.Error("failed to initialize DB", "error", err)
		return fmt.Errorf("initial db setup: %w", err)
	}

	// Run Folder Setup
	if err := migrator.InitialFolderSetup(); err != nil {
		commandLogger.Error("failed to failed to initialize folder", "error", err)
		return fmt.Errorf("initial folder setup: %w", err)
	}

	return nil
}

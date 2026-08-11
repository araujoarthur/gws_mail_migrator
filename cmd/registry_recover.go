package cmd

import (
	"context"
	"fmt"

	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

func newRegistryRecoverCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "recover",
		Short: "recover interrupted entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecovery(cmd.Context())
		},
	}
}

func runRecovery(ctx context.Context) error {
	logger, loggerCloser, err := logging.New(utils.LOG_FILE_PATH, utils.GetRunID(), true)
	if err != nil {
		fmt.Printf("%v", err)
		panic("failed to create a logger")
	}
	defer loggerCloser()

	commandLogger := logger.With("command", "registry-recover")
	manager, err := migrator.NewMigrationManager("recover@local", "recover@local", 5, commandLogger)
	if err != nil {
		commandLogger.Error("failed to create manager for recovery", "error", err)
		return err
	}
	defer manager.Close()

	recovered, err := manager.RecoverInterrupted(ctx)
	if err != nil {
		commandLogger.Error("failed to recover", "error", err)
		return err
	}

	commandLogger.Info("recovered stale entries", "count", recovered)

	return nil
}

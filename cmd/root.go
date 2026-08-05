package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	rootCmd := newRootCommand()
	return rootCmd.ExecuteContext(ctx)
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gmm",
		Short:         "Migrate email files from local storage (.eml) to Google Workspace",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newSetupCommand())
	cmd.AddCommand(newRegistryCommand())

	return cmd
}

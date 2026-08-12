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

	cmd.AddGroup(&cobra.Group{
		ID:    "migration",
		Title: "Migration Commands",
	})

	cmd.AddGroup(&cobra.Group{
		ID:    "management",
		Title: "Management Commands",
	})

	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newSetupCommand())
	cmd.AddCommand(newRegistryCommand())
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newInspectCommand())
	cmd.AddCommand(newCreateCommand())

	return cmd
}

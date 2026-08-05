package cmd

import "github.com/spf13/cobra"

func newRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "registry",
		Short:         "Entry point for management of the email registry database",
		SilenceErrors: true,
	}

	cmd.AddCommand(newRegistryResetCommand())
	cmd.AddCommand(newRegistryMapCommand())

	return cmd
}

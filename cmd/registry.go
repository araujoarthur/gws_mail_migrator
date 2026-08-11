package cmd

import "github.com/spf13/cobra"

func newRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "registry",
		Short:         "Management of the email registry database",
		SilenceErrors: true,
		GroupID:       "management",
	}

	cmd.AddCommand(newRegistryResetCommand())
	cmd.AddCommand(newRegistryMapCommand())
	cmd.AddCommand(newRegistryRecoverCommand())

	return cmd
}

package cmd

import "github.com/spf13/cobra"

func newRegistryMapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "map",
		Short:         "file to database mapping",
		SilenceErrors: true,
	}

	cmd.AddCommand(newRegistryMapFolderCommand())

	return cmd
}

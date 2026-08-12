package cmd

import "github.com/spf13/cobra"

func newInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "inspect",
		Short:         "inspects a diversity of data",
		SilenceErrors: true,
		GroupID:       "management",
	}

	cmd.AddCommand(newInspectLabelsCommand())

	return cmd
}

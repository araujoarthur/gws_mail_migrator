package cmd

import "github.com/spf13/cobra"

func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "handles a diversity of creations",
		GroupID: "management",
	}

	cmd.AddCommand(newCreateLabelCommand())

	return cmd
}

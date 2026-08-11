package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Prints the application version",
		GroupID: "management",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("gmm@%s\n", applicationVersion())
			return nil
		},
	}
}

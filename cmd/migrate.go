package cmd

import (
	"github.com/spf13/cobra"
)

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate",
		Short:   "Migrate pending emails to a target mailbox",
		GroupID: "migration",
		Args:    cobra.NoArgs,
	}

	cmd.AddCommand(newMigrateUserCommand())
	cmd.AddCommand(newMigrateGroupCommand())

	return cmd
}

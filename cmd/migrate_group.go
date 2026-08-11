package cmd

import "github.com/spf13/cobra"

type migrateGroupOptions struct {
}

func newMigrateGroupCommand() *cobra.Command {
	//var options migrateGroupOptions

	cmd := &cobra.Command{
		Use:   "group",
		Short: "Migrate pending emails to a target user's mailbox",
	}

	return cmd
}

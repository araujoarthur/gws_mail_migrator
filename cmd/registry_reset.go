package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/araujoarthur/gws_mail_migrator/lib/challenger"
	"github.com/araujoarthur/gws_mail_migrator/lib/migrator"
	"github.com/spf13/cobra"
)

func newRegistryResetCommand() *cobra.Command {
	var acknowledge string

	comd := &cobra.Command{
		Use:   "reset",
		Short: "Resets the registry database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResetDB(cmd.OutOrStdout(), acknowledge)
		},
	}

	comd.Flags().StringVar(
		&acknowledge,
		"acknowledge",
		"",
		"confirmation code for destructive registry reset",
	)

	return comd
}

func runResetDB(out io.Writer, acknowledge string) error {
	if acknowledge == "" {
		// path for the run without acknowledge code.
		newChallenge, err := challenger.NewDestructiveChallenge(challenger.ActionResetDatabase)
		if err != nil {
			return err
		}

		fmt.Fprintf(
			out,
			"WARNING: This operation will permanently delete the registry database.\n\n"+
				"To continue, run:\n\n"+
				"    gmm registry reset --acknowledge=%s\n",
			newChallenge.Code,
		)

		return nil
	}

	challenge, err := challenger.LoadDestructiveChallenge(challenger.ActionResetDatabase)
	if err != nil {
		return errors.New("failed to load current challenge")
	}

	if err := challenge.Validate(acknowledge); err != nil {
		return err
	}

	return migrator.ResetDB()
}

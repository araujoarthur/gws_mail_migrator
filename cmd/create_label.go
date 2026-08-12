package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/mailinserter"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

type createLabelOptions struct {
	target    string
	labelName string
}

func newCreateLabelCommand() *cobra.Command {
	var options createLabelOptions

	cmd := &cobra.Command{
		Use:   "label",
		Short: "creates a label on a given user's gmail account",
		RunE: func(cmd *cobra.Command, args []string) error {
			options.target = strings.TrimSpace(strings.ToLower(options.target))
			options.labelName = strings.TrimSpace(options.labelName)

			return runCreateLabel(cmd.Context(), options)
		},
	}

	cmd.Flags().StringVarP(
		&options.target,
		"target",
		"t",
		"",
		"account where the label will be created",
	)

	cmd.Flags().StringVarP(
		&options.labelName,
		"name",
		"n",
		"",
		"name of the label to be created",
	)

	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("name")

	return cmd
}

func runCreateLabel(ctx context.Context, options createLabelOptions) error {
	logger, loggerCloser, err := logging.New(utils.LOG_FILE_PATH, utils.GetRunID(), false)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer loggerCloser()

	commandLogger := logger.With("command", "create-label")

	um, err := mailinserter.NewUserMailInserter(options.target, commandLogger)
	if err != nil {
		return fmt.Errorf("create the user mail inserter: %w", err)
	}

	result, err := um.CreateLabel(ctx, options.labelName)
	if err != nil {
		return fmt.Errorf("create label: %w", err)
	}

	fmt.Printf("Label Found:\n%s (%s)\n", result.Name, result.ID)

	return nil
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/araujoarthur/gws_mail_migrator/lib/logging"
	"github.com/araujoarthur/gws_mail_migrator/lib/mailinserter"
	"github.com/araujoarthur/gws_mail_migrator/lib/utils"
	"github.com/spf13/cobra"
)

type inspectLabelsOptions struct {
	target  string
	find    string
	findSet bool
}

func newInspectLabelsCommand() *cobra.Command {
	var options inspectLabelsOptions
	cmd := &cobra.Command{
		Use:           "labels",
		Short:         "inspects the labels of an user",
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("find") {
				options.find = strings.TrimSpace(options.find)
				options.findSet = true
			}

			if options.target == "" {
				return errors.New("target cannot be empty")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListLabels(cmd.Context(), options)
		},
	}

	cmd.Flags().StringVarP(
		&options.target,
		"target",
		"t",
		"",
		"the user to inspect the labels from",
	)

	cmd.Flags().StringVarP(
		&options.find,
		"find",
		"f",
		"",
		"specify a label to look for",
	)

	return cmd
}

func runListLabels(ctx context.Context, options inspectLabelsOptions) error {
	logger, loggerCloser, err := logging.New(utils.LOG_FILE_PATH, utils.GetRunID(), false)
	if err != nil {
		panic(fmt.Errorf("initializing logging capabilities: %w", err))
	}
	defer loggerCloser()

	commandLogger := logger.With("command", "inspect-labels")

	um, err := mailinserter.NewUserMailInserter(options.target, commandLogger)
	if err != nil {
		return fmt.Errorf("create the user mail inserter: %w", err)
	}

	if !options.findSet {
		// just list the labels
		labelList, err := um.GetLabels(ctx)
		if err != nil {
			return fmt.Errorf("get labels: %w", err)
		}

		for _, label := range labelList {
			fmt.Printf("%s (%s - %s)\n", label.Name, label.ID, label.Type)
		}
	} else {
		result, found, err := um.FindLabel(ctx, options.find)
		if err != nil {
			return fmt.Errorf("find label: %w", err)
		}

		if !found {
			return errors.New("label not found")
		}

		fmt.Printf("Label Found:\n%s (%s - %s)\n", result.Name, result.ID, result.Type)
	}

	return nil
}

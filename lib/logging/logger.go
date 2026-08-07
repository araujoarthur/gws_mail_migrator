package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gofrs/uuid/v5"
)

func New(path string, verbose bool) (*slog.Logger, func() error, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0600,
	)
	if err != nil {
		return nil, nil, err
	}

	var writer io.Writer = file
	level := slog.LevelInfo

	if verbose {
		writer = io.MultiWriter(file, os.Stdout)
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
	})

	runUUID, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Errorf("generate log uuid: %w", err))
	}

	logger := slog.New(handler).With("run_id", runUUID.String())
	return logger, file.Close, nil
}

package logging

import (
	"io"
	"log/slog"
	"os"
)

func New(path string, runID string, verbose bool) (*slog.Logger, func() error, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0600,
	)
	if err != nil {
		return nil, nil, err
	}

	var writer io.Writer = file
	level := slog.LevelDebug

	if verbose {
		writer = io.MultiWriter(file, os.Stdout)
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler).With("run_id", runID)
	return logger, file.Close, nil
}

package utils

import "errors"

var (
	ErrDryRun = errors.New("within a dry run")
)

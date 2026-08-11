package cmd

import "errors"

var (
	ErrNoEligibleEmails = errors.New("no eligible emails")
	ErrInvalidFlagValue = errors.New("invalid flag value")
)

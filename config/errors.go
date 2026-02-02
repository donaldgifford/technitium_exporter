package config

import "errors"

var (
	// ErrMissingURL is returned when the Technitium URL is not configured.
	ErrMissingURL = errors.New("TECHNITIUM_URL is required")

	// ErrMissingToken is returned when the Technitium API token is not configured.
	ErrMissingToken = errors.New("TECHNITIUM_TOKEN is required")
)

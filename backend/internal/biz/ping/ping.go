// Package ping is the minimal health-check business command.
//
// Use it as the smallest possible command template (no args, fixed payload).
// For a fuller sample with validation and i18n, see package hello.
//
//	main ping
//	main ping --json
package ping

import (
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

// Run implements the ping command strategy and returns a plain string.
func Run(_ *middleware.Context, _ input.Input) (any, error) {
	return "pong", nil
}

package cli

import (
	"errors"

	"github.com/pformoso/csk/internal/exitcode"
	"github.com/pformoso/csk/internal/procguard"
)

// userErrf wraps an error as a UserError. Use for invalid input, missing
// manifest entries, conflicting state, etc.
func userErr(err error) error {
	if err == nil {
		return nil
	}
	return &exitcode.UserError{Err: err}
}

// envErr wraps an error as an EnvError. Use for filesystem/network/git
// failures the user can't fix by editing input.
func envErr(err error) error {
	if err == nil {
		return nil
	}
	return &exitcode.EnvError{Err: err}
}

// classifyProcguard turns procguard.ErrBusy into a UserError; anything
// else into an EnvError.
func classifyProcguard(err error) error {
	if errors.Is(err, procguard.ErrBusy) {
		return userErr(err)
	}
	return envErr(err)
}

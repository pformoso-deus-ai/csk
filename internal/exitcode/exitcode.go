// Package exitcode defines csk's process exit-code conventions.
//
//	0 — success
//	1 — user error (bad input, missing manifest entry, etc.)
//	2 — environment error (network, filesystem, missing git, etc.)
package exitcode

import "errors"

const (
	OK      = 0
	UserErr = 1
	EnvErr  = 2
)

// UserError marks an error as user-induced. Wrap with %w.
type UserError struct{ Err error }

func (e *UserError) Error() string { return e.Err.Error() }
func (e *UserError) Unwrap() error { return e.Err }

// EnvError marks an error as environment / runtime.
type EnvError struct{ Err error }

func (e *EnvError) Error() string { return e.Err.Error() }
func (e *EnvError) Unwrap() error { return e.Err }

// From maps an error to its exit code.
func From(err error) int {
	if err == nil {
		return OK
	}
	var ue *UserError
	if errors.As(err, &ue) {
		return UserErr
	}
	var ee *EnvError
	if errors.As(err, &ee) {
		return EnvErr
	}
	return EnvErr
}

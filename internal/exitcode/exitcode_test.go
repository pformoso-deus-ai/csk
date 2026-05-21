package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestFrom_Nil(t *testing.T) {
	if got := From(nil); got != OK {
		t.Errorf("From(nil) = %d, want %d", got, OK)
	}
}

func TestFrom_UserError(t *testing.T) {
	err := &UserError{Err: errors.New("bad input")}
	if got := From(err); got != UserErr {
		t.Errorf("From(UserError) = %d, want %d", got, UserErr)
	}
}

func TestFrom_EnvError(t *testing.T) {
	err := &EnvError{Err: errors.New("io")}
	if got := From(err); got != EnvErr {
		t.Errorf("From(EnvError) = %d, want %d", got, EnvErr)
	}
}

func TestFrom_PlainError(t *testing.T) {
	if got := From(errors.New("generic")); got != EnvErr {
		t.Errorf("From(plain) = %d, want %d", got, EnvErr)
	}
}

func TestFrom_WrappedUserError(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", &UserError{Err: errors.New("inner")})
	if got := From(wrapped); got != UserErr {
		t.Errorf("From(wrapped UserError) = %d, want %d", got, UserErr)
	}
}

func TestUserError_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("boom")
	e := &UserError{Err: inner}
	if e.Error() != "boom" {
		t.Errorf("Error() = %q", e.Error())
	}
	if e.Unwrap() != inner {
		t.Errorf("Unwrap() did not return inner")
	}
}

func TestEnvError_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("env-boom")
	e := &EnvError{Err: inner}
	if e.Error() != "env-boom" {
		t.Errorf("Error() = %q", e.Error())
	}
	if e.Unwrap() != inner {
		t.Errorf("Unwrap() did not return inner")
	}
}

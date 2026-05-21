package cli

import (
	"errors"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/exitcode"
	"github.com/pformoso-deus-ai/csk/internal/procguard"
)

func TestUserErr_NilIsNil(t *testing.T) {
	if got := userErr(nil); got != nil {
		t.Errorf("userErr(nil) = %v, want nil", got)
	}
}

func TestUserErr_WrapsAsUserError(t *testing.T) {
	got := userErr(errors.New("bad"))
	if exitcode.From(got) != exitcode.UserErr {
		t.Errorf("userErr(...) did not classify as UserErr (got %d)", exitcode.From(got))
	}
}

func TestEnvErr_NilIsNil(t *testing.T) {
	if got := envErr(nil); got != nil {
		t.Errorf("envErr(nil) = %v, want nil", got)
	}
}

func TestEnvErr_WrapsAsEnvError(t *testing.T) {
	got := envErr(errors.New("io"))
	if exitcode.From(got) != exitcode.EnvErr {
		t.Errorf("envErr(...) did not classify as EnvErr (got %d)", exitcode.From(got))
	}
}

func TestClassifyProcguard_BusyIsUserErr(t *testing.T) {
	got := classifyProcguard(procguard.ErrBusy)
	if exitcode.From(got) != exitcode.UserErr {
		t.Errorf("ErrBusy did not classify as UserErr (got %d)", exitcode.From(got))
	}
}

func TestClassifyProcguard_OtherIsEnvErr(t *testing.T) {
	got := classifyProcguard(errors.New("disk full"))
	if exitcode.From(got) != exitcode.EnvErr {
		t.Errorf("generic error did not classify as EnvErr (got %d)", exitcode.From(got))
	}
}

func TestShortSHA(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{"abcdefghijkl", "abcdefghijkl"},                                 // exactly 12 chars
		{"abcdefghijklmn", "abcdefghijkl"},                               // truncate
		{"a1b2c3d4e5f6789abcdef0123456789abcdef0123", "a1b2c3d4e5f6"},    // full SHA
	}
	for _, c := range cases {
		if got := shortSHA(c.in); got != c.want {
			t.Errorf("shortSHA(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate([]string{"a", "b"}, 5); len(got) != 2 {
		t.Errorf("truncate(short) length = %d, want 2", len(got))
	}
	got := truncate([]string{"a", "b", "c", "d", "e"}, 2)
	if len(got) != 3 {
		t.Errorf("truncate(long) length = %d, want 3 (2 + summary)", len(got))
	}
	if got[2] != "... and 3 more" {
		t.Errorf("summary line = %q", got[2])
	}
}

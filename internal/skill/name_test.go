package skill

import "testing"

func TestInferNameFromSource(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://github.com/foo/bar.git", "bar"},
		{"https://github.com/foo/bar", "bar"},
		{"https://github.com/foo/bar/", "bar"},
		{"https://github.com/foo/bar.git/", "bar"},
		{"git@github.com:foo/bar.git", "bar"},
		{"git@github.com:bar.git", "bar"},
		{"/local/path/to/repo", "repo"},
		{"/local/path/to/repo/", "repo"},
		{"bar", "bar"},
		{"bar.git", "bar"},
		{"", ""},
	}
	for _, c := range cases {
		got := InferNameFromSource(c.in)
		if got != c.want {
			t.Errorf("InferNameFromSource(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

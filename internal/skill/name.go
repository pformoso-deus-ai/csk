package skill

import "strings"

// InferNameFromSource returns the default install name for a git source URL,
// by taking the last path component and stripping a trailing ".git".
//
// Examples:
//
//	https://github.com/foo/bar.git    → "bar"
//	https://github.com/foo/bar/       → "bar"
//	git@github.com:foo/bar.git        → "bar"
//	/local/path/to/repo               → "repo"
//	C:\path\to\repo                   → "repo"
//
// Returns "" if no usable component can be derived.
func InferNameFromSource(source string) string {
	s := strings.TrimRight(source, "/\\")
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndexAny(s, "/:\\"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// Package link abstracts over the platform-native primitive for exposing
// a cache directory as <skills>/<name>:
//
//	Linux / macOS: symlink (os.Symlink).
//	Windows:       directory junction (`cmd /c mklink /J`).
//
// The chosen primitive is decided at install time, never stored.
package link

// Ensure makes <linkPath> point to <target>.
//
// Behavior:
//   - If <linkPath> does not exist → create it.
//   - If <linkPath> exists and is the correct kind of link pointing at
//     <target> → no-op.
//   - If <linkPath> exists but is a regular directory / file or points
//     elsewhere → return ErrWouldClobber, with no modifications.
//
// Removal of an existing wrong link is the caller's responsibility
// (typically `csk adopt` or `csk install --force`), to keep this function
// non-destructive.
//
// Implemented per-OS in link_unix.go and link_windows.go.
func Ensure(target, linkPath string) error {
	return ensure(target, linkPath)
}

// Remove deletes <linkPath> if and only if it is a link (symlink or junction).
// Refuses to remove a regular directory.
func Remove(linkPath string) error {
	return remove(linkPath)
}

// IsManagedLink reports whether <linkPath> is a link (symlink/junction) that
// points at <expectedTarget>. Used by `csk doctor` and `csk list`.
func IsManagedLink(linkPath, expectedTarget string) (bool, error) {
	return isManagedLink(linkPath, expectedTarget)
}

// ErrWouldClobber is returned when Ensure refuses to replace an existing
// non-managed entry at the link path.
type ErrWouldClobber struct{ Path string }

func (e *ErrWouldClobber) Error() string {
	return "link would clobber existing entry at " + e.Path
}

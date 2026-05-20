package cli_test

import (
	"strings"
	"testing"
)

func TestList_EmptyShowsHeaderOnly(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "STATE") {
		t.Errorf("expected table header, got %q", out)
	}
}

func TestList_AfterAddShowsClean(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "handoff") {
		t.Errorf("expected 'handoff' in list output, got %q", out)
	}
	if !strings.Contains(out, "clean") {
		t.Errorf("expected state 'clean', got %q", out)
	}
}

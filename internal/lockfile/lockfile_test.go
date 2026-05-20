package lockfile

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "skills.lock")
	in := New()
	in.Skills = []Entry{
		{Name: "handoff", Source: "https://x/y.git", Ref: "main", Commit: "abc123"},
		{Name: "alpha", Source: "https://x/z.git", Ref: "main", Commit: "def456"},
	}
	if err := in.Save(p); err != nil {
		t.Fatal(err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(out.Skills))
	}
	// Save sorts entries alphabetically by name.
	if out.Skills[0].Name != "alpha" || out.Skills[1].Name != "handoff" {
		t.Errorf("entries not sorted: %s, %s", out.Skills[0].Name, out.Skills[1].Name)
	}
}

func TestUpsertAndRemove(t *testing.T) {
	f := New()
	f.Upsert(Entry{Name: "a", Source: "x", Commit: "1"})
	f.Upsert(Entry{Name: "b", Source: "x", Commit: "1"})
	f.Upsert(Entry{Name: "a", Source: "x", Commit: "2"}) // replace

	if got := f.Find("a"); got == nil || got.Commit != "2" {
		t.Errorf("Find(a).Commit = %v, want 2", got)
	}
	if !f.Remove("a") {
		t.Error("Remove(a) returned false")
	}
	if f.Find("a") != nil {
		t.Error("a still present after Remove")
	}
	if f.Remove("nope") {
		t.Error("Remove(nope) returned true")
	}
}

func TestValidate_RejectsDuplicates(t *testing.T) {
	f := &File{
		Version: Version,
		Skills: []Entry{
			{Name: "a", Source: "x", Commit: "1"},
			{Name: "a", Source: "x", Commit: "2"},
		},
	}
	if err := f.Validate(); err == nil {
		t.Error("expected duplicate-name error")
	}
}

func TestValidate_RejectsMissingFields(t *testing.T) {
	cases := []*File{
		{Version: Version, Skills: []Entry{{Name: "", Source: "x", Commit: "1"}}},
		{Version: Version, Skills: []Entry{{Name: "a", Source: "", Commit: "1"}}},
		{Version: Version, Skills: []Entry{{Name: "a", Source: "x", Commit: ""}}},
	}
	for i, f := range cases {
		if err := f.Validate(); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

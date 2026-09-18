package difftree

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestInspectFollowsSymlinkedRoot: a root that is itself a symlink (e.g.
// `current -> releases/42`) must be walked, not treated as an empty tree.
func TestInspectFollowsSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "a")
	release := filepath.Join(base, "release-1")
	write(t, filepath.Join(a, "x.txt"), "same")
	write(t, filepath.Join(a, "sub", "y.txt"), "old")
	write(t, filepath.Join(release, "x.txt"), "same")
	write(t, filepath.Join(release, "sub", "y.txt"), "new")
	current := filepath.Join(base, "current")
	if err := os.Symlink(release, current); err != nil {
		t.Fatal(err)
	}

	for _, mode := range []Mode{ModeMTime, ModeHash} {
		diffs, terr := Inspect(context.Background(), Request{A: a, B: current, Mode: mode})
		if terr != nil {
			t.Fatalf("%s: %v", mode, terr)
		}
		// Only sub/y.txt differs; with mtime mode x.txt may also differ by
		// mtime, but nothing may be reported as removed or added.
		var sawY bool
		for _, d := range diffs {
			if d.Status != StatusChanged {
				t.Errorf("%s: symlinked root walked as empty: %+v", mode, d)
			}
			if d.Path == "sub/y.txt" {
				sawY = true
			}
		}
		if !sawY {
			t.Errorf("%s: expected sub/y.txt changed, got %+v", mode, diffs)
		}
	}
}

// TestFileDiff_KeepsLinesThatLookLikeHeaders: a removed "-- comment" line is
// rendered "--- comment" and an added "++ x" line "+++ x"; only the two file
// headers before the first hunk may be dropped, not these.
func TestFileDiff_KeepsLinesThatLookLikeHeaders(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.sql")
	b := filepath.Join(dir, "b.sql")
	writeFile(t, a, "select 1;\n-- drop table users;\nselect 2;\n")
	writeFile(t, b, "select 1;\nselect 2;\n++ added\n")
	res, terr := FileDiff(a, b)
	if terr != nil {
		t.Fatal(terr)
	}
	var removed, added *FileDiffLine
	for i, l := range res.Lines {
		switch {
		case l.Kind == FileDiffRemove && l.Text == "-- drop table users;":
			removed = &res.Lines[i]
		case l.Kind == FileDiffAdd && l.Text == "++ added":
			added = &res.Lines[i]
		}
	}
	if removed == nil || added == nil {
		t.Fatalf("header-looking lines were dropped: %+v", res.Lines)
	}
	if removed.AOld != 2 {
		t.Errorf("removed line numbered %d, want 2", removed.AOld)
	}
	if added.BNew != 3 {
		t.Errorf("added line numbered %d, want 3", added.BNew)
	}
}

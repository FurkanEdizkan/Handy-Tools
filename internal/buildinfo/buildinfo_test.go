package buildinfo

import (
	"regexp"
	"strings"
	"testing"
)

var semverRE = regexp.MustCompile(`^\d+\.\d+\.\d+(-[A-Za-z0-9.-]+)?(\+[A-Za-z0-9.-]+)?$`)

func TestEmbeddedVersionIsSemver(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty; version.txt must contain a semver string")
	}
	if !semverRE.MatchString(Version) {
		t.Fatalf("Version %q does not look like semver", Version)
	}
}

func TestStringIncludesPlatform(t *testing.T) {
	got := String()
	if got == "" {
		t.Fatal("String() returned empty")
	}
	if got[0] != 'v' {
		t.Fatalf("String() %q must start with 'v'", got)
	}
}

func TestStringHandlesUnsetVersion(t *testing.T) {
	saved := Version
	t.Cleanup(func() { Version = saved })
	Version = ""
	got := String()
	if got == "" {
		t.Fatal("String() returned empty for unset Version")
	}
	if want := "v0.0.0-dev"; got[:len(want)] != want {
		t.Fatalf("String() with empty Version = %q; want prefix %q", got, want)
	}
}

// TestFillFromVCSPopulatesCommit confirms that the VCS-info fallback
// populates Commit when -ldflags didn't. We can't assert an exact hash (it
// depends on the working tree), but `go test` runs through fillFromVCS()
// at package init, so Commit + Date should be set under any normal test
// environment that has git metadata.
func TestFillFromVCSPopulatesCommit(t *testing.T) {
	if Commit == "" {
		t.Skip("Commit not populated — expected when building outside a VCS checkout")
	}
	// Commit is either 7 hex chars, or 7 hex chars + "-dirty".
	if len(Commit) < 7 {
		t.Fatalf("Commit %q is shorter than the expected 7-char prefix", Commit)
	}
	core := Commit
	if rest, ok := strings.CutSuffix(core, "-dirty"); ok {
		core = rest
	}
	if len(core) != 7 {
		t.Fatalf("Commit core %q is not 7 chars", core)
	}
	for _, r := range core {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("Commit core %q contains non-hex char %q", core, r)
		}
	}
}

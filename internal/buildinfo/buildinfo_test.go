package buildinfo

import (
	"regexp"
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

// Package buildinfo exposes the version, commit, and build date that the
// htools and htoolsd binaries were built with.
//
// The repository's VERSION file is the source of truth in dev: it is embedded
// at compile time and used when no overrides are set. Release builds set
// Version, Commit, and Date via -ldflags "-X" so the values reflect the tag
// and git state, matching what GoReleaser publishes. As a dev-build fallback,
// init() reads commit + timestamp from runtime/debug.ReadBuildInfo so a plain
// `go build` (or `make gui`) still reports which commit it came from, which is
// what the sidebar uses to tell two dev builds apart.
package buildinfo

import (
	_ "embed"
	"runtime"
	"runtime/debug"
	"strings"
)

//go:embed version.txt
var embeddedVersion string

// Version is the semver of this build, without a leading "v".
// Overridable via -ldflags "-X github.com/furkandedizkan/handy-tools/internal/buildinfo.Version=..."
//
// Must be a plain string variable (no initializer expression) — the linker's
// -X flag silently no-ops on variables with non-trivial initializers like
// strings.TrimSpace(...). The embedded fallback is applied in init() below.
var Version string

// Commit is the short git commit hash this build was produced from.
// Empty in plain `go build`; populated in release builds.
var Commit string

// Date is the build timestamp in RFC3339. Empty in plain `go build`.
var Date string

func init() {
	// Apply the embedded fallback only when -ldflags didn't already set
	// Version. In release builds GoReleaser supplies the real calver.
	if Version == "" {
		Version = strings.TrimSpace(embeddedVersion)
	}
	// Fill in Commit + Date from Go's embedded VCS info when -ldflags didn't
	// supply them. This is what makes `go build` / `make gui` reveal which
	// commit a local dev binary was cut from.
	fillFromVCS()
}

// fillFromVCS reads runtime/debug.ReadBuildInfo's vcs.* settings to populate
// Commit and Date when they were left blank (i.e. no -ldflags overrides).
func fillFromVCS() {
	if Commit != "" && Date != "" {
		return
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	settings := map[string]string{}
	for _, s := range bi.Settings {
		settings[s.Key] = s.Value
	}
	if Commit == "" {
		if rev := settings["vcs.revision"]; len(rev) >= 7 {
			Commit = rev[:7]
		}
	}
	if Date == "" {
		Date = settings["vcs.time"]
	}
	if settings["vcs.modified"] == "true" && Commit != "" && !strings.HasSuffix(Commit, "-dirty") {
		Commit += "-dirty"
	}
}

// String returns a one-line human-readable build identifier.
func String() string {
	v := Version
	if v == "" {
		v = "0.0.0-dev"
	}
	out := "v" + v
	if Commit != "" {
		out += " (" + Commit + ")"
	}
	if Date != "" {
		out += " built " + Date
	}
	out += " " + runtime.GOOS + "/" + runtime.GOARCH
	return out
}

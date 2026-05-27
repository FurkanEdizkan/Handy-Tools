package hash

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// drainEvents reads every event from a hash.Run channel into a slice and
// returns the slice plus the terminal event.
func drainEvents(t *testing.T, ch <-chan tools.Progress) ([]tools.Progress, tools.Progress) {
	t.Helper()
	var events []tools.Progress
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) == 0 {
		t.Fatal("no events on progress channel")
	}
	last := events[len(events)-1]
	if !last.Completed {
		t.Fatalf("last event not terminal: %+v", last)
	}
	return events, last
}

// findFailure looks up a Failure by path in the terminal Failures slice.
// Returns nil when not present so tests can assert on absence too.
func findFailure(failures []tools.Failure, path string) *tools.Failure {
	for i := range failures {
		if failures[i].Path == path {
			return &failures[i]
		}
	}
	return nil
}

// TestHashRunMixedBatchScenario exercises the situation the original PR was
// motivated by: a user selects multiple files, one of which the process
// can't read (chmod 0) and another doesn't exist. We assert the documented
// contract:
//
//   - The batch continues past the per-file failures.
//   - Per-file SeverityError events carry classified codes.
//   - The terminal event is Completed with Err == nil (partial success).
//   - Terminal Failures lists exactly the two failed paths with the right
//     codes (PERMISSION_DENIED and NOT_FOUND).
//
// This is the canonical "mixed batch" pattern — see the sibling tests in
// internal/tools/{rename,image,stripmeta,archive} for the same scenario
// against each multi-file tool, and internal/api/http for the wire end.
func TestHashRunMixedBatchScenario(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file modes are bypassed")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "ok.txt")
	writeFile(t, good, "hello")
	blocked := filepath.Join(dir, "blocked.txt")
	writeFile(t, blocked, "secret")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })
	missing := filepath.Join(dir, "ghost.txt")

	_, last := drainEvents(t, Run(context.Background(), Request{
		Sources: []string{good, blocked, missing},
		Algo:    AlgoSHA256,
	}))

	if last.Err != nil {
		t.Fatalf("expected partial-success terminal (Err=nil), got %+v", last.Err)
	}
	if len(last.Failures) != 2 {
		t.Fatalf("want 2 failures, got %d: %+v", len(last.Failures), last.Failures)
	}
	if f := findFailure(last.Failures, blocked); f == nil || f.Code != tools.CodePermissionDenied {
		t.Errorf("blocked: want PERMISSION_DENIED, got %+v", f)
	}
	if f := findFailure(last.Failures, missing); f == nil || f.Code != tools.CodeNotFound {
		t.Errorf("missing: want NOT_FOUND, got %+v", f)
	}
}

// TestHashRunUnanimousNotFoundCoalesces verifies that when every per-file
// failure shares the same Code, the terminal Err.Code surfaces that code
// instead of falling back to IO_ERROR. This is what makes
// `htools hash /missing-1 /missing-2` exit with NOT_FOUND on the wire,
// rather than IO_ERROR swallowing the actual reason.
func TestHashRunUnanimousNotFoundCoalesces(t *testing.T) {
	_, last := drainEvents(t, Run(context.Background(), Request{
		Sources: []string{"/no/such/file/1", "/no/such/file/2"},
		Algo:    AlgoSHA256,
	}))
	if last.Err == nil || last.Err.Code != tools.CodeNotFound {
		t.Fatalf("expected coalesced NOT_FOUND on terminal, got %+v", last.Err)
	}
	if len(last.Failures) != 2 {
		t.Errorf("want 2 failure entries on terminal, got %d", len(last.Failures))
	}
}

// Known-vector pairs for the algorithms we expose. "abc" was chosen because
// it appears in the official NIST / RFC test vectors for both MD5 and
// SHA-256 (and is small enough to keep this table easy to read), with the
// matching BLAKE3 vector pulled from the BLAKE3 reference test suite.
var knownVectors = []struct {
	algo   Algo
	input  string
	digest string
}{
	{AlgoMD5, "abc", "900150983cd24fb0d6963f7d28e17f72"},
	{AlgoSHA256, "abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	// BLAKE3 of "abc", computed via lukechampine.com/blake3 with the
	// default 32-byte output length.
	{AlgoBLAKE3, "abc", "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85"},
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestHashKnownVectors(t *testing.T) {
	dir := t.TempDir()
	for _, v := range knownVectors {
		v := v
		t.Run(string(v.algo), func(t *testing.T) {
			p := filepath.Join(dir, "in.bin")
			writeFile(t, p, v.input)
			res, terr := Hash(context.Background(), p, v.algo)
			if terr != nil {
				t.Fatalf("hash: %v", terr)
			}
			if res.Digest != v.digest {
				t.Errorf("%s digest = %s, want %s", v.algo, res.Digest, v.digest)
			}
			if res.Path != p {
				t.Errorf("Path = %s, want %s", res.Path, p)
			}
			if res.Algo != v.algo {
				t.Errorf("Algo = %s, want %s", res.Algo, v.algo)
			}
		})
	}
}

func TestParseAlgoRoundTrip(t *testing.T) {
	for _, s := range []string{"md5", "MD5", "sha256", "Sha256", "blake3", "BLAKE3"} {
		if _, ok := ParseAlgo(s); !ok {
			t.Errorf("ParseAlgo(%q) = false; want true", s)
		}
	}
	for _, s := range []string{"", "sha1", "sha512", "galaxy"} {
		if _, ok := ParseAlgo(s); ok {
			t.Errorf("ParseAlgo(%q) = true; want false", s)
		}
	}
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "abc")
	writeFile(t, b, "abc")

	ch := Run(context.Background(), Request{Sources: []string{a, b}, Algo: AlgoSHA256})
	var last tools.Progress
	for ev := range ch {
		last = ev
	}
	if !last.Completed || last.Err != nil {
		t.Fatalf("expected success terminal, got %+v", last)
	}
}

func TestRunRejectsEmptySources(t *testing.T) {
	ch := Run(context.Background(), Request{Algo: AlgoSHA256})
	var last tools.Progress
	for ev := range ch {
		last = ev
	}
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %+v", last)
	}
}

func TestRunRejectsUnknownAlgo(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	writeFile(t, p, "x")
	ch := Run(context.Background(), Request{Sources: []string{p}, Algo: "galaxy"})
	var last tools.Progress
	for ev := range ch {
		last = ev
	}
	if last.Err == nil || last.Err.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST for bad algo, got %+v", last)
	}
}

func TestVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Two files, both with content "abc" so the sha256 vector applies.
	for _, name := range []string{"a.txt", "b.txt"} {
		writeFile(t, filepath.Join(dir, name), "abc")
	}
	manifest := filepath.Join(dir, "SHA256SUMS")
	// canonical sha256sum format: <digest>  <relative-path>
	body := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  a.txt\n" +
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  b.txt\n"
	writeFile(t, manifest, body)

	entries, terr := Verify(context.Background(), manifest, AlgoSHA256)
	if terr != nil {
		t.Fatalf("verify: %v", terr)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if !e.OK {
			t.Errorf("%s should be OK, got %+v", e.Path, e)
		}
	}
}

func TestVerifyCatchesMismatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "abc")
	manifest := filepath.Join(dir, "SHA256SUMS")
	// Use a wrong digest so verify reports FAILED.
	writeFile(t, manifest,
		"0000000000000000000000000000000000000000000000000000000000000000  a.txt\n")

	entries, terr := Verify(context.Background(), manifest, AlgoSHA256)
	if terr != nil {
		t.Fatalf("verify: %v", terr)
	}
	if len(entries) != 1 || entries[0].OK {
		t.Fatalf("expected OK=false, got %+v", entries)
	}
}

func TestVerifyHandlesMissingFile(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "SHA256SUMS")
	writeFile(t, manifest,
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  nope.txt\n")
	entries, terr := Verify(context.Background(), manifest, AlgoSHA256)
	if terr != nil {
		t.Fatalf("verify: %v", terr)
	}
	if len(entries) != 1 || entries[0].OK || entries[0].Err == "" {
		t.Fatalf("expected per-entry error for missing file, got %+v", entries)
	}
}

func TestVerifyRejectsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "SHA256SUMS")
	writeFile(t, manifest, "this line has no separator\n")
	_, terr := Verify(context.Background(), manifest, AlgoSHA256)
	if terr == nil || terr.Code != tools.CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST for malformed line, got %+v", terr)
	}
}

func TestParseManifestLine(t *testing.T) {
	d32 := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" // 64 hex (sha256)
	d16 := "900150983cd24fb0d6963f7d28e17f72"                                 // 32 hex (md5)
	cases := []struct {
		in     string
		digest string
		path   string
		ok     bool
	}{
		{d32 + "  path/to/file", d32, "path/to/file", true},
		{d16 + " *binary.bin", d16, "binary.bin", true}, // single-space + binary marker
		{d32 + " \tpath.txt", d32, "path.txt", true},    // tab separator (single-space fallback)
		{d16 + "  *bin.dat", d16, "bin.dat", true},      // double-space + binary marker
		{"missing-separator", "", "", false},            // no whitespace at all
		{"this line has no hex prefix", "", "", false},  // first token isn't hex
		{"abc  path", "", "", false},                    // first token too short
	}
	for _, tc := range cases {
		d, p, ok := parseManifestLine(tc.in)
		if d != tc.digest || p != tc.path || ok != tc.ok {
			t.Errorf("parseManifestLine(%q) = (%q,%q,%v); want (%q,%q,%v)",
				tc.in, d, p, ok, tc.digest, tc.path, tc.ok)
		}
	}
}

// TestHashHonoursContextCancellation locks in the F2 mid-file ctx behavior:
// when ctx is already canceled before Hash is called, the function returns
// CodeAborted instead of completing the hash. The pre-cancel variant is
// fully deterministic — no timing assertions — and exercises the per-chunk
// ctx.Err() check at the head of streamInto.
func TestHashHonoursContextCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	// File must be larger than streamCopyChunk (1 MiB) so streamInto's loop
	// gets at least one full chunk and reaches the ctx.Err() check; ~2 MiB
	// of zero bytes is plenty.
	if err := os.WriteFile(path, make([]byte, 2<<20), 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, terr := Hash(ctx, path, AlgoSHA256)
	if terr == nil {
		t.Fatal("expected aborted error, got nil")
	}
	if terr.Code != tools.CodeAborted {
		t.Fatalf("expected CodeAborted, got %q (%s)", terr.Code, terr.Message)
	}
}

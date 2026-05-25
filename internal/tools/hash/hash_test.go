package hash

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

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
			res, terr := Hash(p, v.algo)
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

	entries, terr := Verify(manifest, AlgoSHA256)
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

	entries, terr := Verify(manifest, AlgoSHA256)
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
	entries, terr := Verify(manifest, AlgoSHA256)
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
	_, terr := Verify(manifest, AlgoSHA256)
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

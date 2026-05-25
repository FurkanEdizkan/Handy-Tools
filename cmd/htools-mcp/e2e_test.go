package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
)

// mcpBinaryPath is populated by TestMain — every subtest spawns this binary
// as a subprocess and talks JSON-RPC over its stdio. Building once amortises
// the compile cost across the whole table.
var mcpBinaryPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "htools-mcp-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mktemp:", err)
		os.Exit(2)
	}
	defer os.RemoveAll(dir)
	mcpBinaryPath = filepath.Join(dir, "htools-mcp")
	build := exec.Command("go", "build", "-o", mcpBinaryPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build htools-mcp: %v\n%s\n", err, out)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// lockedBuffer is a tiny sync wrapper around bytes.Buffer so the test can
// read stderr while exec.Cmd's copy goroutine is still writing into it —
// otherwise -race flags concurrent access.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *lockedBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.String()
}

// mcpClient drives htools-mcp over stdio. Pattern: build payload → write a
// newline-delimited JSON-RPC message to stdin → scan a single line back on
// stdout → decode. The MCP SDK's stdio transport is NDJSON, so a Scanner
// with a generous buffer is enough — no need for the SDK's client.
type mcpClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr *lockedBuffer
	nextID int
}

func newMCPClient(t *testing.T, allowRoots string) *mcpClient {
	t.Helper()
	args := []string{}
	if allowRoots != "" {
		args = append(args, "--allow-roots="+allowRoots)
	}
	cmd := exec.Command(mcpBinaryPath, args...) //nolint:gosec // mcpBinaryPath is built by TestMain into a controlled temp dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start htools-mcp: %v", err)
	}
	sc := bufio.NewScanner(stdout)
	// One tools/list response on its own busts the default 64KB Scanner
	// buffer because every tool's full schema serialises into a single line.
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	c := &mcpClient{cmd: cmd, stdin: stdin, stdout: sc, stderr: stderr, nextID: 1}
	c.initialize(t)
	t.Cleanup(func() { c.close() })
	return c
}

func (c *mcpClient) initialize(t *testing.T) {
	t.Helper()
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "e2e", "version": "0"},
		},
	}
	c.nextID++
	if err := c.send(initReq); err != nil {
		t.Fatalf("init send: %v", err)
	}
	if _, err := c.recv(); err != nil {
		t.Fatalf("init recv: %v", err)
	}
	if err := c.send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}); err != nil {
		t.Fatalf("initialized notification: %v", err)
	}
}

func (c *mcpClient) send(payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.stdin.Write(b)
	return err
}

func (c *mcpClient) recv() (map[string]any, error) {
	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("recv: %w (stderr: %s)", err, c.stderr.String())
		}
		return nil, fmt.Errorf("recv: stream closed (stderr: %s)", c.stderr.String())
	}
	line := c.stdout.Bytes()
	var out map[string]any
	if err := json.Unmarshal(line, &out); err != nil {
		return nil, fmt.Errorf("recv decode: %w (line: %s)", err, string(line))
	}
	return out, nil
}

// toolResult is the test-facing view of an MCP tools/call response: the
// human-readable text block plus the structured payload our drainProgress
// helper attaches as StructuredContent.
type toolResult struct {
	IsError    bool
	Text       string
	Structured map[string]any
}

func (c *mcpClient) callTool(t *testing.T, name string, args map[string]any) toolResult {
	t.Helper()
	id := c.nextID
	c.nextID++
	if err := c.send(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	}); err != nil {
		t.Fatalf("callTool %s send: %v", name, err)
	}
	resp, err := c.recv()
	if err != nil {
		t.Fatalf("callTool %s recv: %v", name, err)
	}
	if e, ok := resp["error"]; ok {
		t.Fatalf("tool %s returned protocol error: %#v", name, e)
	}
	r, _ := resp["result"].(map[string]any)
	if r == nil {
		t.Fatalf("tool %s: missing result; got %#v", name, resp)
	}
	out := toolResult{}
	if ie, ok := r["isError"].(bool); ok {
		out.IsError = ie
	}
	if content, ok := r["content"].([]any); ok {
		for _, ci := range content {
			cm, _ := ci.(map[string]any)
			if cm["type"] == "text" {
				if s, ok := cm["text"].(string); ok {
					out.Text += s
				}
			}
		}
	}
	if sc, ok := r["structuredContent"].(map[string]any); ok {
		out.Structured = sc
	}
	return out
}

func (c *mcpClient) close() {
	_ = c.stdin.Close()
	_ = c.cmd.Wait()
}

// mustAbs resolves p against the test's CWD (the package directory) so test
// invocations don't depend on the working directory.
func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func TestE2E_ImageConvert(t *testing.T) {
	client := newMCPClient(t, "/")
	src := mustAbs(t, "testdata/sample.png")
	out := filepath.Join(t.TempDir(), "out.jpg")

	res := client.callTool(t, "image_convert", map[string]any{
		"source":        src,
		"target_format": "jpeg",
		"output_file":   out,
	})
	if res.IsError {
		t.Fatalf("image_convert error:\n%s", res.Text)
	}
	f, err := os.Open(out) //nolint:gosec // path is under t.TempDir
	if err != nil {
		t.Fatalf("output JPEG missing: %v", err)
	}
	defer f.Close()
	if _, fmtName, err := image.Decode(f); err != nil {
		t.Fatalf("decode output: %v", err)
	} else if fmtName != "jpeg" {
		t.Fatalf("output not JPEG: got %s", fmtName)
	}
}

func TestE2E_ArchiveCompressThenExtract(t *testing.T) {
	client := newMCPClient(t, "/")
	helloPath := mustAbs(t, "testdata/hello.txt")
	pngPath := mustAbs(t, "testdata/sample.png")
	helloBytes, err := os.ReadFile(helloPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "bundle.zip")
	if res := client.callTool(t, "archive_compress", map[string]any{
		"sources": []string{helloPath, pngPath},
		"output":  zipPath,
		"format":  "zip",
	}); res.IsError {
		t.Fatalf("archive_compress error:\n%s", res.Text)
	}
	if info, err := os.Stat(zipPath); err != nil {
		t.Fatalf("zip not written: %v", err)
	} else if info.Size() == 0 {
		t.Fatal("zip is empty")
	}

	extractDir := t.TempDir()
	if res := client.callTool(t, "archive_extract", map[string]any{
		"source":      zipPath,
		"destination": extractDir,
		"overwrite":   true,
	}); res.IsError {
		t.Fatalf("archive_extract error:\n%s", res.Text)
	}

	// Verify both originals reappear after the round-trip, with byte-exact
	// content for the text fixture (the PNG just needs to exist; binary
	// equality is implicit because zip is lossless and we wrote it ourselves).
	var foundHello, foundPNG string
	if err := filepath.WalkDir(extractDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Base(p) {
		case "hello.txt":
			foundHello = p
		case "sample.png":
			foundPNG = p
		}
		return nil
	}); err != nil {
		t.Fatalf("walk extract dir: %v", err)
	}
	if foundHello == "" {
		t.Fatal("hello.txt not found after extract")
	}
	if foundPNG == "" {
		t.Fatal("sample.png not found after extract")
	}
	got, err := os.ReadFile(foundHello) //nolint:gosec // walked path under t.TempDir
	if err != nil {
		t.Fatalf("read extracted hello.txt: %v", err)
	}
	if !bytes.Equal(got, helloBytes) {
		t.Fatalf("hello.txt content mismatch after round-trip\ngot:  %q\nwant: %q", got, helloBytes)
	}
}

func TestE2E_PDFMerge(t *testing.T) {
	client := newMCPClient(t, "/")
	src := mustAbs(t, "testdata/sample.pdf")
	out := filepath.Join(t.TempDir(), "merged.pdf")
	if res := client.callTool(t, "pdf_merge", map[string]any{
		"sources":     []string{src, src},
		"output_file": out,
	}); res.IsError {
		t.Fatalf("pdf_merge error:\n%s", res.Text)
	}
	b, err := os.ReadFile(out) //nolint:gosec // under t.TempDir
	if err != nil {
		t.Fatalf("merged PDF missing: %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("output doesn't look like a PDF; first bytes: %q", b[:min(8, len(b))])
	}
}

func TestE2E_PDFSplit(t *testing.T) {
	client := newMCPClient(t, "/")
	src := mustAbs(t, "testdata/sample.pdf")
	outDir := t.TempDir()
	if res := client.callTool(t, "pdf_split", map[string]any{
		"source":     src,
		"output_dir": outDir,
		"every_n":    1,
	}); res.IsError {
		t.Fatalf("pdf_split error:\n%s", res.Text)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	var pdfs int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pdf") {
			pdfs++
		}
	}
	if pdfs != 2 {
		t.Fatalf("expected 2 PDFs from split (one per page), got %d (entries: %v)", pdfs, entries)
	}
}

func TestE2E_PDFRender(t *testing.T) {
	if !sysdep.Lookup("pdftoppm").Found {
		t.Skip("pdftoppm not on PATH; skipping (htools doctor will list it as missing)")
	}
	client := newMCPClient(t, "/")
	src := mustAbs(t, "testdata/sample.pdf")
	outDir := t.TempDir()
	if res := client.callTool(t, "pdf_render", map[string]any{
		"source":     src,
		"output_dir": outDir,
		"dpi":        72,
	}); res.IsError {
		t.Fatalf("pdf_render error:\n%s", res.Text)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	var pngs int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			pngs++
		}
	}
	if pngs < 1 {
		t.Fatalf("expected at least 1 rendered PNG, got %d", pngs)
	}
}

func TestE2E_PDFText(t *testing.T) {
	if !sysdep.Lookup("pdftotext").Found {
		t.Skip("pdftotext not on PATH; skipping (htools doctor will list it as missing)")
	}
	client := newMCPClient(t, "/")
	src := mustAbs(t, "testdata/sample.pdf")
	out := filepath.Join(t.TempDir(), "extracted.txt")
	if res := client.callTool(t, "pdf_text", map[string]any{
		"source":      src,
		"output_file": out,
	}); res.IsError {
		t.Fatalf("pdf_text error:\n%s", res.Text)
	}
	body, err := os.ReadFile(out) //nolint:gosec // under t.TempDir
	if err != nil {
		t.Fatalf("read extracted text: %v", err)
	}
	if !strings.Contains(string(body), "Page one") {
		t.Fatalf("expected 'Page one' in extracted text, got: %q", body)
	}
}

package http

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	_ "image/jpeg" // register the JPEG decoder for image.Decode in the download test
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/server"
	"github.com/furkandedizkan/handy-tools/internal/upload"
)

// tinyPNGBytes returns a 4x4 PNG as an in-memory byte slice.
func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// postUpload builds a multipart/form-data body from files (name -> content)
// and POSTs it to /v1/uploads. The caller owns the returned response body.
func postUpload(t *testing.T, ts *httptest.Server, files map[string][]byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, content := range files {
		fw, err := mw.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	resp, err := http.Post(ts.URL+"/v1/uploads", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("post upload: %v", err)
	}
	return resp
}

func TestUploadCreateAndConvertAndDownload(t *testing.T) {
	ts := newTestServer(t, t.TempDir())

	// 1. Upload a PNG.
	resp := postUpload(t, ts, map[string][]byte{"in.png": tinyPNGBytes(t)})
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("upload status: got %d want 200; body=%s", resp.StatusCode, raw)
	}
	var up uploadCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&up); err != nil {
		resp.Body.Close()
		t.Fatalf("decode upload response: %v", err)
	}
	resp.Body.Close()
	if up.UploadID == "" || len(up.Files) != 1 || up.OutputDir == "" {
		t.Fatalf("unexpected upload response: %+v", up)
	}
	if _, err := os.Stat(up.Files[0].Path); err != nil {
		t.Fatalf("staged file missing: %v", err)
	}

	// 2. Run the existing convert endpoint against the staged paths.
	body, _ := json.Marshal(convertRequest{
		Source:       fileRef{Path: up.Files[0].Path},
		TargetFormat: "JPEG",
		Options:      imageOptions{Quality: 80},
		Output:       outputRef{Directory: up.OutputDir, Overwrite: true},
	})
	cresp, err := http.Post(ts.URL+"/v1/image/convert", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post convert: %v", err)
	}
	if cresp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(cresp.Body)
		cresp.Body.Close()
		t.Fatalf("convert status: got %d want 202; body=%s", cresp.StatusCode, raw)
	}
	var jr jobResponse
	if err := json.NewDecoder(cresp.Body).Decode(&jr); err != nil {
		cresp.Body.Close()
		t.Fatalf("decode job: %v", err)
	}
	cresp.Body.Close()

	events := readSSE(t, ts.URL+"/v1/jobs/"+jr.JobID+"/events", 5*time.Second)
	var done bool
	for _, e := range events {
		if e.Error != nil {
			t.Fatalf("convert SSE error: %+v", e.Error)
		}
		if e.Completed {
			done = true
		}
	}
	if !done {
		t.Fatalf("convert did not complete: %+v", events)
	}

	// 3. Download the result.
	dresp, err := http.Get(ts.URL + "/v1/uploads/" + up.UploadID + "/download")
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(dresp.Body)
		t.Fatalf("download status: got %d want 200; body=%s", dresp.StatusCode, raw)
	}
	if cd := dresp.Header.Get("Content-Disposition"); cd == "" {
		t.Errorf("download missing Content-Disposition header")
	}
	out, _ := io.ReadAll(dresp.Body)
	if _, err := png.DecodeConfig(bytes.NewReader(out)); err == nil {
		t.Errorf("downloaded file decodes as PNG; expected the JPEG conversion result")
	}
	if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("downloaded result is not a decodable image: %v", err)
	}
}

func TestUploadSanitizesTraversalFilename(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp := postUpload(t, ts, map[string][]byte{"../../escape.png": tinyPNGBytes(t)})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	var up uploadCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&up); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if up.Files[0].Name != "escape.png" {
		t.Errorf("name: got %q want escape.png", up.Files[0].Name)
	}
	if dir := filepath.Dir(up.Files[0].Path); filepath.Base(dir) != "in" {
		t.Errorf("staged path %q escaped the workspace in/ directory", up.Files[0].Path)
	}
}

func TestUploadRejectsEmptyBody(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp := postUpload(t, ts, map[string][]byte{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestUploadRejectsOversizeBody(t *testing.T) {
	um, err := upload.NewManager(filepath.Join(t.TempDir(), "uploads"), 64, time.Hour)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s := New(server.Options{AllowRoots: []string{um.Base}}, queue.New(), um)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	resp := postUpload(t, ts, map[string][]byte{"big.bin": bytes.Repeat([]byte("x"), 4096)})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: got %d want 413", resp.StatusCode)
	}
	// The partially-written workspace must be cleaned up, not leaked.
	entries, err := os.ReadDir(um.Base)
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("workspace leaked after oversize upload: %v", entries)
	}
}

func TestUploadDownloadUnknownID(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/v1/uploads/nope/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestUploadDownloadBeforeRun(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp := postUpload(t, ts, map[string][]byte{"in.png": tinyPNGBytes(t)})
	var up uploadCreateResponse
	_ = json.NewDecoder(resp.Body).Decode(&up)
	resp.Body.Close()

	dresp, err := http.Get(ts.URL + "/v1/uploads/" + up.UploadID + "/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400 (no output produced)", dresp.StatusCode)
	}
}

func TestUploadDeleteThenDownload(t *testing.T) {
	ts := newTestServer(t, t.TempDir())
	resp := postUpload(t, ts, map[string][]byte{"in.png": tinyPNGBytes(t)})
	var up uploadCreateResponse
	_ = json.NewDecoder(resp.Body).Decode(&up)
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/v1/uploads/"+up.UploadID, nil)
	dresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	dresp.Body.Close()
	if dresp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status: got %d want 204", dresp.StatusCode)
	}

	gresp, err := http.Get(ts.URL + "/v1/uploads/" + up.UploadID + "/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer gresp.Body.Close()
	if gresp.StatusCode != http.StatusBadRequest {
		t.Fatalf("download after delete: got %d want 400", gresp.StatusCode)
	}
}

func TestUploadDownloadMultiFileZip(t *testing.T) {
	um := newTestUploadManager(t)
	s := New(server.Options{AllowRoots: []string{um.Base}}, queue.New(), um)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	ws, err := um.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Simulate a tool that wrote two output files into the workspace.
	for name, content := range map[string]string{"one.txt": "alpha", "two.txt": "bravo"} {
		if err := os.WriteFile(filepath.Join(ws.OutDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write output: %v", err)
		}
	}

	resp, err := http.Get(ts.URL + "/v1/uploads/" + ws.ID + "/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("Content-Type: got %q want application/zip", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("zip entries: got %d want 2", len(zr.File))
	}
}

// TestUploadRoutesAbsentWithoutManager confirms a nil upload Manager leaves
// the /v1/uploads routes unregistered (the path-only / desktop posture).
func TestUploadRoutesAbsentWithoutManager(t *testing.T) {
	s := New(server.Options{AllowRoots: []string{t.TempDir()}}, queue.New(), nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	resp := postUpload(t, ts, map[string][]byte{"in.png": tinyPNGBytes(t)})
	defer resp.Body.Close()
	// With no /v1/uploads route, the SPA catch-all answers — anything but a
	// 200 upload envelope. Assert we did not get the upload contract.
	if resp.StatusCode == http.StatusOK {
		var up uploadCreateResponse
		if json.NewDecoder(resp.Body).Decode(&up) == nil && up.UploadID != "" {
			t.Fatal("upload route served despite nil Manager")
		}
	}
}

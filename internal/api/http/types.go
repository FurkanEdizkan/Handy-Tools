// Package http exposes the Handy Tools toolset over HTTP + Server-Sent Events.
//
// It is a sibling transport to internal/server (gRPC) and reuses the same
// internal/server.{Image,Archive,PDF}Handler types so the path-safety and
// tool-driving logic stays in one place. The HTTP layer only owns wire format
// (JSON), routing, and SSE framing.
//
// Wire shape (snake_case to mirror the proto):
//
//	POST /v1/image/convert            → 202 {"job_id": "..."}
//	POST /v1/archive/inspect          → 200 {Inspection}
//	POST /v1/archive/extract          → 202 {"job_id": "..."}
//	POST /v1/pdf/to-image             → 202 {"job_id": "..."}
//	POST /v1/pdf/to-text              → 202 {"job_id": "..."}
//	POST /v1/pdf/merge                → 202 {"job_id": "..."}
//	GET  /v1/jobs/{id}/events         → text/event-stream of Progress
//	GET  /v1/sysdep                   → 200 [SysdepResult, ...]
//	GET  /v1/config                   → 200 {Config}
//	PATCH /v1/config                  → 200 {Config} (partial body, deep-merged)
package http

// fileRef mirrors handytools.v1.FileRef.
type fileRef struct {
	Path string `json:"path"`
}

// outputRef mirrors handytools.v1.OutputRef.
type outputRef struct {
	File      string `json:"file,omitempty"`
	Directory string `json:"directory,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// pageRange mirrors handytools.v1.PageRange.
type pageRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// imageOptions mirrors handytools.v1.ImageOptions.
type imageOptions struct {
	Quality       int  `json:"quality"`
	MaxWidth      int  `json:"max_width"`
	MaxHeight     int  `json:"max_height"`
	StripMetadata bool `json:"strip_metadata"`
}

// convertRequest is the body of POST /v1/image/convert.
type convertRequest struct {
	Source       fileRef      `json:"source"`
	TargetFormat string       `json:"target_format"` // PNG|JPEG|GIF|BMP|TIFF|WEBP|HEIC
	Options      imageOptions `json:"options"`
	Output       outputRef    `json:"output"`
}

// batchConvertRequest is the body of POST /v1/image/batch-convert. Every
// source is converted to TargetFormat with the same Options into Output.
type batchConvertRequest struct {
	Sources      []fileRef    `json:"sources"`
	TargetFormat string       `json:"target_format"` // PNG|JPEG|GIF|BMP|TIFF|WEBP|HEIC
	Options      imageOptions `json:"options"`
	Output       outputRef    `json:"output"`
}

// inspectRequest is the body of POST /v1/archive/inspect.
type inspectRequest struct {
	Source fileRef `json:"source"`
}

// inspectResponse is the body of 200 from POST /v1/archive/inspect.
type inspectResponse struct {
	Format                string   `json:"format"`
	MultiPart             bool     `json:"multi_part"`
	DetectedParts         []string `json:"detected_parts"`
	MissingParts          []string `json:"missing_parts"`
	UncompressedSizeBytes int64    `json:"uncompressed_size_bytes"`
	EntryCount            int      `json:"entry_count"`
	RequiresPassword      bool     `json:"requires_password"`
	RequiresBinary        string   `json:"requires_binary"` // "unrar" / "7z" / "" if pure-Go path is available
	BinaryAvailable       bool     `json:"binary_available"`
}

// extractRequest is the body of POST /v1/archive/extract.
type extractRequest struct {
	Source              fileRef   `json:"source"`
	Parts               []fileRef `json:"parts"`
	DestinationDir      string    `json:"destination_dir"`
	Password            string    `json:"password"`
	Overwrite           bool      `json:"overwrite"`
	AutoAcceptMultiPart bool      `json:"auto_accept_multi_part"`
}

// compressRequest is the body of POST /v1/archive/compress.
type compressRequest struct {
	Sources          []fileRef `json:"sources"`
	Destination      outputRef `json:"destination"`        // File is the archive path
	Format           string    `json:"format"`             // ZIP|TAR|TAR_GZ|TAR_BZ2|TAR_ZST|SEVENZ; "" = infer from extension
	CompressionLevel int       `json:"compression_level"`  // 0 = format default, 1..9
	Password         string    `json:"password,omitempty"` // optional; pure-Go formats reject it
}

// pdfToImageRequest is the body of POST /v1/pdf/to-image.
type pdfToImageRequest struct {
	Source       fileRef   `json:"source"`
	Pages        pageRange `json:"pages"`
	DPI          int       `json:"dpi"`
	TargetFormat string    `json:"target_format"` // "JPEG" or "PNG"; default PNG
	Output       outputRef `json:"output"`
}

// pdfToTextRequest is the body of POST /v1/pdf/to-text.
type pdfToTextRequest struct {
	Source fileRef   `json:"source"`
	Pages  pageRange `json:"pages"`
	Layout bool      `json:"layout"`
	Output outputRef `json:"output"`
}

// pdfMergeRequest is the body of POST /v1/pdf/merge.
type pdfMergeRequest struct {
	Sources []fileRef `json:"sources"`
	Output  outputRef `json:"output"`
}

// pdfSplitRequest is the body of POST /v1/pdf/split. page_ranges and every_n
// are mutually exclusive — exactly one mode must be set.
type pdfSplitRequest struct {
	Source     fileRef     `json:"source"`
	PageRanges []pageRange `json:"page_ranges"`
	EveryN     int         `json:"every_n"`
	Output     outputRef   `json:"output"` // Directory is where the split files land
}

// jobResponse is the body of 202 from any async POST endpoint.
type jobResponse struct {
	JobID string `json:"job_id"`
}

// uploadedFile is one staged file in an uploadCreateResponse. Path is the
// server-side absolute path the client passes back as a fileRef.path.
type uploadedFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// uploadCreateResponse is the body of 200 from POST /v1/uploads. The browser
// feeds Files[].Path as tool sources and OutputDir as the tool output
// directory, then GETs /v1/uploads/{upload_id}/download for the result.
type uploadCreateResponse struct {
	UploadID  string         `json:"upload_id"`
	Files     []uploadedFile `json:"files"`
	OutputDir string         `json:"output_dir"`
}

// jobSummary is the JSON shape of one job — used both in the GET /v1/jobs
// list and in each GET /v1/jobs/events SSE frame.
type jobSummary struct {
	JobID       string         `json:"job_id"`
	Tool        string         `json:"tool"`
	Action      string         `json:"action"`
	StartedAt   int64          `json:"started_unix_ms,omitempty"`
	Status      string         `json:"status"` // queued|running|done|failed
	Fraction    float64        `json:"fraction,omitempty"`
	CurrentItem string         `json:"current_item,omitempty"`
	Message     string         `json:"message,omitempty"`
	Completed   bool           `json:"completed"`
	Error       *errorEnvelope `json:"error,omitempty"`
}

// jobsResponse is the body of 200 from GET /v1/jobs.
type jobsResponse struct {
	Jobs []jobSummary `json:"jobs"`
}

// progressEvent is the JSON shape written into each SSE data: frame.
type progressEvent struct {
	JobID       string         `json:"job_id"`
	Tool        string         `json:"tool"`
	Action      string         `json:"action"`
	StartedAt   int64          `json:"started_unix_ms,omitempty"`
	CurrentItem string         `json:"current_item,omitempty"`
	BytesDone   int64          `json:"bytes_done,omitempty"`
	BytesTotal  int64          `json:"bytes_total,omitempty"`
	Fraction    float64        `json:"fraction,omitempty"`
	Level       string         `json:"level,omitempty"` // INFO|WARNING|ERROR
	Message     string         `json:"message,omitempty"`
	Completed   bool           `json:"completed"`
	Error       *errorEnvelope `json:"error,omitempty"`
}

// healthResponse is the body of 200 from GET /v1/health.
type healthResponse struct {
	Version        string   `json:"version"`
	UptimeSeconds  int64    `json:"uptime_seconds"`
	Transports     []string `json:"transports"`
	ToolsAvailable []string `json:"tools_available"`
}

// sysdepResult mirrors sysdep.Result for the wire.
type sysdepResult struct {
	Name        string            `json:"name"`
	Found       bool              `json:"found"`
	Path        string            `json:"path,omitempty"`
	UsedAlias   string            `json:"used_alias,omitempty"`
	Description string            `json:"description,omitempty"`
	Features    []string          `json:"features,omitempty"`
	InstallHint map[string]string `json:"install_hint,omitempty"`
}

package image

import (
	"bytes"
	"image/png"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// DefaultPreviewBound is the thumbnail bounding box used when a PreviewRequest
// leaves both MaxWidth and MaxHeight unset.
const DefaultPreviewBound = 320

// PreviewRequest asks for a downscaled thumbnail of a single image.
type PreviewRequest struct {
	Source    string
	MaxWidth  int // 0 → DefaultPreviewBound
	MaxHeight int // 0 → DefaultPreviewBound
}

// PreviewResult is a PNG-encoded thumbnail and its pixel dimensions.
type PreviewResult struct {
	PNG    []byte
	Width  int
	Height int
}

// Preview decodes Source, scales it down to fit the requested bounds (never
// up), and returns it PNG-encoded in memory — nothing is written to disk, so
// the web gallery can show a thumbnail without a filesystem round trip. It
// reuses the same decoder as Convert, so every supported input format works
// (HEIC included, when ImageMagick is present).
func Preview(req PreviewRequest) (PreviewResult, error) {
	img, err := decode(req.Source)
	if err != nil {
		return PreviewResult{}, &tools.Error{
			Code: tools.ClassifyFSError(err), Message: "decode failed", Detail: err.Error(),
		}
	}
	w, h := req.MaxWidth, req.MaxHeight
	if w <= 0 && h <= 0 {
		w, h = DefaultPreviewBound, DefaultPreviewBound
	}
	img = resize(img, w, h)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return PreviewResult{}, &tools.Error{
			Code: tools.CodeIO, Message: "encode failed", Detail: err.Error(),
		}
	}
	b := img.Bounds()
	return PreviewResult{PNG: buf.Bytes(), Width: b.Dx(), Height: b.Dy()}, nil
}

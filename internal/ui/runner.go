// runner.go bridges a RunJob from the tool page to the real internal/tools
// packages. It is the production replacement for the #77a simulator: with
// real file paths (#160) and a real option payload (#163) on RunJob, the
// runner maps the job onto a tool request and forwards the tool's progress
// channel to the shared queue.
//
// The TUI calls internal/tools directly — not the internal/server handlers,
// whose Options.CheckPath fail-closed sandbox is a server-only concern; a
// local TUI already has the user's filesystem access.
package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	jobqueue "github.com/furkandedizkan/handy-tools/internal/queue"
	"github.com/furkandedizkan/handy-tools/internal/tools"
	"github.com/furkandedizkan/handy-tools/internal/tools/archive"
	"github.com/furkandedizkan/handy-tools/internal/tools/image"
	"github.com/furkandedizkan/handy-tools/internal/tools/pdf"
)

// realRunner builds the queue.Runner that executes a job's tool for real.
func realRunner(j RunJob) jobqueue.Runner {
	items := realFileItems(j.Files)
	return func(ctx context.Context, emit func(tools.Progress)) {
		if len(items) == 0 {
			emit(failedProgress("no files selected — press b on the tool page to add one"))
			return
		}
		switch j.Tool.mode {
		case modeImage:
			runImage(ctx, j, items, emit)
		case modePackArchive, modeExtractArchive:
			if j.ArchiveMode == archivePack {
				runArchivePack(ctx, j, items, emit)
			} else {
				runArchiveExtract(ctx, j, items, emit)
			}
		case modePDF:
			runPDF(ctx, j, items, emit)
		default:
			emit(failedProgress("this tool cannot run jobs"))
		}
	}
}

// runAction is the short action label stamped on the queued job.
func runAction(j RunJob) string {
	switch j.Tool.mode {
	case modeImage:
		return "convert"
	case modePackArchive, modeExtractArchive:
		if j.ArchiveMode == archivePack {
			return "compress"
		}
		return "extract"
	case modePDF:
		switch j.PDFOp {
		case pdfSplit:
			return "split"
		case pdfRender:
			return "render"
		case pdfText:
			return "to-text"
		default:
			return "merge"
		}
	}
	return "run"
}

// runImage converts each input file, honoring its per-row target format.
func runImage(ctx context.Context, j RunJob, items []fileItem, emit func(tools.Progress)) {
	opts := image.Options{Quality: j.Quality}
	for _, f := range items {
		if ctx.Err() != nil {
			return
		}
		dir := outputDir(j, f.Path)
		if !prepareDir(dir, emit) {
			return
		}
		format, ok := imageFormatFromName(f.Target)
		if !ok {
			format = image.FormatJPEG
		}
		ch := image.Convert(ctx, image.ConvertRequest{
			Source:       f.Path,
			TargetFormat: format,
			Opts:         opts,
			Output:       dir,
			Overwrite:    j.Overwrite,
		})
		for p := range ch {
			emit(p)
		}
	}
}

// prepareDir creates the output directory, emitting a terminal error and
// reporting false if it cannot. internal/tools resolves a not-yet-existing
// Output as a file path, so the directory must exist first.
func prepareDir(dir string, emit func(tools.Progress)) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		emit(failedProgress("cannot create output directory: " + err.Error()))
		return false
	}
	return true
}

// runArchivePack packs every input into a single archive.
func runArchivePack(ctx context.Context, j RunJob, items []fileItem, emit func(tools.Progress)) {
	ext := j.ArchiveOut
	if ext == "" {
		ext = "zip"
	}
	dir := outputDir(j, items[0].Path)
	if !prepareDir(dir, emit) {
		return
	}
	out := filepath.Join(dir, "archive."+ext)
	ch := archive.Compress(ctx, archive.CompressRequest{
		Sources:          paths(items),
		Format:           archiveFormatFromName(j.ArchiveOut),
		Output:           out,
		CompressionLevel: j.CompressionLevel,
	})
	for p := range ch {
		emit(p)
	}
}

// runArchiveExtract extracts the first selected archive.
func runArchiveExtract(ctx context.Context, j RunJob, items []fileItem, emit func(tools.Progress)) {
	src := items[0].Path
	dir := outputDir(j, src)
	if !prepareDir(dir, emit) {
		return
	}
	ch := archive.Extract(ctx, archive.ExtractRequest{
		Source:        src,
		Destination:   dir,
		Overwrite:     j.Overwrite,
		AutoMultiPart: j.AutoMultiPart,
	})
	for p := range ch {
		emit(p)
	}
}

// runPDF dispatches the four PDF operations.
func runPDF(ctx context.Context, j RunJob, items []fileItem, emit func(tools.Progress)) {
	src := items[0].Path
	dir := outputDir(j, src)
	if !prepareDir(dir, emit) {
		return
	}
	stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	var ch <-chan tools.Progress
	switch j.PDFOp {
	case pdfSplit:
		ch = pdf.Split(ctx, pdf.SplitRequest{Source: src, EveryN: j.EveryN, OutputDir: dir})
	case pdfRender:
		ch = pdf.ToImage(ctx, pdf.ToImageRequest{Source: src, DPI: j.DPI, OutputDir: dir, JPEG: j.PDFJpeg})
	case pdfText:
		ch = pdf.ToText(ctx, pdf.ToTextRequest{
			Source: src, OutputFile: filepath.Join(dir, stem+".txt"), Layout: j.PDFLayout,
		})
	default: // pdfMerge
		ch = pdf.Merge(ctx, pdf.MergeRequest{
			Sources: paths(items), OutputFile: filepath.Join(dir, "merged.pdf"),
		})
	}
	for p := range ch {
		emit(p)
	}
}

// outputDir resolves where a job writes, from the output-destination radio.
func outputDir(j RunJob, src string) string {
	switch j.Out {
	case outAlongside:
		return filepath.Dir(src)
	case outCustom:
		if j.CustomPath != "" {
			return j.CustomPath
		}
	}
	return "out" // outDefault — ./out relative to the working directory
}

// realFileItems keeps only files with a real path — the demo sample rows have
// an empty Path and are skipped.
func realFileItems(files []fileItem) []fileItem {
	out := make([]fileItem, 0, len(files))
	for _, f := range files {
		if f.Path != "" {
			out = append(out, f)
		}
	}
	return out
}

func paths(items []fileItem) []string {
	out := make([]string, len(items))
	for i, f := range items {
		out[i] = f.Path
	}
	return out
}

func failedProgress(msg string) tools.Progress {
	return tools.Progress{
		Completed: true,
		Err:       &tools.Error{Code: tools.CodeBadRequest, Message: msg},
	}
}

// imageFormatFromName maps a tool-page target label to an image.Format.
func imageFormatFromName(s string) (image.Format, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PNG":
		return image.FormatPNG, true
	case "JPEG", "JPG":
		return image.FormatJPEG, true
	case "GIF":
		return image.FormatGIF, true
	case "BMP":
		return image.FormatBMP, true
	case "TIFF", "TIF":
		return image.FormatTIFF, true
	case "WEBP":
		return image.FormatWebP, true
	case "HEIC", "HEIF":
		return image.FormatHEIC, true
	}
	return image.FormatUnspecified, false
}

// archiveFormatFromName maps the pack-mode format string to an archive.Format.
// An unrecognized value yields FormatUnknown, which archive.Compress resolves
// from the output-file extension.
func archiveFormatFromName(s string) archive.Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "zip":
		return archive.FormatZip
	case "tar":
		return archive.FormatTar
	case "tar.gz", "tgz", "gz":
		return archive.FormatTarGz
	case "tar.bz2", "bz2":
		return archive.FormatTarBz2
	case "tar.zst", "zst", "zstd":
		return archive.FormatTarZst
	case "7z":
		return archive.FormatSevenZ
	}
	return archive.FormatUnknown
}

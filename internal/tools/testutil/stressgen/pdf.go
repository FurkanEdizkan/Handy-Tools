package stressgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// PDFSet writes count tiny multi-page PDFs into dir, each with `pages` pages.
// Used as the input to the pdf.Merge bench. PDFs are valid and byte-stable
// across runs (deterministic). Returns absolute paths in order.
func PDFSet(dir string, count, pages int, _ uint64) ([]string, error) {
	if err := MkdirAll(dir); err != nil {
		return nil, err
	}
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("doc_%03d.pdf", i))
		paths[i] = path
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			continue
		}
		if err := writeMinimalPDF(path, pages, fmt.Sprintf("doc %03d", i)); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// PDFLarge writes a single PDF with `pages` pages into dir. Used as input
// to pdf.Split, pdf.Render, pdf.Text — operations whose cost scales with
// page count.
func PDFLarge(dir string, pages int) (string, error) {
	if err := MkdirAll(dir); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("large_%dp.pdf", pages))
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil
	}
	if err := writeMinimalPDF(path, pages, "large"); err != nil {
		return "", err
	}
	return path, nil
}

// writeMinimalPDF emits a valid PDF with `pages` pages, each carrying a
// short label string. We hand-build the bytes rather than pull in a PDF
// library — the structure is small and stable, the project already has
// hand-built PDF fixtures in pdf/testdata, and an extra build dep would be
// gratuitous for what is effectively a byte template.
//
// Object layout (forward references resolved by allocating numbers up front):
//
//	1: Catalog
//	2: Pages
//	3: Font (Helvetica)
//	4: Contents-1
//	5: Page-1
//	6: Contents-2
//	7: Page-2
//	...
func writeMinimalPDF(path string, pages int, label string) error {
	if pages < 1 {
		pages = 1
	}
	const (
		catalogID = 1
		pagesID   = 2
		fontID    = 3
	)
	// Allocate object numbers per page: contents N, page N+1.
	contentID := func(p int) int { return fontID + 1 + p*2 }
	pageID := func(p int) int { return fontID + 2 + p*2 }
	totalObjects := fontID + pages*2

	var buf bytes.Buffer
	offsets := make([]int, totalObjects+1) // index 0 unused (PDF objects 1-based)

	writeObj := func(id int, body string) {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	writeObj(catalogID, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesID))

	// Pages object — list every page's object number as /Kids.
	var kids bytes.Buffer
	for p := 0; p < pages; p++ {
		if p > 0 {
			kids.WriteByte(' ')
		}
		fmt.Fprintf(&kids, "%d 0 R", pageID(p))
	}
	writeObj(pagesID, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), pages))

	writeObj(fontID, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	for p := 0; p < pages; p++ {
		stream := fmt.Sprintf("BT /F1 14 Tf 72 720 Td (%s page %d) Tj ET\n", label, p+1)
		writeObj(contentID(p),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
		writeObj(pageID(p), fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] "+
				"/Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>",
			pagesID, contentID(p), fontID))
	}

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", totalObjects+1)
	buf.WriteString("0000000000 65535 f \n")
	for id := 1; id <= totalObjects; id++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		totalObjects+1, catalogID, xrefOffset)

	return os.WriteFile(path, buf.Bytes(), 0o644) //nolint:gosec // deterministic fixture
}

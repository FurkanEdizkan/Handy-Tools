// toolpage renders the per-tool detail screen described in the design:
// dropzone → file list with per-row format override → output destination
// radio → options grid → big Run button.
//
// In a real TUI we can't accept literal drag-and-drop, so the "dropzone" is
// rendered as a styled prompt with sample files pre-populated (matching the
// JSX prototype). Keyboard alone drives selection, format cycling, toggles,
// and the run trigger.
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/ui/theme"
)

// imageFormats tracks the cycle order used by the design's image-mode
// fmt-select dropdown. (Archive output formats use a hardcoded default
// for now; keyboard cycling for archive format is in the project backlog.)
var imageFormats = []string{"JPEG", "PNG", "WebP", "GIF", "BMP", "TIFF"}

// outDest is the 3-way output destination picker.
type outDest int

const (
	outDefault outDest = iota
	outAlongside
	outCustom
)

// fileItem is one row in the per-tool file list. Optional From/Target fields
// drive the per-file format override column shown only in image mode.
type fileItem struct {
	ID     string
	Name   string
	From   string // source format display ("PNG", "JPEG", "7z (multi-part)" …)
	Target string // current target format ("JPEG", "WebP", …) — image mode only
	Size   string
}

// archiveMode is the inner Pack/Extract toggle inside the Archive tool page.
type archiveMode int

const (
	archivePack archiveMode = iota
	archiveExtract
)

// pdfOp is the PDF utility operation picker.
type pdfOp int

const (
	pdfMerge pdfOp = iota
	pdfSplit
	pdfRender
	pdfText
)

func (p pdfOp) Label() string {
	switch p {
	case pdfSplit:
		return "Split"
	case pdfRender:
		return "Pages → image"
	case pdfText:
		return "Extract text"
	}
	return "Merge"
}

// focusable section indices on the tool page — controls which row up/down
// move through.
type focusKind int

const (
	focusInput   focusKind = iota // dropzone (Browse / drop simulation)
	focusFile                     // one of the file rows
	focusOutDest                  // one of the three output rows
	focusOptions                  // one of the option rows
	focusRun                      // the Run button
)

// toolPage is the right-column screen for a single tool.
type toolPage struct {
	styles theme.Styles
	width  int

	tool tool

	archive archiveMode
	pdfop   pdfOp

	files      []fileItem
	defaultFmt string // default image-mode target
	archiveOut string // pack-mode output format

	// focus rotation: (kind, index)
	focusKind focusKind
	focusIdx  int

	// output destination
	out        outDest
	customPath string

	// options
	quality          int
	overwrite        bool
	preserveMtime    bool
	recurse          bool
	compressionLevel int
	dpi              int
}

// newToolPage builds a populated page for the given tool id.
func newToolPage(s theme.Styles, t tool) *toolPage {
	p := &toolPage{
		styles:           s,
		tool:             t,
		defaultFmt:       "JPEG",
		archiveOut:       "zip",
		out:              outDefault,
		customPath:       "/Users/me/converted",
		quality:          90,
		preserveMtime:    true,
		recurse:          true,
		compressionLevel: 6,
		dpi:              150,
		focusKind:        focusInput,
	}
	if t.mode == modeExtractArchive {
		p.archive = archiveExtract
	}
	p.files = p.sampleFiles()
	return p
}

func (p *toolPage) SetWidth(w int) { p.width = w }
func (p *toolPage) Tool() tool     { return p.tool }

// sampleFiles returns the design's pre-populated file rows for the current
// mode. Real input wiring would replace this with the user's actual selection.
func (p *toolPage) sampleFiles() []fileItem {
	switch p.tool.mode {
	case modeImage:
		return []fileItem{
			{ID: "i1", Name: "screenshot-2026-05-14.png", From: "PNG", Target: "JPEG", Size: "2.1 MB"},
			{ID: "i2", Name: "logo-mark.png", From: "PNG", Target: "JPEG", Size: "184 KB"},
			{ID: "i3", Name: "export@2x.png", From: "PNG", Target: "WebP", Size: "5.4 MB"},
			{ID: "i4", Name: "cover-shot.jpg", From: "JPEG", Target: "JPEG", Size: "1.8 MB"},
		}
	case modePackArchive:
		return []fileItem{
			{ID: "p1", Name: "report-q1.pdf", Size: "482 KB"},
			{ID: "p2", Name: "cover.png", Size: "1.1 MB"},
			{ID: "p3", Name: "data/raw.csv", Size: "904 KB"},
			{ID: "p4", Name: "data/clean.csv", Size: "612 KB"},
			{ID: "p5", Name: "notes.md", Size: "12 KB"},
			{ID: "p6", Name: "README.md", Size: "4 KB"},
		}
	case modeExtractArchive:
		return []fileItem{
			{ID: "x1", Name: "backup-2026-05.7z.001", From: "7z (multi-part)", Size: "42 MB"},
		}
	case modePDF:
		return []fileItem{
			{ID: "d1", Name: "chapter-01.pdf", Size: "212 KB"},
			{ID: "d2", Name: "chapter-02.pdf", Size: "188 KB"},
			{ID: "d3", Name: "appendix.pdf", Size: "96 KB"},
		}
	}
	return nil
}

// Update routes keys for the tool page. Returns a RunJob message when the
// user triggers the Run button.
func (p *toolPage) Update(msg tea.Msg) (*toolPage, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch k.String() {
	case "esc":
		return p, func() tea.Msg { return GoHome{} }
	case "tab":
		p.cycleFocus(+1)
	case "shift+tab":
		p.cycleFocus(-1)
	case "down", "j":
		p.cycleFocus(+1)
	case "up", "k":
		p.cycleFocus(-1)
	case "f":
		p.cycleFormat()
	case "F":
		p.applyDefaultToAll()
	case "o":
		p.cycleOutDest()
	case "p":
		p.cyclePackExtract()
	case "P":
		p.cyclePDFOp()
	case " ", "enter":
		if p.focusKind == focusRun {
			return p, p.runCmd()
		}
		p.toggleAtFocus()
	case "+", "=", "right", "l":
		p.adjustOption(+1)
	case "-", "_", "left", "h":
		p.adjustOption(-1)
	case "r":
		return p, p.runCmd()
	}
	return p, nil
}

func (p *toolPage) runCmd() tea.Cmd {
	files := append([]fileItem(nil), p.files...)
	summary := p.summary()
	tool := p.tool
	archive := p.archive
	archiveOut := p.archiveOut
	pdfop := p.pdfop
	outMode := p.out
	customPath := p.customPath
	return func() tea.Msg {
		return RunJob{
			Tool:        tool,
			Files:       files,
			Summary:     summary,
			ArchiveMode: archive,
			ArchiveOut:  archiveOut,
			PDFOp:       pdfop,
			Out:         outMode,
			CustomPath:  customPath,
		}
	}
}

// cycleFocus walks the focus across the visible sections.
func (p *toolPage) cycleFocus(dir int) {
	maxFile := len(p.files) - 1
	switch p.focusKind {
	case focusInput:
		if dir > 0 {
			if maxFile >= 0 {
				p.focusKind, p.focusIdx = focusFile, 0
			} else {
				p.focusKind, p.focusIdx = focusOutDest, 0
			}
		}
	case focusFile:
		if dir > 0 {
			if p.focusIdx < maxFile {
				p.focusIdx++
			} else {
				p.focusKind, p.focusIdx = focusOutDest, 0
			}
		} else {
			if p.focusIdx > 0 {
				p.focusIdx--
			} else {
				p.focusKind = focusInput
			}
		}
	case focusOutDest:
		if dir > 0 {
			if p.focusIdx < 2 {
				p.focusIdx++
			} else {
				p.focusKind, p.focusIdx = focusOptions, 0
			}
		} else {
			if p.focusIdx > 0 {
				p.focusIdx--
			} else if maxFile >= 0 {
				p.focusKind, p.focusIdx = focusFile, maxFile
			} else {
				p.focusKind = focusInput
			}
		}
	case focusOptions:
		n := p.optionCount()
		if dir > 0 {
			if p.focusIdx < n-1 {
				p.focusIdx++
			} else {
				p.focusKind = focusRun
			}
		} else {
			if p.focusIdx > 0 {
				p.focusIdx--
			} else {
				p.focusKind, p.focusIdx = focusOutDest, 2
			}
		}
	case focusRun:
		if dir < 0 {
			p.focusKind, p.focusIdx = focusOptions, p.optionCount()-1
		}
	}
}

// optionCount is the number of option rows for the current mode.
func (p *toolPage) optionCount() int {
	switch p.tool.mode {
	case modeImage:
		return 4
	case modePackArchive, modeExtractArchive:
		return 4
	case modePDF:
		return 2
	}
	return 0
}

// cycleFormat moves the focused file's target format to the next in the list.
func (p *toolPage) cycleFormat() {
	if p.tool.mode != modeImage || p.focusKind != focusFile {
		return
	}
	f := &p.files[p.focusIdx]
	for i, name := range imageFormats {
		if name == f.Target {
			f.Target = imageFormats[(i+1)%len(imageFormats)]
			return
		}
	}
	f.Target = imageFormats[0]
}

// applyDefaultToAll sets every file's target to defaultFmt (image mode).
func (p *toolPage) applyDefaultToAll() {
	if p.tool.mode != modeImage {
		return
	}
	for i := range p.files {
		p.files[i].Target = p.defaultFmt
	}
}

func (p *toolPage) cycleOutDest() {
	p.out = outDest((int(p.out) + 1) % 3)
}

func (p *toolPage) cyclePackExtract() {
	if p.tool.mode != modePackArchive && p.tool.mode != modeExtractArchive {
		return
	}
	if p.archive == archivePack {
		p.archive = archiveExtract
	} else {
		p.archive = archivePack
	}
	p.files = p.sampleFilesForArchive()
}

func (p *toolPage) sampleFilesForArchive() []fileItem {
	if p.archive == archiveExtract {
		return []fileItem{{ID: "x1", Name: "backup-2026-05.7z.001", From: "7z (multi-part)", Size: "42 MB"}}
	}
	return []fileItem{
		{ID: "p1", Name: "report-q1.pdf", Size: "482 KB"},
		{ID: "p2", Name: "cover.png", Size: "1.1 MB"},
		{ID: "p3", Name: "data/raw.csv", Size: "904 KB"},
		{ID: "p4", Name: "data/clean.csv", Size: "612 KB"},
		{ID: "p5", Name: "notes.md", Size: "12 KB"},
		{ID: "p6", Name: "README.md", Size: "4 KB"},
	}
}

func (p *toolPage) cyclePDFOp() {
	if p.tool.mode != modePDF {
		return
	}
	p.pdfop = pdfOp((int(p.pdfop) + 1) % 4)
}

// toggleAtFocus is the SPACE/ENTER action on any focused row.
func (p *toolPage) toggleAtFocus() {
	switch p.focusKind {
	case focusOutDest:
		p.out = outDest(p.focusIdx)
	case focusOptions:
		p.toggleOption()
	}
}

func (p *toolPage) toggleOption() {
	switch p.tool.mode {
	case modeImage:
		switch p.focusIdx {
		case 1:
			p.overwrite = !p.overwrite
		case 2:
			p.preserveMtime = !p.preserveMtime
		case 3:
			p.recurse = !p.recurse
		}
	case modePackArchive, modeExtractArchive:
		// slider on row 0, three toggles on rows 1..3
		if p.focusIdx == 1 {
			p.overwrite = !p.overwrite
		}
		if p.focusIdx == 2 {
			p.preserveMtime = !p.preserveMtime
		}
		if p.focusIdx == 3 {
			p.recurse = !p.recurse
		}
	}
}

func (p *toolPage) adjustOption(dir int) {
	if p.focusKind != focusOptions {
		return
	}
	switch p.tool.mode {
	case modeImage:
		if p.focusIdx == 0 {
			p.quality = clamp(p.quality+dir*5, 50, 100)
		}
	case modePackArchive, modeExtractArchive:
		if p.focusIdx == 0 {
			p.compressionLevel = clamp(p.compressionLevel+dir, 0, 9)
		}
	case modePDF:
		if p.focusIdx == 0 && p.pdfop == pdfRender {
			p.dpi = clamp(p.dpi+dir*12, 72, 600)
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// summary mirrors the JSX run-summary line ("3 → JPEG · 1 → WebP" etc.).
func (p *toolPage) summary() string {
	switch p.tool.mode {
	case modeImage:
		counts := map[string]int{}
		order := []string{}
		for _, f := range p.files {
			if _, ok := counts[f.Target]; !ok {
				order = append(order, f.Target)
			}
			counts[f.Target]++
		}
		parts := make([]string, 0, len(order))
		for _, k := range order {
			parts = append(parts, itoa(counts[k])+" → "+k)
		}
		return strings.Join(parts, " · ")
	case modePackArchive, modeExtractArchive:
		if p.archive == archivePack {
			n := len(p.files)
			noun := "items"
			if n == 1 {
				noun = "item"
			}
			return itoa(n) + " " + noun + " → 1 " + p.archiveOut + " archive"
		}
		return itoa(len(p.files)) + " archive(s) → extracted folder"
	case modePDF:
		n := len(p.files)
		noun := "documents"
		if n == 1 {
			noun = "document"
		}
		return strings.ToLower(p.pdfop.Label()) + " · " + itoa(n) + " " + noun
	case modeDoctor:
		return "system check"
	}
	return ""
}

// View renders the whole tool page.
func (p *toolPage) View() string {
	if p.tool.mode == modeDoctor {
		return p.renderDoctor()
	}
	s := p.styles
	sections := []string{p.renderHeader()}

	if p.tool.mode == modePackArchive || p.tool.mode == modeExtractArchive {
		sections = append(sections, p.renderArchiveToggle())
	}
	if p.tool.mode == modePDF {
		sections = append(sections, p.renderPDFOpPicker())
	}

	sections = append(sections,
		p.renderInput(),
		p.renderFiles(),
		p.renderOutDest(),
		p.renderOptions(),
		p.renderRunRow(),
	)
	_ = s
	return strings.Join(sections, "\n\n")
}

func (p *toolPage) renderHeader() string {
	s := p.styles
	back := s.IconBtn.Render(" ← back ")
	title := s.Accent.Bold(true).Render(p.tool.label)
	sub := s.Dim.Render(p.subFor())
	return lipgloss.JoinHorizontal(lipgloss.Top, back, "  ",
		lipgloss.JoinVertical(lipgloss.Left, title, sub),
	)
}

func (p *toolPage) subFor() string {
	switch p.tool.mode {
	case modeImage:
		return "Reencode between PNG · JPEG · WebP · GIF · BMP · TIFF"
	case modePackArchive, modeExtractArchive:
		return "Pack files into one archive · or extract one apart"
	case modePDF:
		return "Merge · split · render pages → image · extract text"
	case modeDoctor:
		return "Check which optional system binaries are on PATH"
	}
	return ""
}

func (p *toolPage) renderArchiveToggle() string {
	s := p.styles
	pack := chip(s, "PACK", p.archive == archivePack)
	extract := chip(s, "EXTRACT", p.archive == archiveExtract)
	hint := s.Dim.Render("  (p) toggle")
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Section.Render("MODE"), "  ", pack, " ", extract, hint)
}

func (p *toolPage) renderPDFOpPicker() string {
	s := p.styles
	ops := []struct {
		op    pdfOp
		glyph string
		label string
	}{
		{pdfMerge, "⊕", "Merge"},
		{pdfSplit, "⊟", "Split"},
		{pdfRender, "◧", "Pages → image"},
		{pdfText, "¶", "Extract text"},
	}
	row := []string{s.Section.Render("OPERATION"), " "}
	for _, o := range ops {
		row = append(row, chip(s, o.glyph+" "+o.label, p.pdfop == o.op), " ")
	}
	row = append(row, s.Dim.Render("(P) cycle"))
	return lipgloss.JoinHorizontal(lipgloss.Top, row...)
}

func (p *toolPage) renderInput() string {
	s := p.styles
	width := p.width - 4
	if width < 30 {
		width = 30
	}
	headRule := strings.Repeat("─", intMax(4, width-32))
	head := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Section.Render("INPUT"),
		"  ",
		lipgloss.NewStyle().Foreground(s.P.Border).Render(headRule),
		"  ",
		s.Dim.Render(p.inputHint()))

	verb := "files or a folder"
	if p.tool.mode == modeExtractArchive || (p.tool.mode == modePackArchive && p.archive == archiveExtract) {
		verb = "an archive"
	} else if p.tool.mode == modePDF {
		verb = "PDFs"
	}
	big := lipgloss.NewStyle().Foreground(s.P.Text).Bold(true).Render("Drop " + verb + " here")

	browse := s.IconBtn.Render(" ▸ Browse files ")
	browseFolder := s.IconBtn.Render(" ▸ Browse folder ")
	helper := s.Dim.Render("— or —  press ") + s.KbdChip.Render("b") + s.Dim.Render(" to browse")

	border := lipgloss.RoundedBorder()
	bf := s.P.Border
	if p.focusKind == focusInput {
		bf = s.P.Accent
	}
	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(bf).
		Padding(1, 2).
		Align(lipgloss.Center).
		Width(width)

	body := lipgloss.JoinVertical(lipgloss.Center,
		big,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, browse, " ", browseFolder),
		helper,
	)
	return head + "\n" + box.Render(body)
}

func (p *toolPage) inputHint() string {
	switch p.tool.mode {
	case modeImage:
		return "accepts PNG · JPEG · WebP · GIF · BMP · TIFF · HEIC"
	case modePackArchive:
		if p.archive == archivePack {
			return "accepts any file or folder"
		}
		return "accepts .zip · .7z · .rar · .tar · .gz · .bz2 · .zst"
	case modeExtractArchive:
		return "accepts .zip · .7z · .rar · .tar · .gz · .bz2 · .zst"
	case modePDF:
		return "accepts .pdf"
	}
	return ""
}

func (p *toolPage) renderFiles() string {
	if len(p.files) == 0 {
		return ""
	}
	s := p.styles
	width := p.width - 4
	if width < 40 {
		width = 40
	}
	headLeft := s.Section.Render("FILES (" + itoa(len(p.files)) + ")")
	headRule := strings.Repeat("─", intMax(4, width-44))
	right := ""
	switch {
	case p.tool.mode == modeImage:
		right = s.Dim.Render("default → ") + s.Accent.Bold(true).Render(p.defaultFmt) +
			s.Dim.Render("   (f) cycle row · (F) apply to all")
	case (p.tool.mode == modePackArchive || p.tool.mode == modeExtractArchive) && p.archive == archivePack:
		right = s.Dim.Render("archive → ") + s.Accent.Bold(true).Render(p.archiveOut)
	}
	head := lipgloss.JoinHorizontal(lipgloss.Top,
		headLeft, "  ",
		lipgloss.NewStyle().Foreground(s.P.Border).Render(headRule), "  ",
		right,
	)

	rows := []string{head}
	for i, f := range p.files {
		rows = append(rows, p.renderFileRow(i, f, width))
	}
	return strings.Join(rows, "\n")
}

func (p *toolPage) renderFileRow(i int, f fileItem, width int) string {
	s := p.styles
	focused := p.focusKind == focusFile && p.focusIdx == i
	diverged := p.tool.mode == modeImage && f.Target != p.defaultFmt

	marker := "  "
	if focused {
		marker = s.Accent.Bold(true).Render("▸ ")
	}
	icon := s.Dim.Render("▪ ")
	name := lipgloss.NewStyle().Foreground(s.P.Text).Render(f.Name)
	right := ""
	if p.tool.mode == modeImage {
		from := s.Dim.Render(f.From)
		arrow := s.Accent.Render(" → ")
		tgtStyle := lipgloss.NewStyle().Foreground(s.P.Text).
			Border(lipgloss.NormalBorder()).BorderForeground(s.P.Border).Padding(0, 1)
		if diverged {
			tgtStyle = tgtStyle.BorderForeground(s.P.Accent).Foreground(s.P.Accent)
		}
		tgt := tgtStyle.Render(f.Target + " ▾")
		right = lipgloss.JoinHorizontal(lipgloss.Top, from, arrow, tgt)
	} else if f.From != "" {
		right = s.Dim.Render(f.From)
	}
	size := s.Dim.Render(f.Size)

	left := lipgloss.JoinHorizontal(lipgloss.Top, marker, icon, name)
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right) + lipgloss.Width(size) + 4
	pad := width - leftWidth - rightWidth
	if pad < 2 {
		pad = 2
	}
	line := left + strings.Repeat(" ", pad) + right + "  " + size
	if focused {
		line = lipgloss.NewStyle().Background(s.P.Surface).Render(line)
	}
	return line
}

func (p *toolPage) renderOutDest() string {
	s := p.styles
	width := p.width - 4
	if width < 30 {
		width = 30
	}
	head := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Section.Render("OUTPUT DESTINATION"),
		"  ",
		lipgloss.NewStyle().Foreground(s.P.Border).Render(strings.Repeat("─", intMax(4, width-22))),
	)
	mk := func(idx int, on bool, label, badge string) string {
		focused := p.focusKind == focusOutDest && p.focusIdx == idx
		radio := "( )"
		if on {
			radio = s.Accent.Render("(●)")
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(4).Render(radio),
			" ",
			label,
		)
		if badge != "" {
			row += "  " + s.Accent.Bold(true).Render(badge)
		}
		if focused {
			row = s.Accent.Bold(true).Render("▸ ") + row
		} else {
			row = "  " + row
		}
		return row
	}
	rows := []string{head,
		mk(0, p.out == outDefault,
			lipgloss.NewStyle().Foreground(s.P.Text).Render("Default location")+
				s.Dim.Render(" — ./out"), "RECOMMENDED"),
		mk(1, p.out == outAlongside,
			lipgloss.NewStyle().Foreground(s.P.Text).Render("Alongside input")+
				s.Dim.Render(" — write next to each source file"), ""),
		mk(2, p.out == outCustom,
			lipgloss.NewStyle().Foreground(s.P.Text).Render("Custom path")+
				s.Dim.Render(" — ")+
				lipgloss.NewStyle().Foreground(s.P.Text).
					Border(lipgloss.NormalBorder()).BorderForeground(s.P.Border).
					Padding(0, 1).Render(p.customPath),
			""),
	}
	return strings.Join(rows, "\n")
}

func (p *toolPage) renderOptions() string {
	s := p.styles
	width := p.width - 4
	if width < 30 {
		width = 30
	}
	head := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Section.Render("OPTIONS"),
		"  ",
		lipgloss.NewStyle().Foreground(s.P.Border).Render(strings.Repeat("─", intMax(4, width-12))),
	)
	rows := []string{head}
	switch p.tool.mode {
	case modeImage:
		rows = append(rows,
			p.optSlider(0, "JPEG/WebP quality", p.quality, 50, 100),
			p.optToggle(1, "Overwrite existing", p.overwrite),
			p.optToggle(2, "Preserve mtime", p.preserveMtime),
			p.optToggle(3, "Recurse subfolders", p.recurse),
		)
	case modePackArchive, modeExtractArchive:
		if p.archive == archivePack {
			rows = append(rows,
				p.optSlider(0, "Compression level", p.compressionLevel, 0, 9),
				p.optToggle(1, "Follow symlinks", p.overwrite),
				p.optToggle(2, "Preserve permissions", p.preserveMtime),
				p.optToggle(3, "Recurse into folders", p.recurse),
			)
		} else {
			rows = append(rows,
				p.optText(0, "On conflict", "ask"),
				p.optToggle(1, "Verify checksums", true),
				p.optToggle(2, "Strip top-level folder", false),
				p.optToggle(3, "Preserve permissions", true),
			)
		}
	case modePDF:
		switch p.pdfop {
		case pdfMerge:
			rows = append(rows,
				p.optText(0, "Output filename", "combined.pdf"),
				p.optText(1, "Preserve metadata from", "first doc"),
			)
		case pdfSplit:
			rows = append(rows,
				p.optText(0, "Pages per split", "1"),
				p.optText(1, "Page ranges", "all"),
			)
		case pdfRender:
			rows = append(rows,
				p.optSlider(0, "DPI", p.dpi, 72, 600),
				p.optText(1, "Image format", "PNG"),
			)
		case pdfText:
			rows = append(rows,
				p.optText(0, "Layout", "reading"),
				p.optText(1, "Pages", "all"),
			)
		}
	}
	return strings.Join(rows, "\n")
}

func (p *toolPage) optSlider(idx int, label string, val, lo, hi int) string {
	s := p.styles
	focused := p.focusKind == focusOptions && p.focusIdx == idx
	pct := 0
	if hi > lo {
		pct = ((val - lo) * 100) / (hi - lo)
	}
	bar := miniBar(s, pct, 16)
	left := s.Dim.Render(label)
	right := bar + "  " + s.Accent.Bold(true).Render(itoa(val))
	if focused {
		left = s.Accent.Bold(true).Render("▸ ") + left
		right += s.Dim.Render("   ← → adjust")
	} else {
		left = "  " + left
	}
	return left + "    " + right
}

func (p *toolPage) optToggle(idx int, label string, on bool) string {
	s := p.styles
	focused := p.focusKind == focusOptions && p.focusIdx == idx
	box := "[ ]"
	if on {
		box = s.Accent.Render("[●]")
	}
	left := s.Dim.Render(label)
	if focused {
		left = s.Accent.Bold(true).Render("▸ ") + left + s.Dim.Render("   space toggles")
	} else {
		left = "  " + left
	}
	return left + "    " + box
}

func (p *toolPage) optText(idx int, label, val string) string {
	s := p.styles
	focused := p.focusKind == focusOptions && p.focusIdx == idx
	left := s.Dim.Render(label)
	right := lipgloss.NewStyle().Foreground(s.P.Text).
		Border(lipgloss.NormalBorder()).BorderForeground(s.P.Border).
		Padding(0, 1).Render(val)
	if focused {
		left = s.Accent.Bold(true).Render("▸ ") + left
	} else {
		left = "  " + left
	}
	return left + "    " + right
}

func (p *toolPage) renderRunRow() string {
	s := p.styles
	summary := s.Dim.Render("ready: ") +
		lipgloss.NewStyle().Foreground(s.P.Text).Bold(true).Render(itoa(len(p.files))+" inputs") +
		s.Dim.Render("  ·  ") + s.Accent.Bold(true).Render(p.summary())
	focused := p.focusKind == focusRun
	btn := lipgloss.NewStyle().
		Padding(0, 3).
		Foreground(s.P.Background).
		Background(s.P.Accent).
		Bold(true).
		Render("▸ RUN")
	if !focused {
		btn = lipgloss.NewStyle().
			Padding(0, 3).
			Foreground(s.P.Accent).
			Border(lipgloss.NormalBorder()).BorderForeground(s.P.Accent).
			Render("▸ RUN")
	}
	hint := s.Dim.Render("  press ") + s.KbdChip.Render("r") + s.Dim.Render(" or ") + s.KbdChip.Render("ENTER")
	return lipgloss.JoinHorizontal(lipgloss.Top, summary, "    ", btn, hint)
}

func (p *toolPage) renderDoctor() string {
	s := p.styles
	deps := []struct {
		name, feature string
		ok            bool
	}{
		{"unrar", "RAR (incl. multi-part)", true},
		{"7z", "7z multi-part", true},
		{"pdftoppm", "PDF → image", true},
		{"pdftotext", "PDF → text", true},
		{"magick", "HEIC images", false},
	}
	found := 0
	for _, d := range deps {
		if d.ok {
			found++
		}
	}
	rows := []string{
		p.renderHeader(),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			s.Section.Render("SYSTEM DEPENDENCIES"),
			"  ",
			s.Dim.Render(itoa(found)+" / "+itoa(len(deps))+" found"),
		),
	}
	for _, d := range deps {
		icon := s.OK.Render("●")
		state := s.OK.Render("FOUND")
		if !d.ok {
			icon = s.Warn.Render("○")
			state = s.Warn.Render("MISSING")
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
			"  ", icon, "  ",
			lipgloss.NewStyle().Width(12).Foreground(s.P.Text).Bold(true).Render(d.name),
			s.Dim.Render(d.feature),
			"   ", state,
		))
	}
	rows = append(rows, "",
		s.Section.Render("INSTALL HINTS"),
		s.Dim.Render("  macOS:  ")+lipgloss.NewStyle().Foreground(s.P.Text).Render("brew install imagemagick"),
		s.Dim.Render("  Debian: ")+lipgloss.NewStyle().Foreground(s.P.Text).Render("apt install imagemagick"),
		s.Dim.Render("  Arch:   ")+lipgloss.NewStyle().Foreground(s.P.Text).Render("pacman -S imagemagick"),
	)
	return strings.Join(rows, "\n")
}

// chip renders an on/off pill for archive-mode / PDF-op togglers.
func chip(s theme.Styles, label string, on bool) string {
	st := lipgloss.NewStyle().Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(s.P.Border).
		Foreground(s.P.TextDim)
	if on {
		st = st.BorderForeground(s.P.Accent).Foreground(s.P.Accent).Bold(true)
	}
	return st.Render(label)
}

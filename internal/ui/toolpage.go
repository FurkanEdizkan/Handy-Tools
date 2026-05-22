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
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/furkandedizkan/handy-tools/internal/tools/sysdep"
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
	Path   string // absolute path — set for files the user typed in; empty for demo rows
	From   string // source format display ("PNG", "JPEG", "7z (multi-part)" …)
	Target string // current target format ("JPEG", "WebP", …) — image mode only
	Size   string
}

// captureKind tracks which text field, if any, is currently swallowing
// keystrokes. While capture != captureNone every rune key edits captureBuf
// instead of triggering a tool-page shortcut, and the router forwards keys
// straight here (see Model.Update).
type captureKind int

const (
	captureNone   captureKind = iota
	capturePath               // typing a source-file path into the dropzone
	captureOutput             // editing the custom output-destination path
)

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

	// text capture (real file input / editable custom path)
	capture    captureKind
	captureBuf string

	// options
	quality          int
	overwrite        bool
	preserveMtime    bool
	recurse          bool
	compressionLevel int
	dpi              int
	autoMultiPart    bool // archive-extract: accept multi-part without a prompt
	pdfEveryN        int  // pdf-split: pages per output file
	pdfJPEG          bool // pdf-render: JPEG output (false = PNG)
	pdfLayout        bool // pdf-text: preserve physical layout
}

// newToolPage builds a populated page for the given tool id.
func newToolPage(s theme.Styles, t tool) *toolPage {
	p := &toolPage{
		styles:           s,
		tool:             t,
		defaultFmt:       "JPEG",
		archiveOut:       "zip",
		out:              outDefault,
		customPath:       "./converted",
		quality:          90,
		preserveMtime:    true,
		recurse:          true,
		compressionLevel: 6,
		dpi:              150,
		pdfEveryN:        1,
		focusKind:        focusInput,
	}
	if t.mode == modeExtractArchive {
		p.archive = archiveExtract
	}
	// The page opens with no files — the user adds real input via `b`
	// (path entry) or the dropzone. renderFiles() handles an empty list.
	return p
}

func (p *toolPage) SetWidth(w int) { p.width = w }
func (p *toolPage) Tool() tool     { return p.tool }

// Update routes keys for the tool page. Returns a RunJob message when the
// user triggers the Run button.
func (p *toolPage) Update(msg tea.Msg) (*toolPage, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	// The Doctor page is read-only: it has no focusable rows, no Run job.
	// Only Back (esc) and a PATH re-scan (r) apply.
	if p.tool.mode == modeDoctor {
		switch k.String() {
		case "esc":
			return p, func() tea.Msg { return GoHome{} }
		case "r":
			sysdep.Reset() // bust the cache so the next render re-probes PATH
		}
		return p, nil
	}
	// While a text field is open, every key edits it — shortcuts are off.
	if p.capture != captureNone {
		p.updateCapture(k)
		return p, nil
	}
	switch k.String() {
	case "esc":
		return p, func() tea.Msg { return GoHome{} }
	case "b":
		// Open the path-entry field so the user can add a real file.
		p.focusKind = focusInput
		p.capture, p.captureBuf = capturePath, ""
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
		// Enter on the Custom output row selects it and opens its path
		// field for editing in one step.
		if p.focusKind == focusOutDest && p.focusIdx == 2 {
			p.capture, p.captureBuf = captureOutput, p.customPath
		}
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
	job := RunJob{
		Tool:        p.tool,
		Files:       append([]fileItem(nil), p.files...),
		Summary:     p.summary(),
		ArchiveMode: p.archive,
		ArchiveOut:  p.archiveOut,
		PDFOp:       p.pdfop,
		Out:         p.out,
		CustomPath:  p.customPath,

		Quality:          p.quality,
		CompressionLevel: p.compressionLevel,
		DPI:              p.dpi,
		EveryN:           p.pdfEveryN,
		Overwrite:        p.overwrite,
		AutoMultiPart:    p.autoMultiPart,
		PDFJpeg:          p.pdfJPEG,
		PDFLayout:        p.pdfLayout,
	}
	return func() tea.Msg { return job }
}

// capturingText reports whether a text field is open. The router checks this
// to forward every key straight to the page (so a typed path isn't read as a
// shortcut), mirroring how the settings popover swallows keys.
func (p *toolPage) capturingText() bool { return p.capture != captureNone }

// updateCapture edits the open text buffer. Enter commits, Esc cancels;
// otherwise printable runes append and Backspace / Ctrl+U trim.
func (p *toolPage) updateCapture(k tea.KeyMsg) {
	switch k.Type {
	case tea.KeyEsc:
		p.capture, p.captureBuf = captureNone, ""
	case tea.KeyEnter:
		p.commitCapture()
	case tea.KeyBackspace:
		if r := []rune(p.captureBuf); len(r) > 0 {
			p.captureBuf = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		p.captureBuf = ""
	case tea.KeySpace:
		p.captureBuf += " "
	case tea.KeyRunes:
		p.captureBuf += string(k.Runes)
	}
}

// commitCapture applies the typed buffer: capturePath appends a real file row,
// captureOutput updates the custom output path. An empty buffer is a no-op.
func (p *toolPage) commitCapture() {
	switch p.capture {
	case capturePath:
		if raw := strings.TrimSpace(p.captureBuf); raw != "" {
			abs := raw
			if a, err := filepath.Abs(raw); err == nil {
				abs = a
			}
			name := filepath.Base(abs)
			item := fileItem{
				ID:   fmt.Sprintf("u%d", len(p.files)+1),
				Name: name,
				Path: abs,
				From: strings.ToUpper(strings.TrimPrefix(filepath.Ext(name), ".")),
			}
			if p.tool.mode == modeImage {
				item.Target = p.defaultFmt
			}
			p.files = append(p.files, item)
		}
	case captureOutput:
		if v := strings.TrimSpace(p.captureBuf); v != "" {
			p.customPath = v
		}
	}
	p.capture, p.captureBuf = captureNone, ""
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
			} else if p.optionCount() > 0 {
				p.focusKind, p.focusIdx = focusOptions, 0
			} else {
				p.focusKind = focusRun // modes with no options (PDF merge)
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
			if n := p.optionCount(); n > 0 {
				p.focusKind, p.focusIdx = focusOptions, n-1
			} else {
				p.focusKind, p.focusIdx = focusOutDest, 2
			}
		}
	}
}

// optionCount is the number of option rows for the current mode.
func (p *toolPage) optionCount() int {
	switch p.tool.mode {
	case modeImage:
		return 4
	case modePackArchive, modeExtractArchive:
		if p.archive == archivePack {
			return 4
		}
		return 2 // extract: overwrite + auto-multi-part
	case modePDF:
		switch p.pdfop {
		case pdfMerge:
			return 0 // output is <dest>/merged.pdf — no options
		case pdfSplit, pdfText:
			return 1
		case pdfRender:
			return 2
		}
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
	// Pack and extract take different input (loose files vs. an archive),
	// so a mode switch drops whatever was queued.
	p.files = nil
	p.resetFocusToInput() // option-row count differs between pack and extract
}

func (p *toolPage) cyclePDFOp() {
	if p.tool.mode != modePDF {
		return
	}
	p.pdfop = pdfOp((int(p.pdfop) + 1) % 4)
	p.resetFocusToInput() // option-row count differs between PDF operations
}

// resetFocusToInput parks focus on the dropzone. Called when an action
// (pack/extract toggle, PDF-op cycle) changes how many option rows exist, so
// a stale focusIdx can't point past the new option count.
func (p *toolPage) resetFocusToInput() {
	p.focusKind, p.focusIdx = focusInput, 0
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
		if p.archive == archivePack {
			// slider on row 0, three toggles on rows 1..3
			switch p.focusIdx {
			case 1:
				p.overwrite = !p.overwrite
			case 2:
				p.preserveMtime = !p.preserveMtime
			case 3:
				p.recurse = !p.recurse
			}
		} else {
			switch p.focusIdx {
			case 0:
				p.overwrite = !p.overwrite
			case 1:
				p.autoMultiPart = !p.autoMultiPart
			}
		}
	case modePDF:
		switch p.pdfop {
		case pdfRender:
			if p.focusIdx == 1 {
				p.pdfJPEG = !p.pdfJPEG
			}
		case pdfText:
			if p.focusIdx == 0 {
				p.pdfLayout = !p.pdfLayout
			}
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
		switch {
		case p.pdfop == pdfRender && p.focusIdx == 0:
			p.dpi = clamp(p.dpi+dir*12, 72, 600)
		case p.pdfop == pdfSplit && p.focusIdx == 0:
			p.pdfEveryN = clamp(p.pdfEveryN+dir, 1, 50)
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
	// While the path field is open the dropzone becomes a text input.
	if p.capture == capturePath {
		field := lipgloss.NewStyle().
			Foreground(s.P.Text).
			Border(lipgloss.NormalBorder()).
			BorderForeground(s.P.Accent).
			Padding(0, 1).
			Width(intMax(20, width-8)).
			Render(p.captureBuf + "█")
		body = lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(s.P.Text).Bold(true).Render("Type a file or folder path"),
			"",
			field,
			s.Dim.Render("enter to add · esc to cancel"),
		)
	}
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

// customPathField renders the custom output path's bordered box — showing the
// live edit buffer with a cursor while captureOutput is active.
func (p *toolPage) customPathField() string {
	s := p.styles
	val, bf := p.customPath, s.P.Border
	if p.capture == captureOutput {
		val, bf = p.captureBuf+"█", s.P.Accent
	}
	return lipgloss.NewStyle().Foreground(s.P.Text).
		Border(lipgloss.NormalBorder()).BorderForeground(bf).
		Padding(0, 1).Render(val)
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
				p.customPathField(),
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
			p.optSlider("JPEG/WebP quality", p.quality, 50, 100),
			p.optToggle(1, "Overwrite existing", p.overwrite),
			p.optToggle(2, "Preserve mtime", p.preserveMtime),
			p.optToggle(3, "Recurse subfolders", p.recurse),
		)
	case modePackArchive, modeExtractArchive:
		if p.archive == archivePack {
			rows = append(rows,
				p.optSlider("Compression level", p.compressionLevel, 0, 9),
				p.optToggle(1, "Follow symlinks", p.overwrite),
				p.optToggle(2, "Preserve permissions", p.preserveMtime),
				p.optToggle(3, "Recurse into folders", p.recurse),
			)
		} else {
			rows = append(rows,
				p.optToggle(0, "Overwrite existing files", p.overwrite),
				p.optToggle(1, "Auto-accept multi-part archives", p.autoMultiPart),
			)
		}
	case modePDF:
		switch p.pdfop {
		case pdfMerge:
			rows = append(rows, "  "+s.Dim.Render("no options — merges into merged.pdf"))
		case pdfSplit:
			rows = append(rows, p.optSlider("Pages per split file", p.pdfEveryN, 1, 50))
		case pdfRender:
			rows = append(rows,
				p.optSlider("DPI", p.dpi, 72, 600),
				p.optToggle(1, "JPEG output (off = PNG)", p.pdfJPEG),
			)
		case pdfText:
			rows = append(rows, p.optToggle(0, "Preserve physical layout", p.pdfLayout))
		}
	}
	return strings.Join(rows, "\n")
}

// optSlider renders the focus-row-0 numeric option (every mode puts its one
// slider — quality / compression / DPI / pages-per-split — on row 0).
func (p *toolPage) optSlider(label string, val, lo, hi int) string {
	s := p.styles
	focused := p.focusKind == focusOptions && p.focusIdx == 0
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

// renderDoctor shows live system-dependency state. It reads sysdep.All() on
// every render, so a press of `r` (which calls sysdep.Reset()) re-probes PATH
// without restarting the TUI. The same sysdep.All() data drives the
// `htools doctor` subcommand, so both surfaces always agree.
func (p *toolPage) renderDoctor() string {
	s := p.styles
	results := sysdep.All()
	found := 0
	for _, r := range results {
		if r.Found {
			found++
		}
	}
	rows := []string{
		p.renderHeader(),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			s.Section.Render("SYSTEM DEPENDENCIES"),
			"  ",
			s.Dim.Render(itoa(found)+" / "+itoa(len(results))+" found"),
		),
		"",
	}
	name := lipgloss.NewStyle().Width(12).Foreground(s.P.Text).Bold(true)
	for _, r := range results {
		// Main row stays short (glyph · name · state) so it fits the
		// right column at minimum width; detail lines wrap below.
		icon := s.OK.Render("✓")
		state := s.OK.Render("FOUND")
		if !r.Found {
			icon = s.Warn.Render("✗")
			state = s.Warn.Render("MISSING")
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
			"  ", icon, "  ", name.Render(r.Tool.Name), "   ", state,
		))
		rows = append(rows, s.Dim.Render("       "+r.Tool.Description))
		for _, feat := range r.Tool.Features {
			rows = append(rows, s.Dim.Render("       · "+feat))
		}
		if !r.Found {
			if hint := r.Tool.InstallHint[runtime.GOOS]; hint != "" {
				rows = append(rows, s.Dim.Render("       install: ")+
					lipgloss.NewStyle().Foreground(s.P.Text).Render(hint))
			}
		}
	}
	rows = append(rows, "",
		s.Dim.Render("  press ")+s.KbdChip.Render("r")+s.Dim.Render(" to re-scan PATH"),
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

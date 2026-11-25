package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/lancekrogers/peep/internal/browser"
	"github.com/lancekrogers/peep/internal/document"
	"github.com/lancekrogers/peep/internal/render"
)

type fakePDF struct {
	mu                sync.Mutex
	pages, opens      int
	rows              map[int]int
	rowsPerColumn     bool
	rendered          []int
	cellW, cellH      int
	openErr           error
	changed           bool
	pagesAfterChange  int
	lastTextRequested int
}

func (f *fakePDF) Open(ctx context.Context, path string) (document.PDFInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opens++
	return document.PDFInfo{Path: path, Pages: f.pages}, f.openErr
}

func (f *fakePDF) stale() bool {
	if !f.changed {
		return false
	}
	f.changed = false
	f.pages = f.pagesAfterChange
	return true
}

func (f *fakePDF) Page(ctx context.Context, info document.PDFInfo, page, columns, cellWidth, cellHeight int) (document.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stale() {
		return document.Image{}, document.ErrPDFChanged
	}
	f.rendered = append(f.rendered, page)
	f.cellW, f.cellH = cellWidth, cellHeight
	rows := 40
	if r, ok := f.rows[page]; ok {
		rows = r
	} else if f.rowsPerColumn {
		rows = columns
	}
	cells := make([]string, rows)
	for i := range cells {
		cells[i] = fmt.Sprintf("p%d r%d", page, i)
	}
	return document.Image{PNG: []byte{byte(page)}, Width: columns * 10, Height: rows * 20, Columns: columns, Rows: rows, Cells: cells}, nil
}

func (f *fakePDF) Text(ctx context.Context, info document.PDFInfo, page int) (*document.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stale() {
		return nil, document.ErrPDFChanged
	}
	f.lastTextRequested = page
	return &document.Document{Path: info.Path, Source: fmt.Sprintf("page %d text\nneedle on page %d", page, page), Kind: "TEXT"}, nil
}

func (f *fakePDF) renders() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.rendered...)
}

// drain runs commands synchronously and feeds their messages back.
func drain(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			drain(m, c)
		}
	default:
		_, next := m.Update(msg)
		drain(m, next)
	}
}

func update(m *Model, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

func press(m *Model, keys ...tea.KeyType) {
	for _, k := range keys {
		drain(m, m.key(tea.KeyMsg{Type: k}))
	}
}

func typeRunes(m *Model, s string) {
	for _, r := range s {
		drain(m, m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}))
	}
}

func pdfModel(t *testing.T, pages int, native bool) (*Model, *fakePDF) {
	t.Helper()
	m := imageModel()
	m.dir, m.history = "/docs", map[string]string{}
	m.entries = []browser.Entry{{Path: "/docs/manual.pdf", Name: "manual.pdf"}, {Path: "/docs/notes.txt", Name: "notes.txt"}}
	f := &fakePDF{pages: pages}
	m.docPDF = f
	if native {
		m.graphics = &render.KittyWriter{}
		m.cellPixels = func() (int, int) { return 10, 20 }
	}
	drain(m, m.previewSelection())
	if m.pdf == nil || m.pdf.page != 1 {
		t.Fatalf("PDF did not open: %q", m.previewError)
	}
	press(m, tea.KeyEnter)
	if !m.focus {
		t.Fatal("Enter did not focus the PDF")
	}
	return m, f
}

func shownPage(m *Model) int {
	if e := m.docImages[0]; e != nil && len(e.image.PNG) == 1 {
		return int(e.image.PNG[0])
	}
	return 0
}

func bottom(m *Model) int {
	lines, _ := m.documentComposition()
	return len(lines) - m.layout().body
}

func TestPDFPagesAndJumps(t *testing.T) {
	m, f := pdfModel(t, 5, true)
	if m.documentTitle() != "PDF · page 1/5" || shownPage(m) != 1 || f.cellW != 10 || f.cellH != 20 {
		t.Fatalf("%q page %d cell %dx%d", m.documentTitle(), shownPage(m), f.cellW, f.cellH)
	}
	if _, slots := m.documentComposition(); len(slots) != 1 || slots[0].rows != 40 || slots[0].columns != m.documentWidth() {
		t.Fatalf("page not sized by its own grid: %+v", slots)
	}
	press(m, tea.KeyRight)
	if m.pdf.page != 2 || shownPage(m) != 2 || m.docOffset != 0 {
		t.Fatal("right did not show page 2 at its top")
	}
	press(m, tea.KeyLeft, tea.KeyLeft)
	if m.pdf.page != 1 || shownPage(m) != 1 {
		t.Fatal("left did not stop at page 1")
	}
	for keys, want := range map[string]int{"3G": 3, "9G": 5, "gg": 1, "2gg": 2} {
		m.docOffset = 5
		typeRunes(m, keys)
		if m.pdf.page != want || m.docOffset != 0 || shownPage(m) != want {
			t.Fatalf("%s: page %d offset %d", keys, m.pdf.page, m.docOffset)
		}
	}
	typeRunes(m, "G")
	if m.pdf.page != 5 || m.docOffset != bottom(m) {
		t.Fatal("G did not land at the bottom of the last page")
	}
	before := len(f.renders())
	press(m, tea.KeyRight)
	typeRunes(m, "l")
	if m.pdf.page != 5 || len(f.renders()) != before {
		t.Fatal("next page wrapped or re-rendered at the end")
	}
	press(m, tea.KeyHome)
	if m.pdf.page != 1 || m.docOffset != 0 {
		t.Fatal("home did not open page 1")
	}
	press(m, tea.KeyEnd)
	if m.pdf.page != 5 || m.docOffset != bottom(m) {
		t.Fatal("end did not open the last page at its bottom")
	}
}

func TestPDFScrollCrossesPageEdges(t *testing.T) {
	m, f := pdfModel(t, 3, true)
	f.rows = map[int]int{1: 70}
	for i := 0; i < 100 && m.docOffset < bottom(m); i++ {
		press(m, tea.KeyDown)
	}
	if m.pdf.page != 1 || m.docOffset != bottom(m) {
		t.Fatal("j did not scroll within the page")
	}
	press(m, tea.KeyDown)
	if m.pdf.page != 2 || m.docOffset != 0 {
		t.Fatalf("j at the bottom: page %d offset %d", m.pdf.page, m.docOffset)
	}
	press(m, tea.KeyUp)
	if m.pdf.page != 1 || m.docOffset != bottom(m) || bottom(m) != 70-m.layout().body {
		t.Fatalf("k at the top: page %d offset %d", m.pdf.page, m.docOffset)
	}
	press(m, tea.KeyPgDown)
	if m.pdf.page != 2 || m.docOffset != 0 {
		t.Fatal("pgdown at the bottom did not turn the page")
	}
	press(m, tea.KeyPgUp)
	if m.pdf.page != 1 || m.docOffset != bottom(m) {
		t.Fatal("pgup at the top did not go back a page")
	}
	press(m, tea.KeyCtrlD)
	if m.pdf.page != 2 {
		t.Fatal("ctrl+d at the bottom did not turn the page")
	}

	press(m, tea.KeyEsc)
	if m.focus || m.pdf == nil || m.pdf.page != 2 {
		t.Fatal("leaving focus lost the page")
	}
	m.docOffset = 0
	press(m, tea.KeyPgUp)
	if m.pdf.page != 1 || m.docOffset != bottom(m) {
		t.Fatal("unfocused pgup did not turn the page")
	}
	cursor := m.cursor
	press(m, tea.KeyRight)
	if m.pdf.page != 1 || m.cursor != cursor {
		t.Fatal("unfocused right turned a page")
	}
	m.key(tea.KeyMsg{Type: tea.KeyLeft})
	if m.dir != "/" {
		t.Fatal("unfocused left lost its parent-folder meaning")
	}
}

func TestPDFWheelCrossesPageEdges(t *testing.T) {
	for _, focused := range []bool{true, false} {
		m, f := pdfModel(t, 3, true)
		f.rows = map[int]int{1: 70}
		if !focused {
			press(m, tea.KeyEsc)
		}
		l := m.layout()
		over := tea.MouseMsg{X: l.files + 4, Y: 5}
		wheel := func(button tea.MouseButton) {
			over.Button = button
			drain(m, m.mouse(over))
		}
		for i := 0; i < 100 && m.docOffset < bottom(m); i++ {
			wheel(tea.MouseButtonWheelDown)
		}
		if m.pdf.page != 1 || m.docOffset != bottom(m) {
			t.Fatalf("focused=%v: wheel did not scroll within the page", focused)
		}
		wheel(tea.MouseButtonWheelDown)
		if m.pdf.page != 2 || m.docOffset != 0 {
			t.Fatalf("focused=%v: wheel at the bottom: page %d offset %d", focused, m.pdf.page, m.docOffset)
		}
		wheel(tea.MouseButtonWheelUp)
		if m.pdf.page != 1 || m.docOffset != bottom(m) {
			t.Fatalf("focused=%v: wheel at the top: page %d offset %d", focused, m.pdf.page, m.docOffset)
		}
		wheel(tea.MouseButtonWheelUp)
		if m.docOffset != bottom(m)-3 {
			t.Fatalf("focused=%v: wheel up moved to %d", focused, m.docOffset)
		}
		m.docOffset = 0
		wheel(tea.MouseButtonWheelUp)
		if m.pdf.page != 1 || m.docOffset != 0 {
			t.Fatalf("focused=%v: wheel wrapped before the first page", focused)
		}
		over.X, m.docOffset = 0, bottom(m)
		wheel(tea.MouseButtonWheelDown)
		if m.pdf.page != 1 {
			t.Fatalf("focused=%v: wheel outside the preview turned a page", focused)
		}
	}
}

func TestPDFIgnoresLateAndInFlightResults(t *testing.T) {
	m, f := pdfModel(t, 4, true)
	stale := m.key(tea.KeyMsg{Type: tea.KeyRight})
	if !m.pdf.loading || m.pdf.page != 2 {
		t.Fatal("page turn did not start loading")
	}
	if cmd := m.key(tea.KeyMsg{Type: tea.KeyUp}); cmd != nil || m.pdf.page != 2 {
		t.Fatal("page edge crossed while the target page was loading")
	}
	press(m, tea.KeyRight)
	if m.pdf.page != 3 || shownPage(m) != 3 {
		t.Fatal("second turn did not show page 3")
	}
	drain(m, stale)
	if m.pdf.page != 3 || shownPage(m) != 3 {
		t.Fatal("late page 2 result replaced page 3")
	}

	open := m.readDocument("/docs/manual.pdf")
	m.focus = false
	press(m, tea.KeyDown)
	if m.pdf != nil {
		t.Fatal("moving to another file kept the PDF")
	}
	drain(m, open)
	if m.pdf != nil {
		t.Fatal("late open for the previous selection was applied")
	}
	if f.opens != 2 {
		t.Fatalf("opens %d", f.opens)
	}
}

func TestPDFPrefetchFollowsDirectionWithoutChangingDisplay(t *testing.T) {
	m, f := pdfModel(t, 4, true)
	if r := f.renders(); r[len(r)-1] != 2 || shownPage(m) != 1 || m.pdf.page != 1 {
		t.Fatalf("renders %v, showing %d", r, shownPage(m))
	}
	typeRunes(m, "3G")
	press(m, tea.KeyLeft)
	if r := f.renders(); r[len(r)-1] != 1 || shownPage(m) != 2 || m.pdf.page != 2 {
		t.Fatalf("backward prefetch: renders %v, showing %d", r, shownPage(m))
	}
	typeRunes(m, "4G")
	if r := f.renders(); r[len(r)-1] != 4 {
		t.Fatalf("prefetched past the last page: %v", r)
	}
	m.clearDocument()
	if m.pdfPrefetchCancel != nil {
		t.Fatal("clearing the document kept prefetch running")
	}
}

func TestPDFFileChangedReopens(t *testing.T) {
	m, f := pdfModel(t, 5, true)
	typeRunes(m, "4G")
	f.changed, f.pagesAfterChange = true, 3
	drain(m, update(m, tea.WindowSizeMsg{Width: 120, Height: 40}))
	if f.opens != 2 || m.pdf.page != 3 || m.pdf.info.Pages != 3 || shownPage(m) != 3 || m.notice != pdfReloadedNotice {
		t.Fatalf("opens %d page %d/%d showing %d notice %q", f.opens, m.pdf.page, m.pdf.info.Pages, shownPage(m), m.notice)
	}
	press(m, tea.KeyLeft)
	if m.notice != "" {
		t.Fatal("reload notice outlived a page turn")
	}
	f.openErr = errors.New("PDF is password-protected; preview unavailable")
	f.changed, f.pagesAfterChange = true, 3
	press(m, tea.KeyLeft)
	if m.doc != nil || m.pdf != nil || m.previewError != f.openErr.Error() {
		t.Fatal("failed reopen did not show its error")
	}
}

func TestPDFTextModeTogglingAndSearch(t *testing.T) {
	m, f := pdfModel(t, 3, false)
	if m.documentTitle() != "PDF text · page 1/3" || m.doc.Kind != "TEXT" || !strings.Contains(ansi.Strip(m.docLayout.Lines[0].Text), "page 1 text") {
		t.Fatalf("cells default is not text: %q", m.documentTitle())
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "PDF text · page 1/3 · manual.pdf") || !strings.Contains(view, "←/→ page") {
		t.Fatal("view lacks the PDF title or page keys")
	}
	typeRunes(m, "/needle")
	press(m, tea.KeyEnter)
	if m.documentSearchStatus() != "Match 1/1" {
		t.Fatal(m.documentSearchStatus())
	}
	press(m, tea.KeyRight)
	if f.lastTextRequested != 2 || !strings.Contains(ansi.Strip(m.docLayout.Lines[0].Text), "page 2 text") || m.documentTitle() != "PDF text · page 2/3" {
		t.Fatal("text mode did not turn the page")
	}
	typeRunes(m, "R")
	if m.documentTitle() != "PDF · page 2/3" || shownPage(m) != 2 || f.cellW != 1 || f.cellH != 2 {
		t.Fatalf("R did not show the page picture in half-block cells: %q %dx%d", m.documentTitle(), f.cellW, f.cellH)
	}
	if lines, _ := m.documentComposition(); lines[0].Text != "p2 r0" {
		t.Fatalf("cells not drawn from the page grid: %q", lines[0].Text)
	}
	typeRunes(m, "/")
	if m.docSearch.editing || m.notice != pdfSearchNotice {
		t.Fatal("picture mode opened a search that cannot match")
	}
	press(m, tea.KeyLeft)
	if m.notice != "" {
		t.Fatal("search notice outlived a page turn")
	}

	m.graphics = &render.KittyWriter{}
	m.cellPixels = func() (int, int) { return 9, 18 }
	drain(m, update(m, tea.WindowSizeMsg{Width: 100, Height: 30}))
	if m.documentTitle() != "PDF text · page 1/3" || !m.pdf.toggled {
		t.Fatalf("native graphics did not flip the toggled view back to text: %q", m.documentTitle())
	}
	typeRunes(m, "R")
	if m.documentTitle() != "PDF · page 1/3" || f.cellW != 9 || shownPage(m) != 1 {
		t.Fatal("native default is not the page picture")
	}
}

func TestPDFKeepsTransportAndTrackKeys(t *testing.T) {
	m, _ := pdfModel(t, 3, true)
	m.queue, m.index = []string{"a.png", "b.png"}, 0
	position := m.position
	typeRunes(m, "]")
	if m.position == position {
		t.Fatal("seek did not reach playback while reading a PDF")
	}
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.index != 1 || m.pdf == nil || m.pdf.page != 1 {
		t.Fatal("n did not reach the queue while reading a PDF")
	}
}

func TestPDFOpenErrorAndKind(t *testing.T) {
	m := imageModel()
	m.entries = []browser.Entry{{Path: "/docs/locked.pdf", Name: "locked.pdf"}}
	m.docPDF = &fakePDF{openErr: errors.New("PDF is password-protected; preview unavailable")}
	drain(m, m.previewSelection())
	if m.doc != nil || m.previewError != "PDF is password-protected; preview unavailable" {
		t.Fatal(m.previewError)
	}
	if !strings.Contains(ansi.Strip(strings.Join(m.fileRows(60, 4, false), "\n")), "PDF") {
		t.Fatal("PDF rows lack a kind badge")
	}
}

func TestPDFNativePlacementAndReflow(t *testing.T) {
	m, f := pdfModel(t, 3, true)
	m.docOffset = 10
	_, slots := m.documentComposition()
	images := m.documentGraphics(40, 3)
	if len(images) != 1 || images[0].Rect.Columns != slots[0].columns || images[0].Rect.Rows != m.layout().body || images[0].Y != 10*m.docImages[0].image.Height/slots[0].rows {
		t.Fatalf("page placement %+v for slot %+v", images, slots)
	}
	f.rowsPerColumn = true
	press(m, tea.KeyRight, tea.KeyLeft)
	rows := m.docImages[0].image.Rows
	m.docOffset = 40
	press(m, tea.KeyEsc)
	if narrower := m.docImages[0].image.Rows; narrower >= rows || m.docOffset != 40*narrower/rows {
		t.Fatalf("reflow from %d to %d rows kept offset %d", rows, narrower, m.docOffset)
	}
}

func TestPDFNoticesAndDefaultFlip(t *testing.T) {
	m, _ := pdfModel(t, 3, false)
	drain(m, update(m, tea.WindowSizeMsg{Width: 100, Height: 30}))
	m.graphics = &render.KittyWriter{}
	m.cellPixels = func() (int, int) { return 10, 20 }
	drain(m, update(m, tea.WindowSizeMsg{Width: 100, Height: 30}))
	if m.documentTitle() != "PDF · page 1/3" || m.pdf.toggled {
		t.Fatalf("native graphics did not make the picture the default: %q", m.documentTitle())
	}
	typeRunes(m, "/")
	typeRunes(m, "R")
	if m.notice != "" || m.documentTitle() != "PDF text · page 1/3" {
		t.Fatalf("search notice outlived R: %q", m.notice)
	}
	typeRunes(m, "R/")
	m.focus = false
	press(m, tea.KeyDown)
	if m.notice != "" {
		t.Fatal("search notice outlived the PDF")
	}
}

func TestPDFSearchKeepsNAndTextReloads(t *testing.T) {
	m, f := pdfModel(t, 3, false)
	m.queue, m.index = []string{"a.png", "b.png"}, 0
	typeRunes(m, "/needle")
	press(m, tea.KeyEnter)
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.index != 0 {
		t.Fatal("n left the search for the queue")
	}
	f.changed, f.pagesAfterChange = true, 2
	press(m, tea.KeyRight)
	if f.opens != 2 || m.notice != pdfReloadedNotice || m.pdf.page != 2 || f.lastTextRequested != 2 {
		t.Fatalf("text reload: opens %d page %d notice %q", f.opens, m.pdf.page, m.notice)
	}
}

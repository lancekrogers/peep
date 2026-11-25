package tui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/document"
	"golang.org/x/sys/unix"
)

type pdfRenderer interface {
	Open(ctx context.Context, path string) (document.PDFInfo, error)
	Page(ctx context.Context, info document.PDFInfo, page, columns, cellWidth, cellHeight int) (document.Image, error)
	Text(ctx context.Context, info document.PDFInfo, page int) (*document.Document, error)
}

type pdfLanding int

const (
	pdfStay pdfLanding = iota
	pdfTop
	pdfBottom
)

const (
	pdfSearchNotice   = "This page is a picture · R shows its text to search"
	pdfReloadedNotice = "PDF changed on disk · reloaded"
)

// pdfView is the open PDF behind m.doc. Whether a page shows as a picture is
// derived from the current renderer; toggled only records the user's R.
type pdfView struct {
	path             string
	info             document.PDFInfo
	page, direction  int
	toggled, loading bool
	land             pdfLanding
}

type pdfOpenedMsg struct {
	id     int
	path   string
	info   document.PDFInfo
	err    error
	reload bool
}
type pdfTextMsg struct {
	id, anchor int
	doc        *document.Document
	layout     document.Layout
	err        error
}
type pdfPrefetchMsg struct{}

func (m *Model) pdfRenderer() pdfRenderer {
	if m.docPDF == nil {
		m.docPDF = document.NewPDF()
	}
	return m.docPDF
}

func (m *Model) pdfPicture() bool { return m.pdf != nil && (m.graphics != nil) != m.pdf.toggled }

// pdfCellSize lets poppler rasterize at the terminal's real cell size, or
// straight to the half-block sample grid when cells draw the page.
func (m *Model) pdfCellSize() (int, int) {
	if m.graphics == nil {
		return 1, 2
	}
	if m.cellPixels != nil {
		return m.cellPixels()
	}
	ws, err := unix.IoctlGetWinsize(int(m.graphics.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return 0, 0
	}
	return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row)
}

func (m *Model) pdfColumns() int { return min(1024, m.documentWidth()) }

func (m *Model) openPDF(path string, reload bool) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	m.previewCancel = cancel
	id, r := m.previewID, m.pdfRenderer()
	return func() tea.Msg {
		defer cancel()
		info, err := r.Open(ctx, path)
		return pdfOpenedMsg{id, path, info, err, reload}
	}
}

func (m *Model) reloadPDF() tea.Cmd {
	m.cancelDocumentImages()
	m.cancelPDFPrefetch()
	if m.previewCancel != nil {
		m.previewCancel()
	}
	m.previewID++
	return m.openPDF(m.pdf.path, true)
}

func (m *Model) applyPDFOpened(v pdfOpenedMsg) tea.Cmd {
	if v.id != m.previewID {
		return nil
	}
	if v.err != nil {
		m.clearDocument()
		m.previewError = v.err.Error()
		return nil
	}
	if v.reload && m.pdf != nil && m.pdf.path == v.path {
		m.pdf.info = v.info
		m.pdf.page = min(m.pdf.page, v.info.Pages)
		m.notice = pdfReloadedNotice
		return m.showPDFPage(pdfStay)
	}
	m.clearDocument()
	m.pdf = &pdfView{path: v.path, info: v.info, page: 1, direction: 1}
	return m.showPDFPage(pdfTop)
}

// showPDFPage starts showing the current page in the current mode. The new
// generation drops late results for any earlier page, size or mode.
func (m *Model) showPDFPage(land pdfLanding) tea.Cmd {
	p := m.pdf
	anchor := m.documentAnchor()
	m.cancelDocumentImages()
	m.cancelPDFPrefetch()
	if m.previewCancel != nil {
		m.previewCancel()
	}
	m.previewID++
	p.land, p.loading = land, true
	if m.pdfPicture() {
		placeholder := &documentImage{}
		if previous := m.docImages[0]; m.doc != nil && m.doc.Kind == "PDF" && previous != nil {
			placeholder.image = document.Image{Width: previous.image.Width, Height: previous.image.Height, Columns: previous.image.Columns, Rows: previous.image.Rows}
		}
		m.doc = &document.Document{Path: p.path, Kind: "PDF"}
		m.docLayout = document.Layout{Assets: []document.Asset{{Kind: "pdf", Alt: fmt.Sprintf("page %d", p.page)}}}
		m.docImages = map[int]*documentImage{0: placeholder}
		m.docSearch = documentSearch{}
		m.landPDF()
		return m.requestDocumentImages()
	}
	if m.doc == nil || m.doc.Kind == "PDF" {
		m.doc = &document.Document{Path: p.path, Kind: "TEXT"}
		m.docLayout = document.Layout{Lines: []document.Line{{Text: color(fmt.Sprintf("Loading page %d…", p.page), muted)}}}
		m.docImages = nil
		m.docOffset = 0
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.previewCancel = cancel
	id, r, info, page, path, width := m.previewID, m.pdfRenderer(), p.info, p.page, p.path, m.documentWidth()
	return func() tea.Msg {
		defer cancel()
		d, err := r.Text(ctx, info, page)
		var l document.Layout
		if err == nil {
			d.Path = path
			l, err = document.Render(ctx, d, width, false)
		}
		return pdfTextMsg{id, anchor, d, l, err}
	}
}

func (m *Model) applyPDFText(v pdfTextMsg) tea.Cmd {
	if v.id != m.previewID || m.pdf == nil {
		return nil
	}
	if errors.Is(v.err, document.ErrPDFChanged) {
		return m.reloadPDF()
	}
	p := m.pdf
	p.loading = false
	if v.err != nil {
		m.doc = &document.Document{Path: p.path, Kind: "TEXT"}
		m.docLayout = document.Layout{Lines: []document.Line{{Text: color(v.err.Error(), muted)}}}
		m.docOffset = 0
		return nil
	}
	m.doc, m.docLayout = v.doc, v.layout
	if p.land == pdfStay {
		m.restoreDocumentAnchor(v.anchor)
	}
	m.landPDF()
	p.land = pdfStay
	return nil
}

// pdfImageApplied finishes a page picture: it lands the view once the real
// rows are known, then warms the cache for the next page in reading order.
func (m *Model) pdfImageApplied(v documentImageMsg) tea.Cmd {
	m.pdf.loading = false
	m.landPDF()
	m.pdf.land = pdfStay
	if v.err != nil {
		return nil
	}
	next := m.pdf.page + m.pdf.direction
	if next < 1 || next > m.pdf.info.Pages {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.pdfPrefetchCancel = cancel
	r, info, columns := m.pdfRenderer(), m.pdf.info, m.pdfColumns()
	cellWidth, cellHeight := m.pdfCellSize()
	return func() tea.Msg {
		defer cancel()
		r.Page(ctx, info, next, columns, cellWidth, cellHeight)
		return pdfPrefetchMsg{}
	}
}

func (m *Model) cancelPDFPrefetch() {
	if m.pdfPrefetchCancel != nil {
		m.pdfPrefetchCancel()
		m.pdfPrefetchCancel = nil
	}
}

func (m *Model) landPDF() {
	switch m.pdf.land {
	case pdfTop:
		m.docOffset = 0
	case pdfBottom:
		m.docOffset = 1 << 30
	}
	m.scrollDocument(0)
}

func (m *Model) pdfGo(page int, land pdfLanding) tea.Cmd {
	p := m.pdf
	page = max(1, min(p.info.Pages, page))
	if m.notice == pdfSearchNotice || m.notice == pdfReloadedNotice {
		m.notice = ""
	}
	if page == p.page {
		p.land = land
		m.landPDF()
		if !p.loading {
			p.land = pdfStay
		}
		return m.requestDocumentImages()
	}
	p.direction = 1
	if page < p.page {
		p.direction = -1
	}
	p.page = page
	return m.showPDFPage(land)
}

// pdfTurn moves one page; at the first or last page it does nothing.
func (m *Model) pdfTurn(delta int, land pdfLanding) tea.Cmd {
	if page := m.pdf.page + delta; page >= 1 && page <= m.pdf.info.Pages {
		return m.pdfGo(page, land)
	}
	return nil
}

// pdfNavigate scrolls within the page and turns the page only from an edge the
// view already rests on, never while the next page is still loading.
func (m *Model) pdfNavigate(key string, count, delta int, absolute bool) tea.Cmd {
	p := m.pdf
	if absolute {
		switch {
		case key == "home":
			return m.pdfGo(1, pdfTop)
		case key == "end":
			return m.pdfGo(p.info.Pages, pdfBottom)
		case count > 0:
			return m.pdfGo(count, pdfTop)
		case key == "G":
			return m.pdfGo(p.info.Pages, pdfBottom)
		}
		return m.pdfGo(1, pdfTop)
	}
	lines, _ := m.documentComposition()
	if delta > 0 && m.docOffset >= len(lines)-m.layout().body {
		if p.loading {
			return nil
		}
		return m.pdfTurn(1, pdfTop)
	}
	if delta < 0 && m.docOffset <= 0 {
		if p.loading {
			return nil
		}
		return m.pdfTurn(-1, pdfBottom)
	}
	m.scrollDocument(delta)
	return m.requestDocumentImages()
}

func (m *Model) pdfTitle() string {
	mode := "PDF text"
	if m.pdfPicture() {
		mode = "PDF"
	}
	return fmt.Sprintf("%s · page %d/%d", mode, m.pdf.page, m.pdf.info.Pages)
}

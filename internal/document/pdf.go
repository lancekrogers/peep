package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// ErrPDFChanged means the file no longer matches the PDFInfo it was opened
	// as. Nothing from the changed file was cached; reopen it.
	ErrPDFChanged = errors.New("PDF changed on disk")
	// ErrNoPoppler means the poppler tools are not on PATH.
	ErrNoPoppler = errors.New("PDF preview needs poppler (pdfinfo, pdftoppm, pdftotext); run peep setup")
)

const (
	pdfCachePages = 24
	pdfCacheBytes = 64 << 20
	pdfMemoPaths  = 32
	pdfMemoPages  = 512
)

// PDFInfo identifies one version of a PDF file. Page and Text refuse it once
// the file at Path no longer has the same device, inode, size and mtime.
type PDFInfo struct {
	Path  string
	Hash  [32]byte
	Pages int
	id    fileID
}

type fileID struct {
	dev, ino    uint64
	size, mtime int64
}

type pdfPage struct {
	hash [32]byte
	page int
}

type pdfRaster struct {
	pdfPage
	width, height int
}

type pageGeometry struct {
	width, height float64
	rotate        int
}

type popplerRunner func(ctx context.Context, stdout io.Writer, tool string, args ...string) error

// PDF renders PDF pages and text through poppler. It owns bounded in-memory
// caches for the application's lifetime and never writes to the filesystem.
type PDF struct {
	run       popplerRunner
	mu        sync.Mutex
	pages     map[pdfRaster][]byte
	order     []pdfRaster
	bytes     int
	memo      map[string]PDFInfo
	memoOrder []string
	geometry  map[pdfPage]pageGeometry
}

func NewPDF() *PDF { return newPDF(runPoppler) }

func newPDF(run popplerRunner) *PDF {
	return &PDF{run: run, pages: map[pdfRaster][]byte{}, memo: map[string]PDFInfo{}, geometry: map[pdfPage]pageGeometry{}}
}

// Open identifies the PDF at path by content hash and page count. An unchanged
// file (same device, inode, size and mtime) is not read again.
func (p *PDF) Open(ctx context.Context, path string) (PDFInfo, error) {
	if err := ctx.Err(); err != nil {
		return PDFInfo{}, err
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return PDFInfo{}, fmt.Errorf("PDF path: %w", err)
	}
	f, err := OpenRegular(path)
	if err != nil {
		return PDFInfo{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return PDFInfo{}, fmt.Errorf("stat file: %w", err)
	}
	id := fileIDOf(st)
	p.mu.Lock()
	info, ok := p.memo[path]
	if ok && info.id == id {
		p.memoOrder = touch(p.memoOrder, path)
		p.mu.Unlock()
		return info, nil
	}
	p.mu.Unlock()
	out := &limitedOutput{limit: 1 << 20}
	if err := p.run(ctx, out, "pdfinfo", path); err != nil {
		return PDFInfo{}, err
	}
	pages, _, _ := parsePDFInfo(out.String(), 0)
	if pages < 1 {
		return PDFInfo{}, fmt.Errorf("PDF has no pages")
	}
	h := sha256.New()
	chunk := make([]byte, 256<<10)
	for {
		if err := ctx.Err(); err != nil {
			return PDFInfo{}, err
		}
		n, e := f.Read(chunk)
		h.Write(chunk[:n])
		if e == io.EOF {
			break
		}
		if e != nil {
			return PDFInfo{}, fmt.Errorf("read PDF: %w", e)
		}
	}
	info = PDFInfo{Path: path, Pages: pages, id: id}
	h.Sum(info.Hash[:0])
	if err := verify(info); err != nil {
		return PDFInfo{}, err
	}
	p.remember(info)
	return info, nil
}

// Page renders one page into exactly columns × rows cells. pixelWidth and
// pixelHeight give the pixel size of that area; the raster fits inside it at
// the page's aspect ratio, within the document pixel limits. Zero pixel sizes
// assume 10 × 20 pixel cells.
func (p *PDF) Page(ctx context.Context, info PDFInfo, page, columns, rows, pixelWidth, pixelHeight int) (Image, error) {
	if err := check(ctx, info, page); err != nil {
		return Image{}, err
	}
	if columns < 1 || rows < 1 || columns > 1024 || rows > 512 {
		return Image{}, fmt.Errorf("PDF preview area must be 1–1024 columns by 1–512 rows")
	}
	if pixelWidth <= 0 || pixelHeight <= 0 {
		pixelWidth, pixelHeight = columns*10, rows*20
	}
	g, err := p.pageGeometry(ctx, info, page)
	if err != nil {
		return Image{}, err
	}
	w, h := rasterSize(g, pixelWidth, pixelHeight)
	key := pdfRaster{pdfPage{info.Hash, page}, w, h}
	if data := p.cached(key); data != nil {
		return gridImage(data, columns, rows, pixelWidth, pixelHeight)
	}
	out := &limitedOutput{limit: 20 << 20}
	n := strconv.Itoa(page)
	err = p.run(ctx, out, "pdftoppm", "-png", "-f", n, "-l", n, "-singlefile", "-cropbox", "-scale-dimension-before-rotation", "-scale-to-x", strconv.Itoa(w), "-scale-to-y", strconv.Itoa(h), info.Path)
	if err != nil {
		return Image{}, err
	}
	if err := verify(info); err != nil {
		return Image{}, err
	}
	if out.Len() == 0 {
		return Image{}, fmt.Errorf("PDF page %d produced no image", page)
	}
	im, err := gridImage(out.Bytes(), columns, rows, pixelWidth, pixelHeight)
	if err != nil {
		return Image{}, err
	}
	p.store(key, im.PNG)
	return im, nil
}

// Text returns one page's text laid out as poppler sees it, sanitized like
// Read and capped at MaxBytes.
func (p *PDF) Text(ctx context.Context, info PDFInfo, page int) (*Document, error) {
	if err := check(ctx, info, page); err != nil {
		return nil, err
	}
	out := &cappedOutput{limit: MaxBytes}
	n := strconv.Itoa(page)
	if err := p.run(ctx, out, "pdftotext", "-f", n, "-l", n, "-layout", "-nopgbrk", "-cropbox", "-enc", "UTF-8", info.Path, "-"); err != nil {
		return nil, err
	}
	if err := verify(info); err != nil {
		return nil, err
	}
	data := out.Bytes()
	if out.truncated {
		data = trimPartialRune(data)
	}
	return &Document{Path: info.Path, Source: cleanText(strings.ToValidUTF8(string(data), "�")), Kind: "TEXT", Truncated: out.truncated}, nil
}

func (p *PDF) pageGeometry(ctx context.Context, info PDFInfo, page int) (pageGeometry, error) {
	key := pdfPage{info.Hash, page}
	p.mu.Lock()
	g, ok := p.geometry[key]
	p.mu.Unlock()
	if ok {
		return g, nil
	}
	out := &limitedOutput{limit: 1 << 20}
	n := strconv.Itoa(page)
	if err := p.run(ctx, out, "pdfinfo", "-f", n, "-l", n, info.Path); err != nil {
		return pageGeometry{}, err
	}
	if err := verify(info); err != nil {
		return pageGeometry{}, err
	}
	_, g, ok = parsePDFInfo(out.String(), page)
	if !ok {
		return pageGeometry{}, fmt.Errorf("PDF page %d has no usable size", page)
	}
	p.mu.Lock()
	if len(p.geometry) >= pdfMemoPages {
		clear(p.geometry)
	}
	p.geometry[key] = g
	p.mu.Unlock()
	return g, nil
}

func (p *PDF) cached(key pdfRaster) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	data := p.pages[key]
	if data != nil {
		p.order = touch(p.order, key)
	}
	return data
}

func (p *PDF) store(key pdfRaster, data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.pages[key]; exists {
		return
	}
	for len(p.order) > 0 && (len(p.order) >= pdfCachePages || p.bytes+len(data) > pdfCacheBytes) {
		p.bytes -= len(p.pages[p.order[0]])
		delete(p.pages, p.order[0])
		p.order = p.order[1:]
	}
	p.pages[key] = data
	p.order = append(p.order, key)
	p.bytes += len(data)
}

// remember records a path's current hash. Pages rendered from the path's
// previous content are dropped unless another remembered path still has it.
func (p *PDF) remember(info PDFInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if old, ok := p.memo[info.Path]; ok && old.Hash != info.Hash {
		delete(p.memo, info.Path)
		shared := false
		for _, other := range p.memo {
			shared = shared || other.Hash == old.Hash
		}
		if !shared {
			p.order = slices.DeleteFunc(p.order, func(k pdfRaster) bool {
				if k.hash != old.Hash {
					return false
				}
				p.bytes -= len(p.pages[k])
				delete(p.pages, k)
				return true
			})
			for k := range p.geometry {
				if k.hash == old.Hash {
					delete(p.geometry, k)
				}
			}
		}
	}
	p.memo[info.Path] = info
	p.memoOrder = touch(p.memoOrder, info.Path)
	for len(p.memoOrder) > pdfMemoPaths {
		delete(p.memo, p.memoOrder[0])
		p.memoOrder = p.memoOrder[1:]
	}
}

// touch moves key to the most recently used end of order.
func touch[T comparable](order []T, key T) []T {
	if i := slices.Index(order, key); i >= 0 {
		order = slices.Delete(order, i, i+1)
	}
	return append(order, key)
}

func check(ctx context.Context, info PDFInfo, page int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if page < 1 || page > info.Pages {
		return fmt.Errorf("page %d is out of range (1–%d)", page, info.Pages)
	}
	return verify(info)
}

func verify(info PDFInfo) error {
	st, err := os.Stat(info.Path)
	if err != nil || !st.Mode().IsRegular() || fileIDOf(st) != info.id {
		return ErrPDFChanged
	}
	return nil
}

func fileIDOf(st os.FileInfo) fileID {
	id := fileID{size: st.Size(), mtime: st.ModTime().UnixNano()}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		id.dev, id.ino = uint64(sys.Dev), uint64(sys.Ino)
	}
	return id
}

// parsePDFInfo reads pdfinfo output. Later lines win because document metadata,
// which a PDF author controls, is printed before the page count and sizes.
func parsePDFInfo(out string, page int) (pages int, g pageGeometry, ok bool) {
	var sized bool
	n := strconv.Itoa(page)
	for line := range strings.Lines(out) {
		f := strings.Fields(line)
		switch {
		case len(f) == 2 && f[0] == "Pages:":
			pages, _ = strconv.Atoi(f[1])
		case len(f) >= 6 && f[0] == "Page" && f[1] == n && f[2] == "size:" && f[4] == "x":
			w, errW := strconv.ParseFloat(f[3], 64)
			h, errH := strconv.ParseFloat(f[5], 64)
			sized = errW == nil && errH == nil && w > 0 && h > 0 && !math.IsInf(w*h, 0)
			g.width, g.height = w, h
		case len(f) == 4 && f[0] == "Page" && f[1] == n && f[2] == "rot:":
			r, _ := strconv.Atoi(f[3])
			g.rotate = (r%360 + 360) % 360
		}
	}
	return pages, g, sized
}

// rasterSize fits the displayed (rotated) page inside the box and within the
// document pixel limits, so poppler never allocates a raster ImageFromPNG rejects.
func rasterSize(g pageGeometry, boxWidth, boxHeight int) (int, int) {
	w, h := g.width, g.height
	if g.rotate%180 != 0 {
		w, h = h, w
	}
	scale := min(float64(boxWidth)/w, float64(boxHeight)/h, 4096/w, 4096/h, math.Sqrt(float64(8<<20)/(w*h)))
	rw, rh := min(4096, max(1, int(w*scale))), min(4096, max(1, int(h*scale)))
	for rw*rh > 8<<20 {
		if rw > rh {
			rw--
		} else {
			rh--
		}
	}
	return rw, rh
}

// cappedOutput keeps the first limit bytes and discards the rest, so chatty
// stderr or long text cannot fail an otherwise successful run.
type cappedOutput struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := min(len(p), b.limit-b.Len())
	b.truncated = b.truncated || n < len(p)
	b.Buffer.Write(p[:n])
	return len(p), nil
}

func runPoppler(ctx context.Context, stdout io.Writer, tool string, args ...string) error {
	path, err := exec.LookPath(tool)
	if err != nil {
		return ErrNoPoppler
	}
	child, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(child, path, args...)
	stderr := &cappedOutput{limit: 4096}
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = stdout, stderr, time.Second
	if err := cmd.Run(); err != nil {
		if child.Err() != nil {
			return fmt.Errorf("%s: %w", tool, child.Err())
		}
		return popplerError(tool, err, stderr.String())
	}
	return nil
}

func popplerError(tool string, err error, stderr string) error {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return fmt.Errorf("%s: %w", tool, err)
	}
	if strings.Contains(stderr, "Incorrect password") {
		return fmt.Errorf("PDF is password-protected; preview unavailable")
	}
	detail := err.Error()
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		detail = last
	}
	return fmt.Errorf("PDF could not be read: %.200s", cleanText(detail))
}

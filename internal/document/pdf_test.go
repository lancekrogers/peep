package document

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rc4"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const letterInfo = `Title:           Pages: 999
Producer:        Page    1 size:  1 x 1 pts
Pages:           3
Encrypted:       no
Page    1 size:  612 x 792 pts (letter)
Page    1 rot:   0
Page    2 size:  595.276 x 841.89 pts (A4)
Page    2 rot:   90
Page    3 size:  612 x 792 pts (letter)
Page    3 rot:   0
File size:       1152 bytes
`

type fakePoppler struct {
	mu       sync.Mutex
	calls    map[string]int
	info     string
	onRender func()
}

func (f *fakePoppler) run(ctx context.Context, stdout io.Writer, tool string, args ...string) error {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[tool]++
	f.mu.Unlock()
	switch tool {
	case "pdfinfo":
		_, err := io.WriteString(stdout, f.info)
		return err
	case "pdftoppm":
		w, _ := strconv.Atoi(args[slices.Index(args, "-scale-to-x")+1])
		h, _ := strconv.Atoi(args[slices.Index(args, "-scale-to-y")+1])
		im := image.NewNRGBA(image.Rect(0, 0, w, h))
		im.Set(0, 0, color.NRGBA{R: 255, A: 255})
		if f.onRender != nil {
			f.onRender()
		}
		return png.Encode(stdout, im)
	}
	_, err := io.WriteString(stdout, "page text\r\nwith \x1b[31mescape\x00\n")
	return err
}

func (f *fakePoppler) count(tool string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[tool]
}

func writeFile(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestParsePDFInfo(t *testing.T) {
	pages, g, ok := parsePDFInfo(letterInfo, 2)
	if pages != 3 || !ok || g.width != 595.276 || g.height != 841.89 || g.rotate != 90 {
		t.Fatalf("pages %d geometry %+v ok %v", pages, g, ok)
	}
	if _, g, ok := parsePDFInfo(letterInfo, 1); !ok || g.width != 612 || g.rotate != 0 {
		t.Fatalf("page 1 %+v", g)
	}
	for _, out := range []string{"Pages: 3\n", "Pages: 3\nPage    1 size:  0 x 792 pts\n", "Pages: 3\nPage    1 size:  a x 792 pts\n"} {
		if _, _, ok := parsePDFInfo(out, 1); ok {
			t.Fatalf("accepted %q", out)
		}
	}
	if _, g, _ := parsePDFInfo("Page    1 size:  10 x 10 pts\nPage    1 rot:   -90\n", 1); g.rotate != 270 {
		t.Fatalf("rotation %d", g.rotate)
	}
}

func TestPDFRasterSize(t *testing.T) {
	for _, c := range []struct {
		name         string
		g            pageGeometry
		boxW, boxH   int
		wantW, wantH int
	}{
		{"letter", pageGeometry{612, 792, 0}, 800, 800, 618, 800},
		{"letter keeps width", pageGeometry{612, 792, 0}, 800, 5000, 800, 1035},
		{"rotated", pageGeometry{612, 792, 90}, 800, 800, 800, 618},
		{"rotated 270", pageGeometry{612, 792, 270}, 800, 5000, 800, 618},
		{"tiny", pageGeometry{1, 1, 0}, 800, 600, 600, 600},
		{"tall", pageGeometry{200, 20000, 0}, 4096, 1 << 20, 40, 4096},
		{"wide", pageGeometry{20000, 200, 0}, 1 << 20, 4096, 4096, 40},
		{"huge box", pageGeometry{612, 792, 0}, 1 << 20, 1 << 20, 2545, 3294},
		{"thread", pageGeometry{1, 14400, 0}, 4096, 4096, 1, 4096},
	} {
		w, h := rasterSize(c.g, c.boxW, c.boxH)
		if w != c.wantW || h != c.wantH || w*h > 8<<20 {
			t.Errorf("%s: %dx%d, want %dx%d", c.name, w, h, c.wantW, c.wantH)
		}
	}
}

func TestGridImageFillsRequestedCells(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := range 10 {
		for x := range 20 {
			im.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, im)
	got, err := gridImage(b.Bytes(), 10, 10, 100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if got.Columns != 10 || got.Rows != 10 || len(got.Cells) != 10 {
		t.Fatalf("grid %dx%d with %d rows", got.Columns, got.Rows, len(got.Cells))
	}
	for i, row := range got.Cells {
		if n := strings.Count(row, "▀"); n != 10 {
			t.Fatalf("row %d has %d cells", i, n)
		}
	}
	panel := "\x1b[38;2;23;30;45m\x1b[48;2;23;30;45m▀"
	if !strings.HasPrefix(got.Cells[0], panel) || !strings.HasPrefix(got.Cells[9], panel) || !strings.Contains(got.Cells[5], "255;0;0") {
		t.Fatalf("image not centered vertically on the panel: %q", got.Cells)
	}
	if _, err := gridImage([]byte("broken"), 10, 10, 100, 200); err == nil {
		t.Fatal("bad PNG accepted")
	}
}

func TestPDFMissingPoppler(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.pdf")
	writeFile(t, path, "%PDF-1.4", time.Now())
	t.Setenv("PATH", t.TempDir())
	_, err := NewPDF().Open(context.Background(), path)
	if !errors.Is(err, ErrNoPoppler) || !strings.Contains(err.Error(), "peep setup") {
		t.Fatal(err)
	}
}

func TestPopplerErrors(t *testing.T) {
	exit := exec.Command("sh", "-c", "exit 1").Run()
	err := popplerError("pdfinfo", exit, "Syntax Warning: noise\nCommand Line Error: Incorrect password\n")
	if err == nil || err.Error() != "PDF is password-protected; preview unavailable" {
		t.Fatal(err)
	}
	err = popplerError("pdfinfo", exit, strings.Repeat("Syntax Warning: x\n", 50)+"Syntax Error: Couldn't read xref table\x1b[2J\n")
	if err == nil || err.Error() != "PDF could not be read: Syntax Error: Couldn't read xref table�[2J" {
		t.Fatal(err)
	}
	out := &cappedOutput{limit: 4096}
	if n, err := out.Write(make([]byte, 10<<10)); n != 10<<10 || err != nil || out.Len() != 4096 || !out.truncated {
		t.Fatal("stderr cap failed a run or kept too much")
	}
}

func TestPDFPageCacheAndStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.pdf")
	base := time.Unix(1_700_000_000, 0)
	writeFile(t, path, "first", base)
	fake := &fakePoppler{info: letterInfo}
	p := newPDF(fake.run)
	ctx := context.Background()
	info, err := p.Open(ctx, path)
	if err != nil || info.Pages != 3 {
		t.Fatal(info, err)
	}
	if again, err := p.Open(ctx, path); err != nil || again != info || fake.count("pdfinfo") != 1 {
		t.Fatal("unchanged file was read again", err)
	}
	im, err := p.Page(ctx, info, 1, 40, 20, 800, 800)
	if err != nil || im.Width != 618 || im.Height != 800 || len(im.Cells) != 20 {
		t.Fatalf("%dx%d %d rows: %v", im.Width, im.Height, len(im.Cells), err)
	}
	if _, err := p.Page(ctx, info, 1, 80, 40, 800, 800); err != nil || fake.count("pdftoppm") != 1 || fake.count("pdfinfo") != 2 {
		t.Fatal("same raster rendered twice", err)
	}
	if _, err := p.Page(ctx, info, 4, 40, 20, 800, 800); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatal(err)
	}
	if doc, err := p.Text(ctx, info, 3); err != nil || doc.Source != "page text\nwith �[31mescape�\n" {
		t.Fatalf("%+v %v", doc, err)
	}

	writeFile(t, path, "first", base.Add(time.Second))
	if _, err := p.Page(ctx, info, 1, 40, 20, 800, 800); !errors.Is(err, ErrPDFChanged) {
		t.Fatal("stale info accepted", err)
	}
	same, err := p.Open(ctx, path)
	if err != nil || same.Hash != info.Hash || len(p.pages) != 1 {
		t.Fatal("touching a file dropped its pages", err)
	}
	writeFile(t, path, "second version", base)
	changed, err := p.Open(ctx, path)
	if err != nil || changed.Hash == info.Hash || len(p.pages) != 0 || len(p.geometry) != 0 {
		t.Fatal("pages of replaced content kept", err)
	}

	fake.onRender = func() { writeFile(t, path, "third, mid-render", base) }
	if _, err := p.Page(ctx, changed, 2, 40, 20, 800, 800); !errors.Is(err, ErrPDFChanged) || len(p.pages) != 0 {
		t.Fatal("render of a swapped file was cached", err)
	}
}

func TestPDFCacheBounds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.pdf")
	writeFile(t, path, "pdf", time.Now())
	info := strings.Builder{}
	info.WriteString("Pages: 40\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&info, "Page %d size: 100 x 100 pts\nPage %d rot: 0\n", i, i)
	}
	fake := &fakePoppler{info: info.String()}
	p := newPDF(fake.run)
	ctx := context.Background()
	doc, err := p.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for page := 1; page <= pdfCachePages; page++ {
		if _, err := p.Page(ctx, doc, page, 10, 5, 100, 100); err != nil {
			t.Fatal(err)
		}
	}
	p.Page(ctx, doc, 1, 10, 5, 100, 100)
	p.Page(ctx, doc, pdfCachePages+1, 10, 5, 100, 100)
	renders := fake.count("pdftoppm")
	p.Page(ctx, doc, 1, 10, 5, 100, 100)
	if fake.count("pdftoppm") != renders || len(p.pages) != pdfCachePages {
		t.Fatal("recently used page evicted or cache overfilled")
	}
	p.Page(ctx, doc, 2, 10, 5, 100, 100)
	if fake.count("pdftoppm") != renders+1 {
		t.Fatal("least recently used page survived")
	}

	big := newPDF(nil)
	for i := range 8 {
		big.store(pdfRaster{width: i + 1}, make([]byte, 15<<20))
	}
	if big.bytes > pdfCacheBytes || len(big.pages) != 4 || big.cached(pdfRaster{width: 8}) == nil {
		t.Fatalf("cache holds %d bytes in %d entries", big.bytes, len(big.pages))
	}

	for i := range pdfMemoPaths + 5 {
		other := filepath.Join(dir, fmt.Sprintf("%d.pdf", i))
		writeFile(t, other, strconv.Itoa(i), time.Now())
		if _, err := p.Open(ctx, other); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.memo) != pdfMemoPaths || len(p.memoOrder) != pdfMemoPaths {
		t.Fatalf("memo holds %d paths", len(p.memo))
	}
}

type testPage struct {
	width, height, rotate int
	text                  string
}

// testPDF assembles a PDF with one Helvetica text line per page. A non-empty
// password adds revision 2 standard encryption, enough for poppler to refuse it.
func testPDF(password string, pages ...testPage) []byte {
	var b bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	b.WriteString("%PDF-1.4\n")
	var kids []string
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 4+2*i))
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i, p := range pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Rotate %d /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", p.width, p.height, p.rotate, 5+2*i))
		s := fmt.Sprintf("BT /F1 24 Tf 40 %d Td (%s) Tj ET", p.height-60, p.text)
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(s), s))
	}
	trailer := ""
	if password != "" {
		pad := func(pw string) []byte {
			return append([]byte(pw), "\x28\xbf\x4e\x5e\x4e\x75\x8a\x41\x64\x00\x4e\x56\xff\xfa\x01\x08\x2e\x2e\x00\xb6\xd0\x68\x3e\x80\x2f\x0c\xa9\xfe\x64\x53\x69\x7a"[:32-len(pw)]...)
		}
		crypt := func(key, data []byte) []byte {
			c, _ := rc4.NewCipher(key)
			out := make([]byte, len(data))
			c.XORKeyStream(out, data)
			return out
		}
		id := []byte("0123456789abcdef")
		ownerKey := md5.Sum(pad("owner"))
		o := crypt(ownerKey[:5], pad(password))
		key := md5.Sum(slices.Concat(pad(password), o, []byte{0xfc, 0xff, 0xff, 0xff}, id))
		obj(fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /O <%x> /U <%x> /P -4 >>", o, crypt(key[:5], pad(""))))
		trailer = fmt.Sprintf(" /Encrypt %d 0 R /ID [<%s> <%s>]", len(offsets), hex.EncodeToString(id), hex.EncodeToString(id))
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R%s >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, trailer, xref)
	return b.Bytes()
}

func popplerTest(t *testing.T) {
	if os.Getenv("PEEP_INTEGRATION") == "" {
		t.Skip("set PEEP_INTEGRATION=1 with poppler installed")
	}
}

func TestPDFPoppler(t *testing.T) {
	popplerTest(t)
	path := filepath.Join(t.TempDir(), "doc.pdf")
	os.WriteFile(path, testPDF("", testPage{612, 792, 0, "Hello alpha"}, testPage{612, 792, 90, "Rotated beta"}, testPage{200, 20000, 0, "Tall gamma"}), 0600)
	var renders int
	var swap func()
	p := newPDF(func(ctx context.Context, stdout io.Writer, tool string, args ...string) error {
		err := runPoppler(ctx, stdout, tool, args...)
		if tool == "pdftoppm" {
			renders++
			if swap != nil {
				swap()
			}
		}
		return err
	})
	ctx := context.Background()
	info, err := p.Open(ctx, path)
	if err != nil || info.Pages != 3 {
		t.Fatal(info, err)
	}
	for _, c := range []struct{ page, boxW, boxH, w, h int }{{1, 800, 800, 618, 800}, {2, 800, 800, 800, 618}, {3, 4096, 1 << 20, 40, 4096}} {
		im, err := p.Page(ctx, info, c.page, 80, 40, c.boxW, c.boxH)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(im.PNG))
		if err != nil || cfg.Width != c.w || cfg.Height != c.h || len(im.Cells) != 40 {
			t.Fatalf("page %d: %dx%d, want %dx%d (%v)", c.page, cfg.Width, cfg.Height, c.w, c.h, err)
		}
	}
	first, _ := p.Page(ctx, info, 1, 80, 40, 800, 800)
	if renders != 3 {
		t.Fatalf("cached page rendered again: %d renders", renders)
	}
	for page, want := range map[int]string{1: "Hello alpha", 2: "Rotated beta"} {
		if doc, err := p.Text(ctx, info, page); err != nil || !strings.Contains(doc.Source, want) {
			t.Fatalf("page %d text %+v: %v", page, doc, err)
		}
	}

	os.WriteFile(path, testPDF("", testPage{612, 792, 0, "Changed delta and more"}), 0600)
	if _, err := p.Page(ctx, info, 1, 80, 40, 800, 800); !errors.Is(err, ErrPDFChanged) {
		t.Fatal("rewritten file not detected", err)
	}
	changed, err := p.Open(ctx, path)
	if err != nil || changed.Hash == info.Hash || changed.Pages != 1 {
		t.Fatal(changed, err)
	}
	second, err := p.Page(ctx, changed, 1, 80, 40, 800, 800)
	if err != nil || bytes.Equal(first.PNG, second.PNG) {
		t.Fatal("new content rendered the old image", err)
	}

	swap = func() { os.WriteFile(path, testPDF("", testPage{612, 792, 0, "Swapped epsilon text"}), 0600) }
	cached := len(p.pages)
	if _, err := p.Page(ctx, changed, 1, 40, 20, 400, 400); !errors.Is(err, ErrPDFChanged) || len(p.pages) != cached {
		t.Fatal("render of a swapped file was cached", err)
	}

	locked := filepath.Join(filepath.Dir(path), "locked.pdf")
	os.WriteFile(locked, testPDF("secret", testPage{612, 792, 0, "Hidden"}), 0600)
	if _, err := p.Open(ctx, locked); err == nil || err.Error() != "PDF is password-protected; preview unavailable" {
		t.Fatal(err)
	}
	broken := filepath.Join(filepath.Dir(path), "broken.pdf")
	os.WriteFile(broken, []byte("not a pdf"), 0600)
	if _, err := p.Open(ctx, broken); err == nil || !strings.HasPrefix(err.Error(), "PDF could not be read: ") {
		t.Fatal(err)
	}
}

func TestPDFWritesNothing(t *testing.T) {
	popplerTest(t)
	path := filepath.Join(t.TempDir(), "doc.pdf")
	os.WriteFile(path, testPDF("", testPage{612, 792, 0, "Hello alpha"}, testPage{612, 792, 90, "Rotated beta"}), 0600)
	var dirs []string
	for _, env := range []string{"TMPDIR", "HOME", "XDG_CACHE_HOME"} {
		dir := t.TempDir()
		t.Setenv(env, dir)
		dirs = append(dirs, dir)
	}
	p := NewPDF()
	ctx := context.Background()
	info, err := p.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for page := 1; page <= info.Pages; page++ {
		if _, err := p.Page(ctx, info, page, 60, 30, 600, 600); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Text(ctx, info, page); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range dirs {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Fatalf("%s gained %v", dir, entries)
		}
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("fixture directory gained %v", entries)
	}
}

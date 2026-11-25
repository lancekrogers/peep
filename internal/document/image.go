package document

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/url"
	"path/filepath"

	"github.com/lancekrogers/peep/internal/media"
	"github.com/lancekrogers/peep/internal/render"
)

type Image struct {
	PNG                          []byte
	Width, Height, Columns, Rows int
	Cells                        []string
}

func ResolveImage(documentPath, ref string) (string, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid image reference")
	}
	if u.Scheme != "" || u.Host != "" {
		return "", fmt.Errorf("remote images are not fetched")
	}
	path := u.Path
	if path == "" {
		return "", fmt.Errorf("empty image reference")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(documentPath), path)
	}
	return filepath.Clean(path), nil
}
func FitImage(w, h, columns int) (int, int) {
	columns = max(1, min(columns, 120))
	rows := max(1, int(math.Ceil(float64(columns*h)/float64(max(1, w)*2))))
	if rows > 24 {
		rows = 24
		columns = max(1, min(columns, int(float64(rows*w*2)/float64(max(1, h)))))
	}
	return columns, rows
}
func ReadImage(ctx context.Context, path string, columns int) (Image, error) {
	f, err := OpenRegular(path)
	if err != nil {
		return Image{}, err
	}
	f.Close()
	data, err := media.PreviewPNG(ctx, path)
	if err != nil {
		return Image{}, err
	}
	return ImageFromPNG(data, columns)
}
func ImageFromPNG(data []byte, columns int) (Image, error) {
	im, err := decodePNG(data)
	if err != nil {
		return Image{}, err
	}
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	cols, rows := FitImage(w, h, columns)
	return Image{PNG: data, Width: w, Height: h, Columns: cols, Rows: rows, Cells: cells(im, cols, rows, image.Rect(0, 0, cols, rows*2))}, nil
}

// gridImage fills exactly columns × rows cells covering boxWidth × boxHeight
// pixels, centering the image at its own aspect ratio on the panel background.
func gridImage(data []byte, columns, rows, boxWidth, boxHeight int) (Image, error) {
	im, err := decodePNG(data)
	if err != nil {
		return Image{}, err
	}
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	scale := min(float64(boxWidth)/float64(w), float64(boxHeight)/float64(h))
	sw := min(columns, max(1, int(math.Round(float64(w)*scale*float64(columns)/float64(boxWidth)))))
	sh := min(rows*2, max(1, int(math.Round(float64(h)*scale*float64(rows*2)/float64(boxHeight)))))
	x, y := (columns-sw)/2, (rows*2-sh)/2
	return Image{PNG: data, Width: w, Height: h, Columns: columns, Rows: rows, Cells: cells(im, columns, rows, image.Rect(x, y, x+sw, y+sh))}, nil
}

func decodePNG(data []byte) (image.Image, error) {
	if len(data) > 20<<20 {
		return nil, fmt.Errorf("document image exceeds 20 MiB")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image metadata: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 8<<20 {
		return nil, fmt.Errorf("image exceeds document pixel limit")
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode document image: %w", err)
	}
	return im, nil
}

// cells samples im into area, given in half-cell units of a cols × rows grid;
// samples outside area show the panel background.
func cells(im image.Image, cols, rows int, area image.Rectangle) []string {
	b := im.Bounds()
	f := media.Frame{Width: cols, Height: rows * 2, RGB: make([]byte, cols*rows*6)}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < cols; x++ {
			var c color.Color = color.Transparent
			if image.Pt(x, y).In(area) {
				c = im.At(b.Min.X+(x-area.Min.X)*b.Dx()/area.Dx(), b.Min.Y+(y-area.Min.Y)*b.Dy()/area.Dy())
			}
			r, g, bl, a := c.RGBA()
			at := (y*cols + x) * 3
			// Composite transparency onto the document panel background.
			f.RGB[at] = uint8((r + 23*(65535-a)/255) >> 8)
			f.RGB[at+1] = uint8((g + 30*(65535-a)/255) >> 8)
			f.RGB[at+2] = uint8((bl + 45*(65535-a)/255) >> 8)
		}
	}
	return render.Cells(f)
}

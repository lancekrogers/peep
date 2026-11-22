package tui

import (
	"fmt"
	"testing"

	"github.com/lancekrogers/peep/internal/document"
)

func documentImageModel() *Model {
	m := imageModel()
	m.preview.Path = "/docs/readme.md"
	m.doc = &document.Document{Path: "/docs/readme.md", Kind: "MD"}
	m.docLayout = document.Layout{Lines: []document.Line{{Text: "text", SourceLine: 0}, {Text: "after", SourceLine: 10}}, Assets: []document.Asset{{Kind: "image", Ref: "test.png", Line: 1, SourceLine: 2}}}
	m.docImages = map[int]*documentImage{0: {image: document.Image{PNG: []byte("png"), Width: 120, Height: 96}, width: m.documentWidth(), key: 1}}
	return m
}
func TestDocumentImageClippingAndSource(t *testing.T) {
	m := documentImageModel()
	m.docOffset = 3
	graphics := m.documentGraphics(40, 3)
	if len(graphics) != 1 || graphics[0].Y <= 0 || graphics[0].Rect.Y != 3 || graphics[0].Rect.Rows > m.layout().body {
		t.Fatalf("bad clipping: %+v", graphics)
	}
	m.docSource = true
	if len(m.documentGraphics(0, 0)) != 0 {
		t.Fatal("source view retained image")
	}
}
func TestDocumentImageErrorAndStaleResult(t *testing.T) {
	m := documentImageModel()
	m.applyDocumentImage(documentImageMsg{id: m.previewID - 1, index: 0, err: fmt.Errorf("stale")})
	if m.docImages[0].err != "" {
		t.Fatal("stale result replaced image")
	}
	m.applyDocumentImage(documentImageMsg{id: m.previewID, index: 0, err: fmt.Errorf("missing image")})
	if m.docImages[0] == nil || m.docImages[0].err != "missing image" {
		t.Fatal("visible error was evicted")
	}
	if cmd := m.requestDocumentImages(); cmd != nil {
		t.Fatal("failed image retried repeatedly")
	}
	m.clearDocument()
	if len(m.docImages) != 0 || m.docImageCancel != nil {
		t.Fatal("navigation retained images")
	}
}

func TestEvictedImageKeepsScrollGeometry(t *testing.T) {
	m := documentImageModel()
	before, _ := m.documentComposition()
	m.docOffset = 100
	m.requestDocumentImages()
	after, _ := m.documentComposition()
	if len(before) != len(after) || len(m.docImages[0].image.PNG) != 0 {
		t.Fatal("offscreen eviction changed document height or retained pixels")
	}
	m.docOffset = 0
	if m.requestDocumentImages() == nil {
		t.Fatal("returning to evicted image did not reload it")
	}
	m.cancelDocumentImages()
}

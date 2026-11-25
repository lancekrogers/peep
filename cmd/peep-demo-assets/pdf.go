package main

import (
	"bytes"
	"fmt"
	"strings"
)

type logPage struct {
	background, shade, accent, ink [3]float64
	numeral, title                 string
	shape                          string
	entries                        []string
}

var fieldLog = []logPage{
	{[3]float64{.06, .10, .21}, [3]float64{.78, .85, .80}, [3]float64{.35, .84, .75}, [3]float64{.88, .91, .95}, "1", "Dusk", "moon", []string{
		"19:40 Antenna checked and aligned to the north ridge.",
		"19:55 Dome open. Sky clear, light wind from the west.",
		"20:10 First recording started: the quiet before moonrise.",
	}},
	{[3]float64{.95, .62, .32}, [3]float64{.80, .40, .22}, [3]float64{.32, .12, .20}, [3]float64{.20, .08, .12}, "2", "Moonrise", "ridge", []string{
		"21:32 Moonrise over the observatory roof.",
		"22:05 Satellite pass logged at 41 degrees, steady and bright.",
		"22:40 Coffee. The kettle is the loudest thing for miles.",
	}},
	{[3]float64{.10, .34, .30}, [3]float64{.98, .86, .45}, [3]float64{.98, .86, .45}, [3]float64{.92, .96, .90}, "3", "Handoff", "stars", []string{
		"04:50 Last frames saved to the archive.",
		"05:20 Daybreak. Dome closed, heaters off.",
		"05:30 Note for the next shift: the antenna creaks, but it holds.",
	}},
}

// fieldLogPDF assembles the demo PDF by hand. It names only standard PDF
// fonts, so no font program is embedded or redistributed.
func fieldLogPDF() []byte {
	var b bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	b.WriteString("%PDF-1.4\n")
	const first = 7
	var kids []string
	for i := range fieldLog {
		kids = append(kids, fmt.Sprintf("%d 0 R", first+2*i))
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(fieldLog)))
	for _, font := range []string{"Helvetica-Bold", "Times-Roman", "Courier"} {
		obj("<< /Type /Font /Subtype /Type1 /BaseFont /" + font + " /Encoding /WinAnsiEncoding >>")
	}
	obj("<< /Title (Night Shift Field Log) /Author (peep demo collection) /Producer (peep-demo-assets) >>")
	for i, p := range fieldLog {
		content := p.content(i + 1)
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>", first+2*i+1))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R /Info 6 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return b.Bytes()
}

func (p logPage) content(number int) string {
	var c strings.Builder
	rgb := func(v [3]float64, op string) { fmt.Fprintf(&c, "%.2f %.2f %.2f %s\n", v[0], v[1], v[2], op) }
	rgb(p.background, "rg")
	c.WriteString("0 0 612 792 re f\n")
	rgb(p.shade, "rg")
	switch p.shape {
	case "moon":
		circle(&c, 440, 560, 110)
		rgb(p.background, "rg")
		circle(&c, 480, 590, 95)
	case "ridge":
		c.WriteString("0 340 m 150 580 l 260 450 l 400 650 l 612 380 l 612 340 l f\n")
	case "stars":
		for i, s := range [][3]float64{{420, 620, 34}, {520, 520, 22}, {360, 470, 16}, {540, 680, 12}, {460, 400, 26}} {
			star(&c, s[0], s[1], s[2], i%2 == 0)
		}
	}
	rgb(p.accent, "rg")
	fmt.Fprintf(&c, "BT /F1 360 Tf 40 360 Td (%s) Tj ET\n", p.numeral)
	rgb(p.ink, "rg")
	fmt.Fprintf(&c, "BT /F1 30 Tf 40 300 Td (Night Shift Field Log) Tj ET\n")
	fmt.Fprintf(&c, "BT /F2 22 Tf 40 266 Td (Page %d - %s) Tj ET\n", number, p.title)
	for i, entry := range p.entries {
		fmt.Fprintf(&c, "BT /F3 13 Tf 40 %d Td (%s) Tj ET\n", 210-28*i, entry)
	}
	fmt.Fprintf(&c, "BT /F2 11 Tf 40 60 Td (Original demonstration content for peep. Page %d of %d.) Tj ET\n", number, len(fieldLog))
	return c.String()
}

// circle fills a circle from four cubic Bézier arcs.
func circle(c *strings.Builder, x, y, r float64) {
	k := r * 0.5523
	fmt.Fprintf(c, "%.1f %.1f m %.1f %.1f %.1f %.1f %.1f %.1f c %.1f %.1f %.1f %.1f %.1f %.1f c %.1f %.1f %.1f %.1f %.1f %.1f c %.1f %.1f %.1f %.1f %.1f %.1f c f\n",
		x+r, y, x+r, y+k, x+k, y+r, x, y+r, x-k, y+r, x-r, y+k, x-r, y, x-r, y-k, x-k, y-r, x, y-r, x+k, y-r, x+r, y-k, x+r, y)
}

func star(c *strings.Builder, x, y, r float64, wide bool) {
	inner := r * 0.25
	if wide {
		inner = r * 0.4
	}
	fmt.Fprintf(c, "%.1f %.1f m %.1f %.1f l %.1f %.1f l %.1f %.1f l %.1f %.1f l %.1f %.1f l %.1f %.1f l %.1f %.1f l f\n",
		x, y+r, x+inner, y+inner, x+r, y, x+inner, y-inner, x, y-r, x-inner, y-inner, x-r, y, x-inner, y+inner)
}

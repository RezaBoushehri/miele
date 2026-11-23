// Package draw renders a board layout to an image for preview and printing.
package draw

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"miele/internal/optimize"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var palette = []color.RGBA{
	{0x8e, 0xca, 0xe6, 0xff}, {0xff, 0xb7, 0x03, 0xff}, {0x90, 0xbe, 0x6d, 0xff},
	{0xf2, 0x84, 0x82, 0xff}, {0xb8, 0xb8, 0xff, 0xff}, {0xf6, 0xbd, 0x60, 0xff},
	{0x84, 0xa5, 0x9d, 0xff}, {0xcd, 0xb4, 0xdb, 0xff}, {0xff, 0xaf, 0xcc, 0xff},
	{0xa2, 0xd2, 0xff, 0xff}, {0xe9, 0xc4, 0x6a, 0xff}, {0x95, 0xd5, 0xb2, 0xff},
}

// ColorFor returns a stable colour for a part index.
func ColorFor(i int) color.RGBA { return palette[i%len(palette)] }

// Board renders one board layout at the given pixel scale (mm -> px).
// names maps part name -> colour index.
func Board(bl *optimize.BoardLayout, st optimize.Settings, names map[string]int, scale float64) *image.RGBA {
	pad := 6
	w := int(math.Round(bl.L*scale)) + 2*pad
	h := int(math.Round(bl.W*scale)) + 2*pad
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), color.RGBA{0xff, 0xff, 0xff, 0xff})
	boardRect := image.Rect(pad, pad, pad+int(math.Round(bl.L*scale)), pad+int(math.Round(bl.W*scale)))
	fill(img, boardRect, color.RGBA{0xee, 0xee, 0xee, 0xff})
	border(img, boardRect, color.RGBA{0x55, 0x55, 0x55, 0xff})

	for _, p := range bl.Placements(st) {
		x0 := pad + int(math.Round(p.X*scale))
		y0 := pad + int(math.Round(p.Y*scale))
		x1 := pad + int(math.Round((p.X+p.DX)*scale))
		y1 := pad + int(math.Round((p.Y+p.DY)*scale))
		rc := image.Rect(x0, y0, x1, y1)
		idx := 0
		if p.Piece.Part != nil {
			idx = names[p.Piece.Part.Name]
		}
		fill(img, rc, palette[idx%len(palette)])
		border(img, rc, color.RGBA{0x33, 0x33, 0x33, 0xff})
		name := ""
		if p.Piece.Part != nil {
			name = p.Piece.Part.Name
		}
		label := fmt.Sprintf("%g x %g", p.DX, p.DY)
		cx, cy := (x0+x1)/2, (y0+y1)/2
		if x1-x0 > 34 && y1-y0 > 22 {
			drawCenter(img, name, cx, cy-6)
			drawCenter(img, label, cx, cy+7)
		} else if x1-x0 > 26 && y1-y0 > 10 {
			drawCenter(img, label, cx, cy+4)
		}
	}
	return img
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func border(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for x := r.Min.X; x < r.Max.X; x++ {
		img.SetRGBA(x, r.Min.Y, c)
		img.SetRGBA(x, r.Max.Y-1, c)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		img.SetRGBA(r.Min.X, y, c)
		img.SetRGBA(r.Max.X-1, y, c)
	}
}

func drawCenter(img *image.RGBA, s string, cx, cy int) {
	if s == "" {
		return
	}
	face := basicfont.Face7x13
	width := font.MeasureString(face, s).Round()
	d := &font.Drawer{
		Dst: img, Src: image.NewUniform(color.RGBA{0x20, 0x20, 0x20, 0xff}), Face: face,
		Dot: fixed.P(cx-width/2, cy+5),
	}
	d.DrawString(s)
}

// Title renders a header strip of the given width and height with left-aligned text.
func title(img *image.RGBA, y int, text string) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst: img, Src: image.NewUniform(color.RGBA{0x11, 0x11, 0x11, 0xff}), Face: face,
		Dot: fixed.P(8, y+14),
	}
	d.DrawString(text)
}

// Stack composes several board images into one tall image, each under a caption.
func Stack(imgs []*image.RGBA, captions []string, bg color.RGBA) *image.RGBA {
	const hdr = 22
	w, h := 1, 0
	for _, im := range imgs {
		if im.Bounds().Dx() > w {
			w = im.Bounds().Dx()
		}
		h += im.Bounds().Dy() + hdr + 8
	}
	out := image.NewRGBA(image.Rect(0, 0, w+16, h+8))
	fill(out, out.Bounds(), bg)
	y := 8
	for i, im := range imgs {
		cap := ""
		if i < len(captions) {
			cap = captions[i]
		}
		title(out, y, cap)
		y += hdr
		drawImageAt(out, im, 8, y)
		y += im.Bounds().Dy() + 8
	}
	return out
}

func drawImageAt(dst, src *image.RGBA, x, y int) {
	b := src.Bounds()
	for sy := b.Min.Y; sy < b.Max.Y; sy++ {
		for sx := b.Min.X; sx < b.Max.X; sx++ {
			dst.SetRGBA(x+sx-b.Min.X, y+sy-b.Min.Y, src.RGBAAt(sx, sy))
		}
	}
}

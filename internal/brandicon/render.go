package brandicon

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

//go:embed icon.png
var iconPNG []byte

// source is the decoded artwork with a transparent background.
var (
	source     *image.RGBA
	SpriteSize int
)

func init() {
	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		panic("brandicon: decode icon.png: " + err.Error())
	}
	source = toRGBATransparentBG(img)
	SpriteSize = source.Bounds().Dx()
}

// Render draws the PhraseMate icon at the given size (nearest-neighbor).
func Render(size int) *image.RGBA {
	return scaleNearest(source, size)
}

// RenderRounded builds a tray/shortcut-friendly icon: padded artwork on a soft
// rounded tile so jagged black outlines do not smear against the desktop.
func RenderRounded(size int) *image.RGBA {
	if size < 1 {
		size = 1
	}
	// Keep a quiet margin so the hard pixel outline is not clipped by the round mask.
	inner := size * 78 / 100
	if inner < 1 {
		inner = 1
	}
	art := scaleNearest(source, inner)

	out := image.NewRGBA(image.Rect(0, 0, size, size))
	// Soft warm plate behind the glyph (helps Windows alpha compositing).
	plate := color.RGBA{R: 0xf4, G: 0xf7, B: 0xfa, A: 255}
	radius := float64(size) * 0.22

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			cover := roundedRectCoverage(float64(x)+0.5, float64(y)+0.5, float64(size), radius)
			if cover <= 0 {
				continue
			}
			pr, pg, pb, pa := plate.RGBA()
			a := uint8(float64(pa>>8) * cover)
			out.Set(x, y, color.RGBA{R: uint8(pr >> 8), G: uint8(pg >> 8), B: uint8(pb >> 8), A: a})
		}
	}

	ox := (size - inner) / 2
	oy := ox
	for y := 0; y < inner; y++ {
		for x := 0; x < inner; x++ {
			sr, sg, sb, sa := art.At(x, y).RGBA()
			if sa == 0 {
				continue
			}
			dx := float64(ox+x) + 0.5
			dy := float64(oy+y) + 0.5
			cover := roundedRectCoverage(dx, dy, float64(size), radius)
			if cover <= 0 {
				continue
			}
			dst := out.RGBAAt(ox+x, oy+y)
			srcA := float64(sa>>8) / 255 * cover
			if srcA <= 0 {
				continue
			}
			dstA := float64(dst.A) / 255
			outA := srcA + dstA*(1-srcA)
			if outA <= 0 {
				continue
			}
			blend := func(sc, dc uint32) uint8 {
				s := float64(sc>>8) / 255
				d := float64(dc) / 255
				return uint8(((s*srcA + d*dstA*(1-srcA)) / outA) * 255)
			}
			out.Set(ox+x, oy+y, color.RGBA{
				R: blend(sr, uint32(dst.R)),
				G: blend(sg, uint32(dst.G)),
				B: blend(sb, uint32(dst.B)),
				A: uint8(outA * 255),
			})
		}
	}
	return out
}

func scaleNearest(src *image.RGBA, size int) *image.RGBA {
	if size < 1 {
		size = 1
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		sy := y * sh / size
		if sy >= sh {
			sy = sh - 1
		}
		for x := 0; x < size; x++ {
			sx := x * sw / size
			if sx >= sw {
				sx = sw - 1
			}
			out.Set(x, y, src.At(sb.Min.X+sx, sb.Min.Y+sy))
		}
	}
	return out
}

// roundedRectCoverage returns 0..1 coverage for an antialiased rounded square.
func roundedRectCoverage(px, py, size, radius float64) float64 {
	if radius < 1 {
		radius = 1
	}
	// Signed distance to rounded rect centered in [0,size]x[0,size].
	half := size / 2
	qx := math.Abs(px-half) - (half - radius)
	qy := math.Abs(py-half) - (half - radius)
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - radius
	// Soft 1px AA band.
	return clamp01(0.5 - outside)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// toRGBATransparentBG copies img to RGBA and clears near-white background pixels.
func toRGBATransparentBG(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			r, g, b, a := out.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			rr, gg, bb := uint8(r>>8), uint8(g>>8), uint8(b>>8)
			// Treat flat white / near-white canvas as transparent.
			if rr >= 245 && gg >= 245 && bb >= 245 {
				out.Set(x, y, color.RGBA{})
			}
		}
	}
	return out
}

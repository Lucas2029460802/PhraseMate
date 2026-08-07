package brandicon

import (
	"image"
	"image/color"
	"math"
)

// Render draws the PhraseMate mark at the given pixel size.
func Render(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	radius := float64(size) * 0.28
	cx, cy := float64(size)/2, float64(size)/2
	sea := color.RGBA{R: 31, G: 111, B: 120, A: 255}
	seaHi := color.RGBA{R: 42, G: 143, B: 153, A: 255}
	ink := color.RGBA{R: 238, G: 243, B: 246, A: 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !inRoundedSquare(x, y, size, radius) {
				img.Set(x, y, color.RGBA{})
				continue
			}
			t := float64(y) / float64(size-1)
			img.Set(x, y, lerpRGBA(seaHi, sea, t*0.85))
		}
	}
	drawGlyph(img, size, cx, cy, ink)
	return img
}

func inRoundedSquare(x, y, size int, radius float64) bool {
	fx, fy := float64(x)+0.5, float64(y)+0.5
	if fx < radius && fy < radius {
		return dist(fx, fy, radius, radius) <= radius
	}
	if fx > float64(size)-radius && fy < radius {
		return dist(fx, fy, float64(size)-radius, radius) <= radius
	}
	if fx < radius && fy > float64(size)-radius {
		return dist(fx, fy, radius, float64(size)-radius) <= radius
	}
	if fx > float64(size)-radius && fy > float64(size)-radius {
		return dist(fx, fy, float64(size)-radius, float64(size)-radius) <= radius
	}
	return true
}

func dist(x1, y1, x2, y2 float64) float64 {
	dx, dy := x1-x2, y1-y2
	return math.Sqrt(dx*dx + dy*dy)
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func drawGlyph(img *image.RGBA, size int, cx, cy float64, c color.RGBA) {
	scale := float64(size) / 32.0
	set := func(x, y int) {
		px := int(cx + float64(x-8)*scale)
		py := int(cy + float64(y-10)*scale)
		for dx := 0; dx < int(1.15*scale)+1; dx++ {
			for dy := 0; dy < int(1.15*scale)+1; dy++ {
				if px+dx >= 0 && py+dy >= 0 && px+dx < size && py+dy < size {
					img.Set(px+dx, py+dy, c)
				}
			}
		}
	}
	for y := 0; y < 12; y++ {
		set(0, y)
	}
	for x := 1; x <= 5; x++ {
		set(x, 0)
		set(x, 5)
	}
	for y := 1; y <= 4; y++ {
		set(5, y)
	}
	for y := 6; y < 12; y++ {
		set(8, y)
		set(11, y)
		set(14, y)
	}
	for x := 9; x <= 10; x++ {
		set(x, 6)
	}
	for x := 12; x <= 13; x++ {
		set(x, 6)
	}
}

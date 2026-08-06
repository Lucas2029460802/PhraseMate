//go:build windows

package tray

import (
	"image"
	"image/color"
	"math"
	"syscall"
	"unsafe"
)

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
)

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// createTrayIcon renders a branded PhraseMate tray icon (32×32).
func createTrayIcon() uintptr {
	return rgbaToHICON(renderIcon(32))
}

func renderIcon(size int) *image.RGBA {
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
			bg := lerpRGBA(seaHi, sea, t*0.85)
			img.Set(x, y, bg)
		}
	}

	// Stylized "Pm" mark
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
	// "P" stem + bowl + small "m" humps — pixel art at 32px
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
	// P
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
	// m
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

func rgbaToHICON(img *image.RGBA) uintptr {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	xor := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			i := (y*w + x) * 4
			xor[i+0] = byte(b >> 8)
			xor[i+1] = byte(g >> 8)
			xor[i+2] = byte(r >> 8)
			xor[i+3] = byte(a >> 8)
		}
	}

	andLine := ((w + 31) / 32) * 4
	andMask := make([]byte, andLine*h) // all zero = opaque for 32bpp alpha icons

	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)

	colorBmp := createDIB(hdc, w, h, xor)
	maskBmp := createDIB(hdc, w, h, andMask)
	if colorBmp == 0 {
		return fallbackIcon()
	}

	info := iconInfo{
		FIcon:    1,
		HbmMask:  maskBmp,
		HbmColor: colorBmp,
	}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	procDeleteObject.Call(colorBmp)
	if maskBmp != 0 {
		procDeleteObject.Call(maskBmp)
	}
	if icon == 0 {
		return fallbackIcon()
	}
	return icon
}

func createDIB(hdc uintptr, w, h int, pixels []byte) uintptr {
	var bi bitmapInfoHeader
	bi.Size = uint32(unsafe.Sizeof(bi))
	bi.Width = int32(w)
	bi.Height = int32(h)
	bi.Planes = 1
	bi.BitCount = 32
	bi.Compression = 0 // BI_RGB

	var bits uintptr
	hbmp, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&bi)),
		0,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hbmp == 0 || bits == 0 {
		return 0
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(bits)), len(pixels))
	copy(dst, pixels)
	return hbmp
}

func fallbackIcon() uintptr {
	icon, _, _ := procLoadIconW.Call(0, 32512) // IDI_APPLICATION
	return icon
}

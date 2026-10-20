//go:build windows

package tray

import (
	"image"
	"syscall"
	"unsafe"

	"phrasemate/internal/brandicon"
)

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
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
	return rgbaToHICON(brandicon.RenderRounded(32))
}

func rgbaToHICON(img *image.RGBA) uintptr {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Top-down BGRA buffer (matches CreateDIBSection with negative Height).
	xor := make([]byte, w*h*4)
	andLine := ((w + 31) / 32) * 4
	andMask := make([]byte, andLine*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			i := (y*w + x) * 4
			xor[i+0] = byte(b >> 8)
			xor[i+1] = byte(g >> 8)
			xor[i+2] = byte(r >> 8)
			xor[i+3] = byte(a >> 8)
			if a>>8 < 16 {
				byteIndex := y*andLine + x/8
				andMask[byteIndex] |= 0x80 >> uint(x%8)
			}
		}
	}

	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)

	colorBmp := createDIB(hdc, w, h, xor)
	maskBmp := createMonoMask(hdc, w, h, andMask)
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
	// Negative height = top-down DIB so scanlines match image.RGBA order.
	bi.Height = -int32(h)
	bi.Planes = 1
	bi.BitCount = 32
	bi.Compression = 0

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

func createMonoMask(hdc uintptr, w, h int, bits []byte) uintptr {
	var bi bitmapInfoHeader
	bi.Size = uint32(unsafe.Sizeof(bi))
	bi.Width = int32(w)
	bi.Height = -int32(h)
	bi.Planes = 1
	bi.BitCount = 1
	bi.Compression = 0

	// 1bpp needs a color table (2 RGBQUADs).
	type rgbQuad struct{ B, G, R, Reserved byte }
	hdr := struct {
		bitmapInfoHeader
		Colors [2]rgbQuad
	}{
		bitmapInfoHeader: bi,
		Colors: [2]rgbQuad{
			{0, 0, 0, 0},
			{255, 255, 255, 0},
		},
	}
	var bitsPtr uintptr
	hbmp, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&hdr)),
		0,
		uintptr(unsafe.Pointer(&bitsPtr)),
		0,
		0,
	)
	if hbmp == 0 || bitsPtr == 0 {
		return 0
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(bitsPtr)), len(bits))
	copy(dst, bits)
	return hbmp
}

func fallbackIcon() uintptr {
	icon, _, _ := procLoadIconW.Call(0, 32512) // IDI_APPLICATION
	return icon
}

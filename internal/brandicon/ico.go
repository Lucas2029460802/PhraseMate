package brandicon

import (
	"encoding/binary"
	"image"
	"os"
	"path/filepath"
)

// WriteICO writes a 32bpp ICO (with alpha) for the given RGBA image.
func WriteICO(path string, img *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	if w <= 0 || h <= 0 || w > 256 || h > 256 {
		w, h = 32, 32
		img = Render(32)
	}

	xorSize := w * h * 4
	andRow := ((w + 31) / 32) * 4
	andSize := andRow * h
	dibSize := 40 + xorSize + andSize

	buf := make([]byte, 6+16+dibSize)
	// ICONDIR
	binary.LittleEndian.PutUint16(buf[0:], 0) // reserved
	binary.LittleEndian.PutUint16(buf[2:], 1) // type = icon
	binary.LittleEndian.PutUint16(buf[4:], 1) // count
	// ICONDIRENTRY
	off := 6
	if w < 256 {
		buf[off] = byte(w)
	}
	if h < 256 {
		buf[off+1] = byte(h)
	}
	buf[off+2] = 0 // colors
	buf[off+3] = 0 // reserved
	binary.LittleEndian.PutUint16(buf[off+4:], 1)  // planes
	binary.LittleEndian.PutUint16(buf[off+6:], 32) // bit count
	binary.LittleEndian.PutUint32(buf[off+8:], uint32(dibSize))
	binary.LittleEndian.PutUint32(buf[off+12:], 22) // offset to image data

	data := buf[22:]
	// BITMAPINFOHEADER — height is 2*h for XOR+AND
	binary.LittleEndian.PutUint32(data[0:], 40)
	binary.LittleEndian.PutUint32(data[4:], uint32(w))
	binary.LittleEndian.PutUint32(data[8:], uint32(h*2))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	binary.LittleEndian.PutUint32(data[16:], 0)
	binary.LittleEndian.PutUint32(data[20:], uint32(xorSize+andSize))

	pixels := data[40:]
	for y := 0; y < h; y++ {
		srcY := h - 1 - y // bottom-up
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, srcY).RGBA()
			i := (y*w + x) * 4
			pixels[i+0] = byte(b >> 8)
			pixels[i+1] = byte(g >> 8)
			pixels[i+2] = byte(r >> 8)
			pixels[i+3] = byte(a >> 8)
		}
	}
	// AND mask left zeroed (opaque for 32bpp alpha icons)

	return os.WriteFile(path, buf, 0o644)
}

// EnsureFile writes PhraseMate.ico next to dir if missing/outdated size, returns path.
func EnsureFile(dir string) (string, error) {
	path := filepath.Join(dir, "PhraseMate.ico")
	if st, err := os.Stat(path); err == nil && !st.IsDir() && st.Size() > 100 {
		return path, nil
	}
	if err := WriteICO(path, Render(256)); err != nil {
		// Fallback smaller if 256 fails for any reason
		if err2 := WriteICO(path, Render(32)); err2 != nil {
			return "", err2
		}
	}
	return path, nil
}

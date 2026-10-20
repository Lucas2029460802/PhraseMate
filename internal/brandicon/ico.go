package brandicon

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"os"
	"path/filepath"
	"strings"
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
		img = RenderRounded(32)
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
	andMask := data[40+xorSize:]
	for y := 0; y < h; y++ {
		srcY := h - 1 - y // ICO XOR bitmap is bottom-up
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, srcY).RGBA()
			i := (y*w + x) * 4
			pixels[i+0] = byte(b >> 8)
			pixels[i+1] = byte(g >> 8)
			pixels[i+2] = byte(r >> 8)
			pixels[i+3] = byte(a >> 8)
			if a>>8 < 16 {
				byteIndex := y*andRow + x/8
				andMask[byteIndex] |= 0x80 >> uint(x%8)
			}
		}
	}

	return os.WriteFile(path, buf, 0o644)
}

// EnsureFile writes a content-hashed PhraseMate-*.ico into dir and returns its path.
// A new filename is used whenever the artwork changes so Windows icon cache cannot
// keep serving a stale image from the previous path.
func EnsureFile(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	img := RenderRounded(256)
	name := "PhraseMate-" + iconContentID(img) + ".ico"
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err != nil || st.IsDir() || st.Size() < 100 {
		if err := WriteICO(path, img); err != nil {
			fallback := filepath.Join(dir, "PhraseMate-"+iconContentID(RenderRounded(32))+".ico")
			if err2 := WriteICO(fallback, RenderRounded(32)); err2 != nil {
				return "", err2
			}
			path = fallback
			name = filepath.Base(path)
		}
	}
	cleanupOldIcons(dir, name)
	return path, nil
}

func iconContentID(img *image.RGBA) string {
	sum := sha256.Sum256(img.Pix)
	return hex.EncodeToString(sum[:])[:10]
}

func cleanupOldIcons(dir, keepName string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if n == keepName {
			continue
		}
		lower := strings.ToLower(n)
		if lower == "phrasemate.ico" || (strings.HasPrefix(lower, "phrasemate-") && strings.HasSuffix(lower, ".ico")) {
			_ = os.Remove(filepath.Join(dir, n))
		}
	}
}

package brandicon

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
)

// WriteICNS writes a PNG-based Apple icon file.
func WriteICNS(path string, sizes ...int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(sizes) == 0 {
		sizes = []int{16, 32, 64, 128, 256, 512}
	}
	wanted := map[int][]string{
		16:  {"icp4"},
		32:  {"icp5", "ic11"},
		64:  {"icp6", "ic12"},
		128: {"ic07"},
		256: {"ic08", "ic13"},
		512: {"ic09", "ic14"},
	}
	var body bytes.Buffer
	seen := map[int][]byte{}
	for _, size := range sizes {
		types, ok := wanted[size]
		if !ok {
			continue
		}
		pngBytes, ok := seen[size]
		if !ok {
			var buf bytes.Buffer
			if err := png.Encode(&buf, Render(size)); err != nil {
				return err
			}
			pngBytes = buf.Bytes()
			seen[size] = pngBytes
		}
		for _, ostype := range types {
			hdr := make([]byte, 8)
			copy(hdr[0:4], ostype)
			binary.BigEndian.PutUint32(hdr[4:8], uint32(8+len(pngBytes)))
			body.Write(hdr)
			body.Write(pngBytes)
		}
	}

	out := make([]byte, 8+body.Len())
	copy(out[0:4], "icns")
	binary.BigEndian.PutUint32(out[4:8], uint32(len(out)))
	copy(out[8:], body.Bytes())
	return os.WriteFile(path, out, 0o644)
}

// EnsureICNS writes AppIcon.icns into dir if missing.
func EnsureICNS(dir string) (string, error) {
	path := filepath.Join(dir, "AppIcon.icns")
	if st, err := os.Stat(path); err == nil && !st.IsDir() && st.Size() > 100 {
		return path, nil
	}
	if err := WriteICNS(path); err != nil {
		return "", err
	}
	return path, nil
}

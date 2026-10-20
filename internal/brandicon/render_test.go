package brandicon

import "testing"

func TestRenderMagnifierIcon(t *testing.T) {
	img := Render(64)
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("size = %v", img.Bounds())
	}
	opaque := 0
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				opaque++
			}
		}
	}
	if opaque < 200 {
		t.Fatalf("expected filled icon pixels, got %d opaque", opaque)
	}
	_, _, _, a := img.At(0, 0).RGBA()
	if a != 0 {
		t.Fatalf("corner should be transparent, alpha=%d", a>>8)
	}
}

func TestRenderRounded(t *testing.T) {
	img := RenderRounded(64)
	if img.Bounds().Dx() != 64 {
		t.Fatalf("size = %v", img.Bounds())
	}
	_, _, _, corner := img.At(0, 0).RGBA()
	if corner != 0 {
		t.Fatalf("rounded corner should be transparent, alpha=%d", corner>>8)
	}
	_, _, _, mid := img.At(32, 32).RGBA()
	if mid == 0 {
		t.Fatal("center should be opaque")
	}
}

package emulator

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestFrameDumpEncoderWritesPNGConcurrently(t *testing.T) {
	dir := t.TempDir()
	encoder := NewFrameDumpEncoder(2)
	defer encoder.Close()

	frameA := solidTestRGBA(2, 2, []byte{0x11, 0x22, 0x33, 0xFF})
	frameB := solidTestRGBA(2, 2, []byte{0x44, 0x55, 0x66, 0xFF})
	resultA := encoder.EncodePNG(filepath.Join(dir, "a.png"), 2, 2, frameA)
	resultB := encoder.EncodePNG(filepath.Join(dir, "nested", "b.png"), 2, 2, frameB)

	if err := <-resultA; err != nil {
		t.Fatalf("encode frame A: %v", err)
	}
	if err := <-resultB; err != nil {
		t.Fatalf("encode frame B: %v", err)
	}

	assertPNGPixel(t, filepath.Join(dir, "a.png"), []byte{0x11, 0x22, 0x33, 0xFF})
	assertPNGPixel(t, filepath.Join(dir, "nested", "b.png"), []byte{0x44, 0x55, 0x66, 0xFF})
}

func TestFrameDumpEncoderSnapshotsPixelsBeforeEncode(t *testing.T) {
	dir := t.TempDir()
	encoder := NewFrameDumpEncoder(1)
	defer encoder.Close()

	frame := solidTestRGBA(1, 1, []byte{0x10, 0x20, 0x30, 0xFF})
	result := encoder.EncodePNG(filepath.Join(dir, "snapshot.png"), 1, 1, frame)
	copy(frame, []byte{0xAA, 0xBB, 0xCC, 0xFF})

	if err := <-result; err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	assertPNGPixel(t, filepath.Join(dir, "snapshot.png"), []byte{0x10, 0x20, 0x30, 0xFF})
}

func solidTestRGBA(width, height int, rgba []byte) []byte {
	frame := make([]byte, width*height*4)
	for offset := 0; offset < len(frame); offset += 4 {
		copy(frame[offset:offset+4], rgba)
	}
	return frame
}

func assertPNGPixel(t *testing.T, path string, want []byte) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open PNG %s: %v", path, err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode PNG %s: %v", path, err)
	}
	got := img.At(0, 0)
	r, g, b, a := got.RGBA()
	pixel := []byte{byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)}
	for i := range want {
		if pixel[i] != want[i] {
			t.Fatalf("PNG pixel %s = %v, want %v", path, pixel, want)
		}
	}
}

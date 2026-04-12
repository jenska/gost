package emulator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDiskImageReturnsRawSTBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.st")
	want := make([]byte, 9*80*2*512)
	copy(want[:4], []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("write raw disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load raw disk image: %v", err)
	}
	if string(got.Data) != string(want) {
		t.Fatalf("unexpected raw disk image bytes: got %x want %x", got.Data[:4], want[:4])
	}
	if got.Geometry.SectorsPerTrack != 9 || got.Geometry.Sides != 2 || got.Geometry.Tracks != 80 {
		t.Fatalf("unexpected raw disk geometry: %+v", got.Geometry)
	}
}

func TestLoadDiskImageDecodesMSA(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.msa")
	msa := []byte{
		0x0E, 0x0F, // magic
		0x00, 0x01, // sectors per track
		0x00, 0x00, // one side
		0x00, 0x00, // start track
		0x00, 0x00, // end track
		0x00, 0x04, // compressed block length
		0xE5, 0x11, 0x02, 0x00, // repeat 0x11 for the full 512-byte track
	}
	if err := os.WriteFile(path, msa, 0o644); err != nil {
		t.Fatalf("write MSA disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load MSA disk image: %v", err)
	}
	if len(got.Data) != 512 {
		t.Fatalf("decoded MSA length = %d, want 512", len(got.Data))
	}
	for i := range len(got.Data) {
		if got.Data[i] != 0x11 {
			t.Fatalf("decoded byte %d = %02x, want 11", i, got.Data[i])
		}
	}
	if got.Geometry.SectorsPerTrack != 1 || got.Geometry.Sides != 1 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected MSA geometry: %+v", got.Geometry)
	}
}

func TestLoadDiskImagePreservesDoubleSidedMSAGeometry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "double-sided.msa")
	msa := []byte{
		0x0E, 0x0F, // magic
		0x00, 0x01, // sectors per track
		0x00, 0x01, // two sides
		0x00, 0x00, // start track
		0x00, 0x00, // end track
		0x00, 0x04, // side 0 compressed block length
		0xE5, 0x11, 0x02, 0x00,
		0x00, 0x04, // side 1 compressed block length
		0xE5, 0x22, 0x02, 0x00,
	}
	if err := os.WriteFile(path, msa, 0o644); err != nil {
		t.Fatalf("write MSA disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load double-sided MSA: %v", err)
	}
	if got.Geometry.SectorsPerTrack != 1 || got.Geometry.Sides != 2 || got.Geometry.Tracks != 1 {
		t.Fatalf("unexpected double-sided MSA geometry: %+v", got.Geometry)
	}
	if len(got.Data) != 1024 {
		t.Fatalf("decoded double-sided MSA length = %d, want 1024", len(got.Data))
	}
	if got.Data[0] != 0x11 || got.Data[512] != 0x22 {
		t.Fatalf("unexpected double-sided data markers: %02x %02x", got.Data[0], got.Data[512])
	}
}

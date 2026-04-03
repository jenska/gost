package emulator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDiskImageReturnsRawSTBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.st")
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("write raw disk image: %v", err)
	}

	got, err := LoadDiskImage(path)
	if err != nil {
		t.Fatalf("load raw disk image: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("unexpected raw disk image bytes: got %x want %x", got, want)
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
	if len(got) != 512 {
		t.Fatalf("decoded MSA length = %d, want 512", len(got))
	}
	for i := 0; i < len(got); i++ {
		if got[i] != 0x11 {
			t.Fatalf("decoded byte %d = %02x, want 11", i, got[i])
		}
	}
}

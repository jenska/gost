package emulator

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureHardDiskImageFileCreatesMissingImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images", "disk.hd")
	initial := bytes.Repeat([]byte{0xA5}, 4*hardDiskSectorSize)

	image, created, err := EnsureHardDiskImageFile(path, initial)
	if err != nil {
		t.Fatalf("ensure hard disk image file: %v", err)
	}
	if !created {
		t.Fatalf("expected missing image to be created")
	}
	if !bytes.Equal(image, initial) {
		t.Fatalf("created image bytes do not match initial image")
	}

	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created image file: %v", err)
	}
	if !bytes.Equal(stored, initial) {
		t.Fatalf("stored image bytes do not match initial image")
	}
}

func TestEnsureHardDiskImageFileLoadsExistingImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.hd")
	existing := bytes.Repeat([]byte{0x5A}, 2*hardDiskSectorSize)
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatalf("seed existing image file: %v", err)
	}
	initial := bytes.Repeat([]byte{0xA5}, 4*hardDiskSectorSize)

	image, created, err := EnsureHardDiskImageFile(path, initial)
	if err != nil {
		t.Fatalf("ensure hard disk image file: %v", err)
	}
	if created {
		t.Fatalf("expected existing image to be loaded, not created")
	}
	if !bytes.Equal(image, existing) {
		t.Fatalf("loaded image bytes do not match existing file")
	}
}

func TestEnsureHardDiskImageFileFailsWithoutInitialImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.hd")
	if _, _, err := EnsureHardDiskImageFile(path, nil); err == nil {
		t.Fatalf("expected error when image file is missing and no initial image is provided")
	}
}

func TestSaveHardDiskImageFileWritesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.hd")
	image := bytes.Repeat([]byte{0x11, 0x22, 0x33, 0x44}, hardDiskSectorSize)

	if err := SaveHardDiskImageFile(path, image); err != nil {
		t.Fatalf("save hard disk image file: %v", err)
	}

	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved image file: %v", err)
	}
	if !bytes.Equal(stored, image) {
		t.Fatalf("saved image bytes do not match")
	}
}

func TestSaveHardDiskImageFileRejectsEmptyImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.hd")
	if err := SaveHardDiskImageFile(path, nil); err == nil {
		t.Fatalf("expected empty image to be rejected")
	}
}

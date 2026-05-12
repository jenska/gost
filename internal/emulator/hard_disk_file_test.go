package emulator

import (
	"bytes"
	"encoding/binary"
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

func TestEnsureHardDiskImageFileLoadsExistingHDIImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.hdi")
	existing := bytes.Repeat([]byte{0x5A}, 63*16*hardDiskSectorSize)
	if err := os.WriteFile(path, encodeAnex86HDI(existing), 0o644); err != nil {
		t.Fatalf("seed existing HDI file: %v", err)
	}
	initial := bytes.Repeat([]byte{0xA5}, 4*hardDiskSectorSize)

	image, created, err := EnsureHardDiskImageFile(path, initial)
	if err != nil {
		t.Fatalf("ensure HDI image file: %v", err)
	}
	if created {
		t.Fatalf("expected existing HDI image to be loaded, not created")
	}
	if !bytes.Equal(image, existing) {
		t.Fatalf("loaded HDI payload does not match existing file")
	}
}

func TestEnsureHardDiskImageFileCreatesMissingHDIImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images", "disk.hdi")
	initial := bytes.Repeat([]byte{0xA5}, 63*16*hardDiskSectorSize)

	image, created, err := EnsureHardDiskImageFile(path, initial)
	if err != nil {
		t.Fatalf("ensure HDI image file: %v", err)
	}
	if !created {
		t.Fatalf("expected missing HDI image to be created")
	}
	if !bytes.Equal(image, initial) {
		t.Fatalf("created HDI payload does not match initial image")
	}

	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created HDI file: %v", err)
	}
	payload, ok, err := decodeAnex86HDI(stored)
	if err != nil {
		t.Fatalf("decode created HDI file: %v", err)
	}
	if !ok {
		t.Fatalf("expected created .hdi file to use an Anex86 HDI header")
	}
	if !bytes.Equal(payload, initial) {
		t.Fatalf("stored HDI payload does not match initial image")
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

func TestSaveHardDiskImageFilePreservesExistingHDIContainer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.hdi")
	initial := bytes.Repeat([]byte{0x22}, 63*16*hardDiskSectorSize)
	header := encodeAnex86HDI(initial)[:anex86HDIHeaderSize]
	binary.LittleEndian.PutUint32(header[0x08:0x0C], 4096)
	header = append(header, bytes.Repeat([]byte{0x7E}, 4096-anex86HDIHeaderSize)...)
	storedInitial := append(header, initial...)
	if err := os.WriteFile(path, storedInitial, 0o644); err != nil {
		t.Fatalf("seed HDI file: %v", err)
	}
	updated := bytes.Repeat([]byte{0x33}, len(initial))

	if err := SaveHardDiskImageFile(path, updated); err != nil {
		t.Fatalf("save HDI image file: %v", err)
	}

	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved HDI file: %v", err)
	}
	payload, ok, err := decodeAnex86HDI(stored)
	if err != nil {
		t.Fatalf("decode saved HDI file: %v", err)
	}
	if !ok {
		t.Fatalf("expected saved .hdi file to preserve HDI header")
	}
	if len(stored) != 4096+len(updated) {
		t.Fatalf("saved HDI length = %d, want %d", len(stored), 4096+len(updated))
	}
	if stored[anex86HDIHeaderSize] != 0x7E {
		t.Fatalf("expected HDI comment/header padding to be preserved, got %02x", stored[anex86HDIHeaderSize])
	}
	if !bytes.Equal(payload, updated) {
		t.Fatalf("saved HDI payload does not match updated image")
	}
}

func TestSaveHardDiskImageFileRejectsEmptyImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.hd")
	if err := SaveHardDiskImageFile(path, nil); err == nil {
		t.Fatalf("expected empty image to be rejected")
	}
}

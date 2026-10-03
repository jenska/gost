package emulator

import (
	"os"
	"testing"
)

func TestLoadROMImagePadsOddLengthImages(t *testing.T) {
	path := writeTempROM(t, []byte{0x12, 0x34, 0x56})

	image, err := loadROMImage(path)
	if err != nil {
		t.Fatalf("load ROM: %v", err)
	}
	if got, want := len(image), 4; got != want {
		t.Fatalf("unexpected ROM length: got %d want %d", got, want)
	}
	if image[3] != 0xFF {
		t.Fatalf("unexpected padded byte: got %02x want ff", image[3])
	}
}

func TestLoadROMImageKeepsEvenLengthImages(t *testing.T) {
	path := writeTempROM(t, []byte{0x12, 0x34})

	image, err := loadROMImage(path)
	if err != nil {
		t.Fatalf("load ROM: %v", err)
	}
	if got, want := len(image), 2; got != want {
		t.Fatalf("unexpected ROM length: got %d want %d", got, want)
	}
}

func TestLoadROMImageRejectsEmptyImage(t *testing.T) {
	path := writeTempROM(t, nil)

	if _, err := loadROMImage(path); err == nil {
		t.Fatal("expected an error loading an empty ROM image")
	}
}

func TestLoadROMImageRejectsOversizedImage(t *testing.T) {
	path := writeTempROM(t, make([]byte, romImageMaxBytes+2))

	if _, err := loadROMImage(path); err == nil {
		t.Fatalf("expected an error loading a ROM image larger than %d bytes", romImageMaxBytes)
	}
}

func writeTempROM(t *testing.T, data []byte) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "rom-*.img")
	if err != nil {
		t.Fatalf("create temp ROM: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatalf("write temp ROM: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp ROM: %v", err)
	}
	return file.Name()
}

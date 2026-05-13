package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestCartridgeROMReadsImageAtCartridgeWindow(t *testing.T) {
	cart, err := NewCartridgeROM([]byte{0x12, 0x34, 0x56, 0x78})
	if err != nil {
		t.Fatalf("create cartridge: %v", err)
	}

	value, err := cart.Read(cpu.Long, cartridgeROMBase)
	if err != nil {
		t.Fatalf("read cartridge long: %v", err)
	}
	if value != 0x12345678 {
		t.Fatalf("unexpected cartridge long: got %08x want 12345678", value)
	}
}

func TestCartridgeROMUnpopulatedBytesReadAsFF(t *testing.T) {
	cart, err := NewCartridgeROM([]byte{0x12})
	if err != nil {
		t.Fatalf("create cartridge: %v", err)
	}

	value, err := cart.Read(cpu.Word, cartridgeROMBase)
	if err != nil {
		t.Fatalf("read padded cartridge word: %v", err)
	}
	if value != 0x12FF {
		t.Fatalf("unexpected padded cartridge word: got %04x want 12ff", value)
	}
}

func TestCartridgeROMRejectsOversizedImages(t *testing.T) {
	_, err := NewCartridgeROM(make([]byte, cartridgeROMSize+1))
	if err == nil {
		t.Fatalf("expected oversized cartridge image to fail")
	}
}

func TestCartridgeROMIgnoresWrites(t *testing.T) {
	cart, err := NewCartridgeROM([]byte{0x12})
	if err != nil {
		t.Fatalf("create cartridge: %v", err)
	}
	if err := cart.Write(cpu.Byte, cartridgeROMBase, 0x99); err != nil {
		t.Fatalf("write cartridge: %v", err)
	}
	value, err := cart.Read(cpu.Byte, cartridgeROMBase)
	if err != nil {
		t.Fatalf("read cartridge after write: %v", err)
	}
	if value != 0x12 {
		t.Fatalf("cartridge write changed data: got %02x want 12", value)
	}
}

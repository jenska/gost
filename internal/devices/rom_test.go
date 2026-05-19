package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestOverlayROMSynthesizesResetSPForAtariTOSImages(t *testing.T) {
	rom := NewROM([]byte{
		0x60, 0x2e, 0x01, 0x02,
		0x00, 0xfc, 0x00, 0x30,
	}, 0xFC0000)
	overlay := NewOverlayROM(rom, NewRAM(0, 2*1024*1024))

	value, err := overlay.Read(cpu.Long, 0)
	if err != nil {
		t.Fatalf("read synthetic reset SP: %v", err)
	}
	if value != 0x00200000 {
		t.Fatalf("unexpected reset SP: got %08x want 00200000", value)
	}

	value, err = overlay.Read(cpu.Long, 4)
	if err != nil {
		t.Fatalf("read reset PC: %v", err)
	}
	if value != 0x00FC0030 {
		t.Fatalf("unexpected reset PC: got %08x want 00fc0030", value)
	}
}

func TestOverlayROMKeepsPlausibleResetSPFromVectorROM(t *testing.T) {
	rom := NewROM([]byte{
		0x00, 0x08, 0x00, 0x00,
		0x00, 0xfc, 0x00, 0x08,
	}, 0xFC0000)
	overlay := NewOverlayROM(rom, NewRAM(0, 1024*1024))

	value, err := overlay.Read(cpu.Long, 0)
	if err != nil {
		t.Fatalf("read reset SP: %v", err)
	}
	if value != 0x00080000 {
		t.Fatalf("unexpected reset SP: got %08x want 00080000", value)
	}
}

func TestOverlayROMSyntheticResetSPFragments(t *testing.T) {
	rom := NewROM([]byte{
		0x60, 0x2e, 0x01, 0x02,
		0x00, 0xfc, 0x00, 0x30,
	}, 0xFC0000)
	overlay := NewOverlayROM(rom, NewRAM(0, 2*1024*1024))

	tests := []struct {
		size    cpu.Size
		address uint32
		want    uint32
	}{
		{size: cpu.Word, address: 0, want: 0x0020},
		{size: cpu.Word, address: 2, want: 0x0000},
		{size: cpu.Byte, address: 1, want: 0x20},
	}

	for _, tt := range tests {
		got, err := overlay.Read(tt.size, tt.address)
		if err != nil {
			t.Fatalf("read size %d address %d: %v", tt.size, tt.address, err)
		}
		if got != tt.want {
			t.Fatalf("read size %d address %d: got %04x want %04x", tt.size, tt.address, got, tt.want)
		}
	}
}

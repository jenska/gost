package devices

import (
	"testing"

	"github.com/jenska/m68kemu"
)

func TestScratchRegionDefaultsToZero(t *testing.T) {
	region := NewScratchRegion(0xFF8006, 0xFF8008)

	value, err := region.Read(m68kemu.Word, 0xFF8006)
	if err != nil {
		t.Fatalf("read scratch region: %v", err)
	}
	if got := uint16(value); got != 0 {
		t.Fatalf("unexpected default value: got %04x want 0000", got)
	}
}

func TestScratchRegionByteWritesUpdateHighAndLowBytes(t *testing.T) {
	region := NewScratchRegion(0xFF8006, 0xFF8008)

	if err := region.Write(m68kemu.Byte, 0xFF8006, 0x12); err != nil {
		t.Fatalf("write high byte: %v", err)
	}
	if err := region.Write(m68kemu.Byte, 0xFF8007, 0x34); err != nil {
		t.Fatalf("write low byte: %v", err)
	}

	value, err := region.Read(m68kemu.Word, 0xFF8006)
	if err != nil {
		t.Fatalf("read word: %v", err)
	}
	if got := uint16(value); got != 0x1234 {
		t.Fatalf("unexpected word: got %04x want 1234", got)
	}

	region.Reset()
	if value, _ := region.Read(m68kemu.Word, 0xFF8006); value != 0 {
		t.Fatalf("Reset did not clear the register: %04x", value)
	}
}

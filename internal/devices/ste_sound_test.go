package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestSTESoundFaultsOnAbsentDMAWindow(t *testing.T) {
	sound := NewSTESound()

	if _, err := sound.Read(cpu.Byte, 0xFF8901); err == nil {
		t.Fatalf("expected byte read to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}

	if err := sound.Write(cpu.Word, 0xFF8900, 0x1234); err == nil {
		t.Fatalf("expected word write to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}
}

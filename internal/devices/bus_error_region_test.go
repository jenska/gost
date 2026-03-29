package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestBusErrorRegionFaultsInConfiguredRange(t *testing.T) {
	region := NewBusErrorRegion(AddressRange{Start: 0xFFFE00, End: 0xFFFE10})

	if !region.Contains(0xFFFE00) || !region.Contains(0xFFFE0F) || region.Contains(0xFFFE10) {
		t.Fatalf("unexpected range match behavior")
	}

	if _, err := region.Read(cpu.Byte, 0xFFFE00); err == nil {
		t.Fatalf("expected read to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}

	if err := region.Write(cpu.Word, 0xFFFE00, 0x1234); err == nil {
		t.Fatalf("expected write to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}
}

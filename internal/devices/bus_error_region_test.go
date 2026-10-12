package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestBusErrorRegionFaultsInConfiguredRange(t *testing.T) {
	region := NewBusErrorRegion(AddressRange{Start: 0xFFFE00, End: 0xFFFE10})

	if start, end := region.AddressRange(); start != 0xFFFE00 || end != 0xFFFE0F {
		t.Fatalf("unexpected range %06x-%06x", start, end)
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

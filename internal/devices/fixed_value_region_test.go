package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestFixedValueRegionReturnsConfiguredValue(t *testing.T) {
	region := NewFixedValueRegion(0xFFFFFFFF, AddressRange{Start: 0xFA0000, End: 0xFA0010})

	if !region.Contains(0xFA0004) {
		t.Fatalf("expected address to be covered by fixed value region")
	}

	value, err := region.Read(cpu.Byte, 0xFA0000)
	if err != nil {
		t.Fatalf("read byte: %v", err)
	}
	if value != 0xFF {
		t.Fatalf("unexpected byte value: got %02x want ff", value)
	}

	value, err = region.Read(cpu.Word, 0xFA0000)
	if err != nil {
		t.Fatalf("read word: %v", err)
	}
	if value != 0xFFFF {
		t.Fatalf("unexpected word value: got %04x want ffff", value)
	}

	value, err = region.Read(cpu.Long, 0xFA0000)
	if err != nil {
		t.Fatalf("read long: %v", err)
	}
	if value != 0xFFFFFFFF {
		t.Fatalf("unexpected long value: got %08x want ffffffff", value)
	}
}

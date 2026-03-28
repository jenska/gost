package emulator

import (
	"testing"

	"github.com/jenska/m68kemu"
)

func TestIsBootTraceAddress(t *testing.T) {
	tests := []struct {
		name    string
		address uint32
		want    bool
	}{
		{name: "watched low memory", address: 0x000010, want: true},
		{name: "watched io register", address: 0xFF8001, want: true},
		{name: "masked high bits still match 24-bit bus address", address: 0x12FF8201, want: true},
		{name: "unwatched address", address: 0x00E003CE, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBootTraceAddress(tt.address); got != tt.want {
				t.Fatalf("isBootTraceAddress(%08x) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestTraceValueString(t *testing.T) {
	tests := []struct {
		name  string
		size  m68kemu.Size
		value uint32
		want  string
	}{
		{name: "byte", size: m68kemu.Byte, value: 0x1234, want: "34"},
		{name: "word", size: m68kemu.Word, value: 0x1234, want: "1234"},
		{name: "long", size: m68kemu.Long, value: 0x12345678, want: "12345678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := traceValueString(tt.size, tt.value); got != tt.want {
				t.Fatalf("traceValueString(%d, %08x) = %q, want %q", tt.size, tt.value, got, tt.want)
			}
		})
	}
}

func TestMachineTracePCInRange(t *testing.T) {
	machine := &Machine{cfg: Config{TraceStart: 0x00E16780, TraceEnd: 0x00E1679A}}

	tests := []struct {
		name string
		pc   uint32
		want bool
	}{
		{name: "inside configured range", pc: 0x00E16794, want: true},
		{name: "outside configured range", pc: 0x00E02874, want: false},
		{name: "high bits masked on 24-bit bus", pc: 0x12E16794, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := machine.tracePCInRange(tt.pc); got != tt.want {
				t.Fatalf("tracePCInRange(%08x) = %v, want %v", tt.pc, got, tt.want)
			}
		})
	}
}

func TestMachineTracePCInRangeDefaults(t *testing.T) {
	machine := &Machine{}

	if !machine.tracePCInRange(0x00E003CE) {
		t.Fatalf("expected default trace range to include early boot PC")
	}
	if machine.tracePCInRange(0x00E16794) {
		t.Fatalf("expected default trace range to exclude late boot PC")
	}
}

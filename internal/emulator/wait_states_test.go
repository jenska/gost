package emulator

import (
	"testing"

	"github.com/jenska/gost/internal/config"
)

// readCycles returns the cycles one `move.w <address>.l,d0` takes, executed
// from ROM with interrupts masked.
func readCycles(t *testing.T, address uint32) uint64 {
	t.Helper()
	code := []byte{
		0x46, 0xFC, 0x27, 0x00, // move.w #$2700,sr
		0x30, 0x39, byte(address >> 24), byte(address >> 16), byte(address >> 8), byte(address), // move.w <address>.l,d0
		0x60, 0xFE, // bra.s *
	}
	m, err := NewMachine(config.DefaultConfig(), loopROM(code))
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	read := uint32(defaultROMHighAlias + 8 + 4)
	for m.Registers().PC != read {
		if err := m.cpu.Step(); err != nil {
			t.Fatalf("step to read: %v", err)
		}
	}
	start := m.Cycles()
	if err := m.cpu.Step(); err != nil {
		t.Fatalf("read $%06x: %v", address, err)
	}
	return m.Cycles() - start
}

// TestSTFWaitStates pins the bus timing of a real STF (as measured for
// Hatari): RAM and ROM run at the plain 68000 bus cycle, while I/O devices add
// a fixed delay to each access. Every instruction is then rounded up to a
// multiple of 4 cycles, so the ACIA's 6-cycle delay turns 16+6 into 24.
func TestSTFWaitStates(t *testing.T) {
	// move.w abs.l,Dn is 16 cycles on a 68000 with a zero-wait bus.
	ram := readCycles(t, 0x000400)
	if ram != 16 {
		t.Fatalf("RAM read = %d cycles, want the plain 68000 timing of 16", ram)
	}

	for _, tc := range []struct {
		name    string
		address uint32
		extra   uint64
	}{
		{"ROM", defaultROMHighAlias, 0},
		{"shifter palette", 0xFF8240, 0},
		{"DMA/FDC", 0xFF8604, 4},
		{"YM2149", 0xFF8800, 4},
		{"MFP", 0xFFFA00, 4},
		{"ACIA", 0xFFFC00, 8},
	} {
		if got := readCycles(t, tc.address) - ram; got != tc.extra {
			t.Errorf("%s ($%06x): %d wait cycles, want %d", tc.name, tc.address, got, tc.extra)
		}
	}
}

// TestSTFRoundsInstructionsToBusSlots checks that an instruction whose 68000
// time is not a multiple of 4 occupies the next 4-cycle bus slot.
func TestSTFRoundsInstructionsToBusSlots(t *testing.T) {
	code := []byte{
		0x46, 0xFC, 0x27, 0x00, // move.w #$2700,sr
		0xC1, 0x41, // exg d0,d1: 6 cycles on a 68000
		0x60, 0xFE, // bra.s *
	}
	m, err := NewMachine(config.DefaultConfig(), loopROM(code))
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	exg := uint32(defaultROMHighAlias + 8 + 4)
	for m.Registers().PC != exg {
		if err := m.cpu.Step(); err != nil {
			t.Fatalf("step to exg: %v", err)
		}
	}
	start := m.Cycles()
	if err := m.cpu.Step(); err != nil {
		t.Fatalf("exg: %v", err)
	}
	if got := m.Cycles() - start; got != 8 {
		t.Fatalf("exg d0,d1 = %d cycles, want 8 (6 rounded up to the bus slot)", got)
	}
}

package emulator

import (
	"testing"

	"github.com/jenska/gost/internal/assets"
	cpu "github.com/jenska/m68kemu"
)

func TestRomAliasIsIsolated(t *testing.T) {
	const k256 = 256 * 1024
	const k192 = 192 * 1024

	cases := []struct {
		name   string
		base   uint32
		length uint32
		want   bool
	}{
		{"emutos 256K at E00000", 0xE00000, k256, true},
		{"tos 1.x 192K at FC0000", 0xFC0000, k192, true},
		{"256K at FC0000 reaches I/O", 0xFC0000, k256, false},
		{"reaches cartridge probe", 0xF90000, k256, false},
		{"below ROM space", 0x000000, k256, false},
		{"zero length", 0xE00000, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := romAliasIsIsolated(tc.base, tc.length); got != tc.want {
				t.Fatalf("romAliasIsIsolated(%06x, %d) = %v, want %v", tc.base, tc.length, got, tc.want)
			}
		})
	}
}

func hasFastRegionAt(m *Machine, address uint32) bool {
	for _, r := range m.fastRegions {
		if address >= r.Base && uint64(address) < uint64(r.Base)+uint64(len(r.Mem)) {
			return true
		}
	}
	return false
}

// TestFastRegionsCoverRomEntry verifies the reset PC lands in a fast region.
func TestFastRegionsCoverRomEntry(t *testing.T) {
	m := mustMachine(t, loopROM(nil))
	if err := m.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if !hasFastRegionAt(m, m.Registers().PC) {
		t.Fatalf("no fast region covers reset PC %06x; regions=%+v", m.Registers().PC, m.fastRegions)
	}
	for _, r := range m.fastRegions {
		if !r.ReadOnly {
			t.Fatalf("fast region at %06x is not read-only", r.Base)
		}
	}
}

// TestFastRegionCycleCountMatchesBus runs a short ROM-resident program that
// mixes word and long fetches with long RAM load/store, once with the ROM fast
// region and once purely on the bus. The total cycle count must be identical.
func TestFastRegionCycleCountMatchesBus(t *testing.T) {
	// FC0008: MOVE.L  $00001200,D0   2039 0000 1200
	// FC000E: MOVE.L  D0,$00001300   23C0 0000 1300
	// FC0014: MOVEQ   #1,D1          7201
	// FC0016: ADD.L   D1,D0          D081
	// FC0018: BRA *                  60FE
	prog := []byte{
		0x20, 0x39, 0x00, 0x00, 0x12, 0x00,
		0x23, 0xC0, 0x00, 0x00, 0x13, 0x00,
		0x72, 0x01,
		0xD0, 0x81,
		0x60, 0xFE,
	}

	run := func(disable bool) uint64 {
		m, err := NewMachine(DefaultConfig(), loopROM(prog))
		if err != nil {
			t.Fatalf("NewMachine: %v", err)
		}
		m.disableFastMemory = disable
		if err := m.Reset(); err != nil {
			t.Fatalf("Reset: %v", err)
		}
		if !disable && !hasFastRegionAt(m, m.Registers().PC) {
			t.Fatalf("expected a ROM fast region over the PC")
		}
		if _, err := m.RunUntil(cpu.RunUntilOptions{MaxInstructions: 8}); err != nil {
			t.Fatalf("RunUntil: %v", err)
		}
		return m.Cycles()
	}

	if fast, bus := run(false), run(true); fast != bus {
		t.Fatalf("cycle count differs: fast=%d bus=%d", fast, bus)
	}
}

// TestFastRegionBootParity is the regression guard for the fast-memory path: a
// real EmuTOS boot must land on exactly the same cycle count, PC and SR whether
// the ROM fast region is used or every fetch goes through the bus.
func TestFastRegionBootParity(t *testing.T) {
	run := func(disable bool) (uint64, uint32, uint16) {
		m, err := NewMachine(DefaultConfig(), assets.DefaultROM())
		if err != nil {
			t.Fatalf("NewMachine: %v", err)
		}
		m.disableFastMemory = disable
		if err := m.Reset(); err != nil {
			t.Fatalf("Reset: %v", err)
		}
		for frame := 0; frame < 45; frame++ {
			if _, err := m.StepFrame(); err != nil {
				t.Fatalf("StepFrame %d: %v", frame, err)
			}
		}
		r := m.Registers()
		return m.Cycles(), r.PC, r.SR
	}

	bc, bpc, bsr := run(true)
	fc, fpc, fsr := run(false)
	if bc != fc || bpc != fpc || bsr != fsr {
		t.Fatalf("boot diverged: bus=(cyc %d pc %06x sr %04x) fast=(cyc %d pc %06x sr %04x)",
			bc, bpc, bsr, fc, fpc, fsr)
	}
}

package emulator

import (
	"testing"

	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/devices"
)

// TestMachineShifterSeesRegisterWritesAtTheirCPUCycle checks that a palette
// write lands at the pixel the beam is on when the CPU makes it, not at the
// end of the CPU quantum the write happens in.
func TestMachineShifterSeesRegisterWritesAtTheirCPUCycle(t *testing.T) {
	code := []byte{
		0x46, 0xFC, 0x27, 0x00, // move.w #$2700,sr
		0x30, 0x3C, 0x05, 0x0F, // move.w #1295,d0
		0x51, 0xC8, 0xFF, 0xFE, // dbra d0,*
		0x33, 0xFC, 0x07, 0x77, 0x00, 0xFF, 0x82, 0x40, // move.w #$0777,$ff8240
		0x60, 0xFE, // bra.s *
	}
	afterWrite := uint32(defaultROMHighAlias + 8 + 20)
	cfg := config.DefaultConfig()
	cfg.ColorMonitor = true

	// Find the write's frame cycle by stepping the CPU alone.
	probe, err := NewMachine(cfg, loopROM(code))
	if err != nil {
		t.Fatalf("create probe machine: %v", err)
	}
	start := probe.Cycles()
	for probe.Registers().PC != afterWrite {
		if err := probe.cpu.Step(); err != nil {
			t.Fatalf("step probe: %v", err)
		}
	}
	writeCycle := probe.Cycles() - start

	m, err := NewMachine(cfg, loopROM(code))
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if _, err := m.StepFrame(); err != nil {
		t.Fatalf("step frame: %v", err)
	}
	// Screen memory is blank, so every pixel shows colour 0: black up to the
	// write, white from it on.
	timing := devices.VideoTimingFor(cfg)
	line := writeCycle * timing.Scanlines / timing.FrameCycles
	start, end := timing.DisplayWindow(line)
	if line >= 200 || writeCycle <= start || writeCycle >= end {
		t.Fatalf("write at frame cycle %d is not inside a displayed line; adjust the delay", writeCycle)
	}
	x := int((writeCycle - start) * 320 / (end - start))
	fb := m.shifter.FrameBuffer()
	pixel := func(x, y int) byte { return fb[(y*320+x)*4] }
	if pixel(x-1, int(line)) != 0 || pixel(x, int(line)) != 0xFF {
		t.Fatalf("line %d: pixels %d/%d = %d/%d, want the change from black to white at x=%d (frame cycle %d)",
			line, x-1, x, pixel(x-1, int(line)), pixel(x, int(line)), x, writeCycle)
	}
	if pixel(319, int(line)-1) != 0 || pixel(0, int(line)+1) != 0xFF {
		t.Fatalf("lines around the write: line %d ends %d, line %d starts %d; want black then white",
			line-1, pixel(319, int(line)-1), line+1, pixel(0, int(line)+1))
	}
}

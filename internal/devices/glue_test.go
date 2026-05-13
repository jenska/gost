package devices

import (
	"testing"

	"github.com/jenska/gost/internal/config"
	"github.com/jenska/m68kemu"
)

func TestGLUEConfigRegisterDefaultsToZero(t *testing.T) {
	glue := NewGLUE()

	value, err := glue.Read(m68kemu.Word, glueBase)
	if err != nil {
		t.Fatalf("read glue register: %v", err)
	}
	if got := uint16(value); got != 0 {
		t.Fatalf("unexpected default glue value: got %04x want 0000", got)
	}
}

func TestGLUEByteWritesUpdateHighAndLowBytes(t *testing.T) {
	glue := NewGLUE()

	if err := glue.Write(m68kemu.Byte, glueBase, 0x12); err != nil {
		t.Fatalf("write glue high byte: %v", err)
	}
	if err := glue.Write(m68kemu.Byte, glueBase+1, 0x34); err != nil {
		t.Fatalf("write glue low byte: %v", err)
	}

	value, err := glue.Read(m68kemu.Word, glueBase)
	if err != nil {
		t.Fatalf("read glue word: %v", err)
	}
	if got := uint16(value); got != 0x1234 {
		t.Fatalf("unexpected glue word: got %04x want 1234", got)
	}
}

func TestGLUEQueuesHBLAtScanlineBoundary(t *testing.T) {
	cfg := &config.Config{ClockHz: 8_000_000, FrameHz: 50}
	glue := NewGLUE(cfg)

	cycles, ok := glue.NextEventCycles()
	if !ok {
		t.Fatalf("expected GLUE timing event")
	}
	if cycles != 511 {
		t.Fatalf("unexpected first PAL scanline cycles: got %d want 511", cycles)
	}

	glue.Advance(cycles)
	irqs := glue.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected one HBL interrupt, got %d", len(irqs))
	}
	if irqs[0].Level != 2 || irqs[0].Vector != nil {
		t.Fatalf("unexpected HBL interrupt: %+v", irqs[0])
	}
}

func TestGLUEQueuesVBLAtFrameBoundary(t *testing.T) {
	cfg := &config.Config{ClockHz: 8_000_000, FrameHz: 50}
	glue := NewGLUE(cfg)

	glue.Advance(cfg.FrameCycles())
	irqs := glue.DrainInterrupts()
	if len(irqs) == 0 {
		t.Fatalf("expected GLUE frame interrupts")
	}
	last := irqs[len(irqs)-1]
	if last.Level != 4 || last.Vector != nil {
		t.Fatalf("expected final frame interrupt to be VBL autovector, got %+v", last)
	}
}

func TestGLUENTSCUsesShorterScanlineTiming(t *testing.T) {
	cfg := &config.Config{ClockHz: 8_000_000, FrameHz: 60}
	glue := NewGLUE(cfg)

	cycles, ok := glue.NextEventCycles()
	if !ok {
		t.Fatalf("expected GLUE timing event")
	}
	if cycles != 506 {
		t.Fatalf("unexpected first NTSC scanline cycles: got %d want 506", cycles)
	}
}

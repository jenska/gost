package devices

import (
	"testing"

	"github.com/jenska/gost/internal/config"
)

func TestGLUEAssertsHBLLineAtScanlineBoundary(t *testing.T) {
	cfg := &config.Config{ClockHz: 8_000_000, FrameHz: 50}
	glue := NewGLUE(cfg)

	cycles, ok := glue.NextEventCycles()
	if !ok {
		t.Fatalf("expected GLUE timing event")
	}
	if cycles != 511 {
		t.Fatalf("unexpected first PAL scanline cycles: got %d want 511", cycles)
	}

	if level, _ := glue.PendingIRQ(); level != 0 {
		t.Fatalf("GLUE line asserted before any scanline edge: level %d", level)
	}

	glue.Advance(cycles)
	level, vector := glue.PendingIRQ()
	if level != 2 || vector != AutoVector {
		t.Fatalf("expected HBL autovector line, got level %d vector %d", level, vector)
	}

	glue.AckIRQ(2)
	if level, _ := glue.PendingIRQ(); level != 0 {
		t.Fatalf("GLUE line still asserted after AckIRQ: level %d", level)
	}
}

func TestGLUEAssertsVBLLineAtFrameBoundary(t *testing.T) {
	cfg := &config.Config{ClockHz: 8_000_000, FrameHz: 50}
	glue := NewGLUE(cfg)

	glue.Advance(cfg.FrameCycles())
	if level, vector := glue.PendingIRQ(); level != 4 || vector != AutoVector {
		t.Fatalf("expected VBL autovector line after a full frame, got level %d vector %d", level, vector)
	}
}

// TestGLUEVBLLineOutranksLaterHBLEdges guards the priority guard in Advance: an
// unacknowledged VBL (level 4) must not be downgraded to an HBL (level 2) by the
// scanline edges that follow it in the next frame.
func TestGLUEVBLLineOutranksLaterHBLEdges(t *testing.T) {
	cfg := &config.Config{ClockHz: 8_000_000, FrameHz: 50}
	glue := NewGLUE(cfg)

	glue.Advance(cfg.FrameCycles())
	if level, _ := glue.PendingIRQ(); level != 4 {
		t.Fatalf("expected VBL line, got level %d", level)
	}

	// Several more scanlines pass without the CPU taking the VBL.
	glue.Advance(cfg.FrameCycles() / 100)
	if level, _ := glue.PendingIRQ(); level != 4 {
		t.Fatalf("VBL line was lowered by later HBL edges: level %d", level)
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

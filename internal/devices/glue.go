package devices

import (
	"github.com/jenska/gost/internal/config"
	cpu "github.com/jenska/m68kemu"
)

// GLUE models the part of the ST's glue logic the emulator core needs directly:
// the horizontal- and vertical-blank autovector timing. It is not memory-mapped
// (the system-control probe address it used to answer is a plain ScratchRegion
// now); the machine drives it through advanceDevices and the IRQ line.
type GLUE struct {
	frameCycles  uint64
	scanlines    uint64
	cycleInFrame uint64
	nextLine     uint64
	// lineLevel is the autovector interrupt level GLUE currently drives onto the
	// CPU: 0 (idle), 2 (HBL) or 4 (VBL). Each scanline/frame edge (re)asserts it;
	// AckIRQ lowers it once the CPU has taken it. A pulse the CPU never gets to
	// (masked the whole time) is simply overwritten by the next edge, so it is
	// taken at most about one scanline late rather than latched indefinitely.
	lineLevel uint8
}

func NewGLUE(cfg ...*config.Config) *GLUE {
	g := &GLUE{}
	if len(cfg) != 0 && cfg[0] != nil {
		g.configureTiming(cfg[0])
	}
	g.Reset()
	return g
}

func (g *GLUE) Reset() {
	g.cycleInFrame = 0
	g.nextLine = 1
	g.lineLevel = 0
}

func (g *GLUE) Advance(cycles uint64) {
	if g.frameCycles == 0 || g.scanlines == 0 {
		return
	}

	g.cycleInFrame += cycles
	for g.cycleInFrame >= g.nextLineCycle() {
		if g.nextLine >= g.scanlines {
			g.lineLevel = 4
			g.cycleInFrame -= g.frameCycles
			g.nextLine = 1
			continue
		}
		if g.lineLevel < 2 {
			g.lineLevel = 2
		}
		g.nextLine++
	}
}

// PendingIRQ reports the autovector line GLUE is currently driving (HBL level 2
// or VBL level 4), or level 0 when idle.
func (g *GLUE) PendingIRQ() (level, vector uint8) {
	return g.lineLevel, cpu.AutoVector
}

// AckIRQ lowers the line once the CPU has accepted the pulse GLUE raised.
func (g *GLUE) AckIRQ(level uint8) {
	if g.lineLevel == level {
		g.lineLevel = 0
	}
}

func (g *GLUE) NextEventCycles() (uint64, bool) {
	if g.frameCycles == 0 || g.scanlines == 0 {
		return 0, false
	}
	next := g.nextLineCycle()
	if g.cycleInFrame >= next {
		return 1, true
	}
	return next - g.cycleInFrame, true
}

func (g *GLUE) configureTiming(cfg *config.Config) {
	if cfg.FrameCycles() == 0 {
		return // no usable clock/refresh; GLUE stays idle
	}
	timing := cfg.Video()
	g.frameCycles = timing.FrameCycles
	g.scanlines = timing.Scanlines
}

func (g *GLUE) nextLineCycle() uint64 {
	if g.nextLine == 0 {
		g.nextLine = 1
	}
	cycle := g.nextLine * g.frameCycles / g.scanlines
	if cycle == 0 {
		return 1
	}
	return cycle
}

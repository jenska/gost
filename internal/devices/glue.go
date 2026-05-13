package devices

import (
	"github.com/jenska/gost/internal/config"
	cpu "github.com/jenska/m68kemu"
)

const (
	glueBase          = 0xFF8006
	glueSize          = 2
	gluePALScanlines  = 313
	glueNTSCScanlines = 263
)

// GLUE models the ST's glue logic that is visible to the emulator core:
// address-decoded system-control probes plus horizontal/vertical blank
// autovector timing. Most GLUE behavior is expressed by other devices through
// the bus map, MFP lines, and shifter registers.
type GLUE struct {
	configRegister uint16
	frameCycles    uint64
	scanlines      uint64
	cycleInFrame   uint64
	nextLine       uint64
	pending        []Interrupt
}

func NewGLUE(cfg ...*config.Config) *GLUE {
	g := &GLUE{}
	if len(cfg) != 0 && cfg[0] != nil {
		g.configureTiming(cfg[0])
	}
	g.Reset()
	return g
}

func (g *GLUE) Contains(address uint32) bool {
	return address >= glueBase && address < glueBase+glueSize
}

func (g *GLUE) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		if address&1 == 0 {
			return uint32(g.configRegister >> 8), nil
		}
		return uint32(g.configRegister & 0xFF), nil
	default:
		return uint32(g.configRegister), nil
	}
}

func (g *GLUE) Peek(size cpu.Size, address uint32) (uint32, error) {
	return g.Read(size, address)
}

func (g *GLUE) Write(size cpu.Size, address uint32, value uint32) error {
	switch size {
	case cpu.Byte:
		if address&1 == 0 {
			g.configRegister = (g.configRegister & 0x00FF) | uint16(value&0xFF)<<8
		} else {
			g.configRegister = (g.configRegister & 0xFF00) | uint16(value&0xFF)
		}
	default:
		g.configRegister = uint16(value)
	}
	return nil
}

func (g *GLUE) Reset() {
	g.configRegister = 0
	g.cycleInFrame = 0
	g.nextLine = 1
	g.pending = g.pending[:0]
}

func (g *GLUE) Advance(cycles uint64) {
	if g.frameCycles == 0 || g.scanlines == 0 {
		return
	}

	g.cycleInFrame += cycles
	for g.cycleInFrame >= g.nextLineCycle() {
		if g.nextLine >= g.scanlines {
			g.pending = append(g.pending, Interrupt{Level: 4})
			g.cycleInFrame -= g.frameCycles
			g.nextLine = 1
			continue
		}
		g.pending = append(g.pending, Interrupt{Level: 2})
		g.nextLine++
	}
}

func (g *GLUE) DrainInterrupts() []Interrupt {
	if len(g.pending) == 0 {
		return nil
	}
	out := append([]Interrupt(nil), g.pending...)
	g.pending = g.pending[:0]
	return out
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

func (g *GLUE) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (g *GLUE) configureTiming(cfg *config.Config) {
	g.frameCycles = cfg.FrameCycles()
	if g.frameCycles == 0 {
		return
	}
	if cfg.FrameHz >= 55 {
		g.scanlines = glueNTSCScanlines
		return
	}
	g.scanlines = gluePALScanlines
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

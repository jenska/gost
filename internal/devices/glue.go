package devices

import "github.com/jenska/m68kemu"

const (
	glueBase = 0xFF8006
	glueSize = 2
)

// GLUE exposes the small subset of early ST system-control registers that
// EmuTOS probes during startup.
type GLUE struct {
	config uint16
}

func NewGLUE() *GLUE {
	g := &GLUE{}
	g.Reset()
	return g
}

func (g *GLUE) Contains(address uint32) bool {
	return address >= glueBase && address < glueBase+glueSize
}

func (g *GLUE) Read(size m68kemu.Size, address uint32) (uint32, error) {
	switch size {
	case m68kemu.Byte:
		if address&1 == 0 {
			return uint32(g.config >> 8), nil
		}
		return uint32(g.config & 0xFF), nil
	default:
		return uint32(g.config), nil
	}
}

func (g *GLUE) Peek(size m68kemu.Size, address uint32) (uint32, error) {
	return g.Read(size, address)
}

func (g *GLUE) Write(size m68kemu.Size, address uint32, value uint32) error {
	switch size {
	case m68kemu.Byte:
		if address&1 == 0 {
			g.config = (g.config & 0x00FF) | uint16(value&0xFF)<<8
		} else {
			g.config = (g.config & 0xFF00) | uint16(value&0xFF)
		}
	default:
		g.config = uint16(value)
	}
	return nil
}

func (g *GLUE) Reset() {
	g.config = 0
}

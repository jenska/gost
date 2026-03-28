package devices

import cpu "github.com/jenska/m68kemu"

const (
	psgBase = 0xFF8800
	psgSize = 4
)

// PSG keeps register state for future audio work.
type PSG struct {
	address byte
	regs    [16]byte
}

func NewPSG() *PSG {
	return &PSG{}
}

func (p *PSG) Contains(address uint32) bool {
	return address >= psgBase && address < psgBase+psgSize
}

func (p *PSG) WaitStates(cpu.Size, uint32) uint32 {
	return 2
}

func (p *PSG) Reset() {
	p.address = 0
	clear(p.regs[:])
}

func (p *PSG) Read(cpu.Size, uint32) (uint32, error) {
	return uint32(p.regs[p.address&0x0F]), nil
}

func (p *PSG) Write(size cpu.Size, address uint32, value uint32) error {
	switch address - psgBase {
	case 0, 1:
		p.address = byte(value) & 0x0F
	case 2, 3:
		p.regs[p.address&0x0F] = byte(value)
	}
	return nil
}

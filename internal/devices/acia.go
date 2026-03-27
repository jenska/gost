package devices

import "github.com/jenska/m68kemu"

const (
	aciaBase = 0xFFFC00
	aciaSize = 4
)

// ACIA fronts the IKBD as a memory-mapped serial interface.
type ACIA struct {
	ikbd     *IKBD
	control  byte
	status   byte
	data     byte
	pending  []Interrupt
	vector   uint8
	rxLoaded bool
}

func NewACIA(ikbd *IKBD) *ACIA {
	a := &ACIA{
		ikbd:   ikbd,
		vector: 0x4C,
	}
	a.Reset()
	return a
}

func (a *ACIA) Contains(address uint32) bool {
	return address >= aciaBase && address < aciaBase+aciaSize
}

func (a *ACIA) WaitStates(m68kemu.Size, uint32) uint32 {
	return 2
}

func (a *ACIA) Reset() {
	a.control = 0
	a.status = 0x02
	a.data = 0
	a.pending = a.pending[:0]
	a.rxLoaded = false
}

func (a *ACIA) Read(size m68kemu.Size, address uint32) (uint32, error) {
	a.pollIKBD()
	switch address - aciaBase {
	case 0, 1:
		return uint32(a.status), nil
	case 2, 3:
		value := a.data
		a.rxLoaded = false
		a.status &^= 0x01
		a.pollIKBD()
		return uint32(value), nil
	default:
		return 0, nil
	}
}

func (a *ACIA) Write(size m68kemu.Size, address uint32, value uint32) error {
	switch address - aciaBase {
	case 0, 1:
		a.control = byte(value)
	case 2, 3:
		a.ikbd.HandleCommand(byte(value))
	}
	a.pollIKBD()
	return nil
}

func (a *ACIA) Advance(uint64) {
	a.pollIKBD()
}

func (a *ACIA) DrainInterrupts() []Interrupt {
	if len(a.pending) == 0 {
		return nil
	}
	out := append([]Interrupt(nil), a.pending...)
	a.pending = a.pending[:0]
	return out
}

func (a *ACIA) pollIKBD() {
	if a.rxLoaded || !a.ikbd.HasData() {
		return
	}
	value, ok := a.ikbd.ReadByte()
	if !ok {
		return
	}
	a.data = value
	a.rxLoaded = true
	a.status |= 0x01
	vector := a.vector
	a.pending = append(a.pending, Interrupt{Level: 2, Vector: &vector})
}

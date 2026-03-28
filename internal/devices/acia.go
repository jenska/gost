package devices

import (
	"github.com/jenska/m68kemu"
	cpu "github.com/jenska/m68kemu"
)

const (
	aciaBase        = 0xFFFC00
	aciaChannelSize = 4
	aciaChannelCt   = 2
	aciaSize        = aciaChannelSize * aciaChannelCt
)

// ACIA fronts the IKBD as a memory-mapped serial interface.
type ACIA struct {
	ikbd     *IKBD
	control  [aciaChannelCt]byte
	status   [aciaChannelCt]byte
	data     [aciaChannelCt]byte
	rxLoaded [aciaChannelCt]bool
}

func NewACIA(ikbd *IKBD) *ACIA {
	a := &ACIA{ikbd: ikbd}
	a.Reset()
	return a
}

func (a *ACIA) Contains(address uint32) bool {
	return address >= aciaBase && address < aciaBase+aciaSize
}

func (a *ACIA) WaitStates(cpu.Size, uint32) uint32 {
	return 2
}

func (a *ACIA) Reset() {
	for i := 0; i < aciaChannelCt; i++ {
		a.control[i] = 0
		a.status[i] = 0x02
		a.data[i] = 0
		a.rxLoaded[i] = false
	}
}

func (a *ACIA) Read(size m68kemu.Size, address uint32) (uint32, error) {
	a.pollIKBD()
	channel := aciaChannelIndex(address)
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		return uint32(a.status[channel]), nil
	case 2, 3:
		value := a.data[channel]
		a.rxLoaded[channel] = false
		a.status[channel] &^= 0x81
		a.pollIKBD()
		return uint32(value), nil
	default:
		return 0, nil
	}
}

func (a *ACIA) Write(size m68kemu.Size, address uint32, value uint32) error {
	channel := aciaChannelIndex(address)
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		a.control[channel] = byte(value)
		if a.control[channel]&0x03 == 0x03 {
			a.status[channel] = 0x02
			a.rxLoaded[channel] = false
		}
	case 2, 3:
		if channel == 0 {
			a.ikbd.HandleCommand(byte(value))
		}
	}
	a.pollIKBD()
	return nil
}

func (a *ACIA) Advance(uint64) {
	a.pollIKBD()
}

func (a *ACIA) DrainInterrupts() []Interrupt {
	return nil
}

func (a *ACIA) pollIKBD() {
	if a.rxLoaded[0] || !a.ikbd.HasData() {
		return
	}
	value, err := a.ikbd.ReadByte()
	if err != nil {
		return
	}
	a.data[0] = value
	a.rxLoaded[0] = true
	a.status[0] |= 0x01
	// The ST routes keyboard RX interrupts through the MFP GPIP lines, not as a
	// direct CPU interrupt from the ACIA block. Until that path is modeled,
	// expose receive-ready status only.
	if a.control[0]&0x80 != 0 {
		a.status[0] |= 0x80
	}
}

func aciaChannelIndex(address uint32) uint32 {
	return (address - aciaBase) / aciaChannelSize
}

package devices

import (
	cpu "github.com/jenska/m68kemu"
)

const (
	// The machine exposes two ACIA channels within this 8-byte register window.
	aciaBase        = 0xFFFC00
	aciaChannelSize = 4
	aciaChannelCt   = 2
	aciaSize        = aciaChannelSize * aciaChannelCt
)

// ACIA fronts the IKBD as a memory-mapped serial interface.
type ACIA struct {
	ikbd        *IKBD
	keyboardIRQ func(bool)
	// control/status/data hold the memory-mapped register state per channel.
	control [aciaChannelCt]byte
	status  [aciaChannelCt]byte
	data    [aciaChannelCt]byte
	// rxLoaded reports whether the receive register currently contains unread data.
	rxLoaded [aciaChannelCt]bool
	// rxCooldown delays refilling the receive register until the next advance tick after a read.
	rxCooldown [aciaChannelCt]bool
}

// NewACIA wires the IKBD behind keyboard channel 0 and resets both channels.
func NewACIA(keyboardIRQ func(bool)) *ACIA {
	a := &ACIA{ikbd: NewIKBD(), keyboardIRQ: keyboardIRQ}
	a.Reset()
	return a
}

// Contains reports whether the given address is serviced by the ACIA.
func (a *ACIA) Contains(address uint32) bool {
	return address >= aciaBase && address < aciaBase+aciaSize
}

// WaitStates returns the fixed ACIA bus latency.
func (a *ACIA) WaitStates(cpu.Size, uint32) uint32 {
	return 2
}

// Reset restores each channel to its post-reset control and status state.
func (a *ACIA) Reset() {
	a.ikbd.Reset()
	for i := range aciaChannelCt {
		a.control[i] = 0
		a.status[i] = 0x02
		a.data[i] = 0
		a.rxLoaded[i] = false
		a.rxCooldown[i] = false
	}
}

// Read serves the status or data register selected by the CPU address and
// updates receive state when a data byte is consumed.
func (a *ACIA) Read(size cpu.Size, address uint32) (uint32, error) {
	channel := aciaChannelIndex(address)
	if !a.rxCooldown[channel] {
		a.pollIKBD()
	}
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		return uint32(a.status[channel]), nil
	case 2, 3:
		value := a.data[channel]
		a.rxLoaded[channel] = false
		a.rxCooldown[channel] = true
		a.status[channel] &^= 0x81
		if channel == 0 {
			a.updateKeyboardIRQ()
		}
		return uint32(value), nil
	default:
		return 0, nil
	}
}

// Write updates a control register or forwards keyboard-channel data bytes to
// the IKBD command parser.
func (a *ACIA) Write(size cpu.Size, address uint32, value uint32) error {
	channel := aciaChannelIndex(address)
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		a.control[channel] = byte(value)
		if a.control[channel]&0x03 == 0x03 {
			a.status[channel] = 0x02
			a.rxLoaded[channel] = false
			a.rxCooldown[channel] = false
			a.updateKeyboardIRQ()
		}
	case 2, 3:
		if channel == 0 {
			a.ikbd.HandleCommand(byte(value))
		}
	}
	a.pollIKBD()
	a.updateKeyboardIRQ()
	return nil
}

// Advance releases the one-tick receive cooldown and polls the IKBD for a new byte.
func (a *ACIA) Advance(uint64) {
	for i := range a.rxCooldown {
		a.rxCooldown[i] = false
	}
	a.pollIKBD()
}

func (a *ACIA) DrainInterrupts() []Interrupt {
	return nil
}

// pollIKBD loads one pending IKBD byte into channel 0 when the receive register is empty.
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
	if a.control[0]&0x80 != 0 {
		a.status[0] |= 0x80
	}
	a.updateKeyboardIRQ()
}

// updateKeyboardIRQ keeps the status IRQ bit and external IRQ callback in sync
// with receive-ready state.
func (a *ACIA) updateKeyboardIRQ() {
	if a.rxLoaded[0] && a.control[0]&0x80 != 0 {
		a.status[0] |= 0x80
	} else {
		a.status[0] &^= 0x80
	}
	if a.keyboardIRQ == nil {
		return
	}
	a.keyboardIRQ(a.status[0]&0x80 != 0)
}

// aciaChannelIndex maps an address inside the ACIA window to channel 0 or 1.
func aciaChannelIndex(address uint32) uint32 {
	return (address - aciaBase) / aciaChannelSize
}

func (a *ACIA) PushKey(scancode byte, pressed bool) {
	a.ikbd.PushKey(scancode, pressed)
}

func (a *ACIA) PushMouse(dx, dy int, buttons byte) {
	a.ikbd.PushMouse(dx, dy, buttons)
}

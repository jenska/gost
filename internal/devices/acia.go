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

// ACIA fronts the IKBD and MIDI byte devices as memory-mapped serial channels.
type ACIA struct {
	ikbd    *IKBD
	midi    *MIDI
	aciaIRQ func(bool)
	// control/status/data hold the memory-mapped register state per channel.
	control [aciaChannelCt]byte
	status  [aciaChannelCt]byte
	data    [aciaChannelCt]byte
	// rxLoaded reports whether the receive register currently contains unread data.
	rxLoaded [aciaChannelCt]bool
	// rxCooldown delays refilling the receive register until the next advance tick after a read.
	rxCooldown [aciaChannelCt]bool
}

// NewACIA wires the IKBD behind channel 0 and leaves MIDI channel 1 attachable.
func NewACIA(aciaIRQ func(bool)) *ACIA {
	a := &ACIA{ikbd: NewIKBD(), aciaIRQ: aciaIRQ}
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
	a.updateIRQ()
}

// Read serves the status or data register selected by the CPU address and
// updates receive state when a data byte is consumed.
func (a *ACIA) Read(size cpu.Size, address uint32) (uint32, error) {
	channel := aciaChannelIndex(address)
	if !a.rxCooldown[channel] {
		a.pollReceiveChannel(channel)
	}
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		return uint32(a.status[channel]), nil
	case 2, 3:
		value := a.data[channel]
		a.rxLoaded[channel] = false
		a.rxCooldown[channel] = true
		a.status[channel] &^= 0x81
		a.updateIRQ()
		return uint32(value), nil
	default:
		return 0, nil
	}
}

// Write updates a control register or forwards channel data bytes to the
// attached IKBD/MIDI endpoints.
func (a *ACIA) Write(size cpu.Size, address uint32, value uint32) error {
	channel := aciaChannelIndex(address)
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		a.control[channel] = byte(value)
		if a.control[channel]&0x03 == 0x03 {
			a.status[channel] = 0x02
			a.rxLoaded[channel] = false
			a.rxCooldown[channel] = false
			a.updateIRQ()
		}
	case 2, 3:
		if channel == 0 {
			a.ikbd.HandleCommand(byte(value))
		} else if a.midi != nil {
			a.midi.WriteOutput(byte(value))
		}
	}
	a.pollReceiveChannel(channel)
	a.updateIRQ()
	return nil
}

// Advance releases the one-tick receive cooldown and polls attached endpoints
// for new bytes.
func (a *ACIA) Advance(uint64) {
	for i := range a.rxCooldown {
		a.rxCooldown[i] = false
	}
	for channel := range aciaChannelCt {
		a.pollReceiveChannel(uint32(channel))
	}
}

func (a *ACIA) DrainInterrupts() []Interrupt {
	return nil
}

// pollReceiveChannel loads one pending endpoint byte when that channel's
// receive register is empty.
func (a *ACIA) pollReceiveChannel(channel uint32) {
	switch channel {
	case 0:
		a.pollIKBD()
	case 1:
		a.pollMIDI()
	}
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
	a.updateIRQ()
}

func (a *ACIA) pollMIDI() {
	if a.rxLoaded[1] || a.midi == nil || !a.midi.InputAvailable() {
		return
	}
	value, ok := a.midi.PopInput()
	if !ok {
		return
	}
	a.data[1] = value
	a.rxLoaded[1] = true
	a.status[1] |= 0x01
	if a.control[1]&0x80 != 0 {
		a.status[1] |= 0x80
	}
	a.updateIRQ()
}

// updateIRQ keeps both channel IRQ bits and the shared external ACIA IRQ line
// in sync with receive-ready state.
func (a *ACIA) updateIRQ() {
	asserted := false
	for channel := range aciaChannelCt {
		if a.rxLoaded[channel] && a.control[channel]&0x80 != 0 {
			a.status[channel] |= 0x80
			asserted = true
		} else {
			a.status[channel] &^= 0x80
		}
	}
	if a.aciaIRQ != nil {
		a.aciaIRQ(asserted)
	}
}

// aciaChannelIndex maps an address inside the ACIA window to channel 0 or 1.
func aciaChannelIndex(address uint32) uint32 {
	return (address - aciaBase) / aciaChannelSize
}

func (a *ACIA) AttachMIDI(midi *MIDI) {
	a.midi = midi
	a.pollMIDI()
}

func (a *ACIA) PushKey(scancode byte, pressed bool) {
	a.ikbd.PushKey(scancode, pressed)
}

func (a *ACIA) PushMouse(dx, dy int, buttons byte) {
	a.ikbd.PushMouse(dx, dy, buttons)
}

func (a *ACIA) PushMIDIInput(data []byte) {
	if a.midi == nil {
		return
	}
	a.midi.PushInput(data)
	a.pollMIDI()
}

func (a *ACIA) MIDIOutput() []byte {
	if a.midi == nil {
		return nil
	}
	return a.midi.Output()
}

func (a *ACIA) ClearMIDIOutput() {
	if a.midi != nil {
		a.midi.ClearOutput()
	}
}

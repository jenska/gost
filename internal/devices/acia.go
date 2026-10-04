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

	// Both ACIAs run from a 500 kHz clock (the 8 MHz machine clock / 16) and
	// receive 10-bit frames: start bit, 8 data bits, stop bit. The keyboard
	// link divides by 64 (7812.5 baud), MIDI by 16 (31250 baud). These are the
	// machine cycles one byte spends on the wire.
	aciaIKBDByteCycles = 10 * 64 * 16
	aciaMIDIByteCycles = 10 * 16 * 16
)

// aciaByteCycles is the per-channel receive time: channel 0 is the IKBD,
// channel 1 MIDI.
var aciaByteCycles = [aciaChannelCt]uint64{aciaIKBDByteCycles, aciaMIDIByteCycles}

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
	// rxShift is the byte arriving on each channel's serial line, valid while
	// rxShifting. rxShiftCycles counts down the machine cycles until it has been
	// received; at zero it moves to the data register as soon as that is free.
	rxShift       [aciaChannelCt]byte
	rxShifting    [aciaChannelCt]bool
	rxShiftCycles [aciaChannelCt]uint64
}

// NewACIA wires the IKBD behind channel 0 and  MIDI behind channel 1.
func NewACIA(aciaIRQ func(bool)) *ACIA {
	a := &ACIA{ikbd: NewIKBD(), midi: NewMIDI(), aciaIRQ: aciaIRQ}
	a.Reset()
	return a
}

// Contains reports whether the given address is serviced by the ACIA.
func (a *ACIA) AddressRange() (uint32, uint32) {
	return aciaBase, aciaBase + aciaSize - 1
}

// WaitStates is the extra delay of an ACIA register access on a real STF: 6
// cycles. The additional 0-8 cycles of E clock synchronisation are not modelled.
func (a *ACIA) WaitStates(cpu.Size, uint32) uint32 {
	return 6
}

// Reset restores each channel to its post-reset control and status state.
func (a *ACIA) Reset() {
	a.ikbd.Reset()
	for i := range aciaChannelCt {
		a.control[i] = 0
		a.status[i] = 0x02
		a.data[i] = 0
		a.rxLoaded[i] = false
		a.rxShifting[i] = false
		a.rxShiftCycles[i] = 0
	}
	a.updateIRQ()
}

// Read serves the status or data register selected by the CPU address and
// updates receive state when a data byte is consumed.
func (a *ACIA) Read(size cpu.Size, address uint32) (uint32, error) {
	channel := aciaChannelIndex(address)
	switch (address - aciaBase) % aciaChannelSize {
	case 0, 1:
		return uint32(a.status[channel]), nil
	case 2, 3:
		value := a.data[channel]
		a.rxLoaded[channel] = false
		a.status[channel] &^= 0x81
		// A byte that finished arriving while the register was full moves up now.
		a.completeReceive(channel)
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
			// Master reset clears the registers. A byte already on the wire
			// keeps arriving, so host input is not dropped.
			a.status[channel] = 0x02
			a.rxLoaded[channel] = false
			a.updateIRQ()
		}
	case 2, 3:
		if channel == 0 {
			a.ikbd.HandleCommand(byte(value))
		} else {
			a.midi.WriteOutput(byte(value))
		}
	}
	a.updateIRQ()
	return nil
}

// Advance moves each channel's serial line on by cycles, receiving bytes from
// the attached IKBD and MIDI endpoints at their baud rates.
func (a *ACIA) Advance(cycles uint64) {
	for channel := range uint32(aciaChannelCt) {
		a.advanceReceive(channel, cycles)
	}
}

// NextEventCycles reports when the next byte in flight finishes arriving.
func (a *ACIA) NextEventCycles() (uint64, bool) {
	var next uint64
	for channel := range aciaChannelCt {
		if !a.rxShifting[channel] || a.rxShiftCycles[channel] == 0 {
			continue
		}
		if next == 0 || a.rxShiftCycles[channel] < next {
			next = a.rxShiftCycles[channel]
		}
	}
	return next, next != 0
}

// advanceReceive runs one channel's receiver for cycles. The endpoint sends
// back to back, so a new byte starts as soon as the previous one is received.
func (a *ACIA) advanceReceive(channel uint32, cycles uint64) {
	for {
		if !a.rxShifting[channel] && !a.startReceive(channel) {
			return
		}
		if cycles < a.rxShiftCycles[channel] {
			a.rxShiftCycles[channel] -= cycles
			return
		}
		cycles -= a.rxShiftCycles[channel]
		a.rxShiftCycles[channel] = 0
		if !a.completeReceive(channel) {
			return // data register still full; hold the byte until it is read
		}
	}
}

// startReceive puts the endpoint's next byte on the channel's serial line.
func (a *ACIA) startReceive(channel uint32) bool {
	var value byte
	switch channel {
	case 0:
		if !a.ikbd.HasData() {
			return false
		}
		b, err := a.ikbd.ReadByte()
		if err != nil {
			return false
		}
		value = b
	case 1:
		b, ok := a.midi.PopInput()
		if !ok {
			return false
		}
		value = b
	default:
		return false
	}
	a.rxShift[channel] = value
	a.rxShifting[channel] = true
	a.rxShiftCycles[channel] = aciaByteCycles[channel]
	return true
}

// completeReceive moves a fully received byte into the empty data register and
// raises receive-ready. It reports false when there is no such byte or the
// register is still full.
func (a *ACIA) completeReceive(channel uint32) bool {
	if !a.rxShifting[channel] || a.rxShiftCycles[channel] != 0 || a.rxLoaded[channel] {
		return false
	}
	a.rxShifting[channel] = false
	a.data[channel] = a.rxShift[channel]
	a.rxLoaded[channel] = true
	a.status[channel] |= 0x01
	a.updateIRQ()
	return true
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

func (a *ACIA) PushKey(scancode byte, pressed bool) {
	a.ikbd.PushKey(scancode, pressed)
}

func (a *ACIA) PushMouse(dx, dy int, buttons byte) {
	a.ikbd.PushMouse(dx, dy, buttons)
}

func (a *ACIA) PushMIDIInput(data []byte) {
	a.midi.PushInput(data)
}

func (a *ACIA) MIDIOutput() []byte {
	return a.midi.Output()
}

func (a *ACIA) ClearMIDIOutput() {
	a.midi.ClearOutput()
}

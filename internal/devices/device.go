package devices

import (
	"encoding/binary"

	cpu "github.com/jenska/m68kemu"
)

// AutoVector is the Interrupt.Vector value that asks the CPU to auto-vector the
// request (vector 24+Level) rather than take a device-supplied vector.
const AutoVector = cpu.AutoVector

// Interrupt models a single interrupt a device drives onto the CPU: a priority
// Level (1-7) and an exception Vector (AutoVector to auto-vector). Devices
// expose their interrupt line through PendingIRQ/AckIRQ; the machine aggregates
// those lines into the CPU's IRQ source.
type Interrupt struct {
	Level  uint8
	Vector uint8
}

// Clocked represents a device that advances its internal state with CPU cycles.
// Devices implementing this interface will have their Advance method called
// during each emulation frame to progress their operation.
type Clocked interface {
	// Advance progresses the device state by the specified number of CPU cycles.
	Advance(cycles uint64)
}

// EventPredictor reports timing information for device state changes.
// This allows the machine to optimize its execution by predicting when
// devices will require service, rather than checking every cycle.
type EventPredictor interface {
	// NextEventCycles returns the number of CPU cycles until the next event
	// and a boolean indicating whether a prediction is available (true) or
	// no immediate events are predicted (false).
	NextEventCycles() (uint64, bool)
}

func readUint16BE(buf []byte, offset uint32) uint16 {
	return binary.BigEndian.Uint16(buf[offset:])
}

func readUint32BE(buf []byte, offset uint32) uint32 {
	return binary.BigEndian.Uint32(buf[offset:])
}

func writeBySize(buf []byte, offset uint32, size cpu.Size, value uint32) {
	switch size {
	case cpu.Byte:
		buf[offset] = byte(value)
	case cpu.Word:
		binary.BigEndian.PutUint16(buf[offset:], uint16(value))
	case cpu.Long:
		binary.BigEndian.PutUint32(buf[offset:], value)
	}
}

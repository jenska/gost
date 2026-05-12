package devices

import (
	"encoding/binary"

	cpu "github.com/jenska/m68kemu"
)

// Interrupt models a pending CPU interrupt from a device to the CPU.
type Interrupt struct {
	// Level is the interrupt priority level (1-7).
	Level uint8
	// Vector is a pointer to the exception vector address if applicable, nil for autovector.
	Vector *uint8
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

// InterruptSource exposes pending interrupts from a device to the CPU via
// the interrupt controller.
type InterruptSource interface {
	// DrainInterrupts returns all pending interrupts and clears the device's
	// interrupt queue. Interrupts are returned in priority order.
	DrainInterrupts() []Interrupt
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

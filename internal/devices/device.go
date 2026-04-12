package devices

import (
	"encoding/binary"

	cpu "github.com/jenska/m68kemu"
)

// Interrupt models a pending CPU interrupt coming from a device.
type Interrupt struct {
	Level  uint8
	Vector *uint8
}

// Clocked devices advance with emulated CPU cycles.
type Clocked interface {
	Advance(cycles uint64)
}

// EventPredictor reports the next clock boundary at which a device's visible
// state may change.
type EventPredictor interface {
	NextEventCycles() (uint64, bool)
}

// InterruptSource exposes pending interrupts to the machine.
type InterruptSource interface {
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

package devices

import "github.com/jenska/m68kemu"

// Interrupt models a pending CPU interrupt coming from a device.
type Interrupt struct {
	Level  uint8
	Vector *uint8
}

// Clocked devices advance with emulated CPU cycles.
type Clocked interface {
	Advance(cycles uint64)
}

// InterruptSource exposes pending interrupts to the machine.
type InterruptSource interface {
	DrainInterrupts() []Interrupt
}

func readUint16BE(buf []byte, offset uint32) uint16 {
	return uint16(buf[offset])<<8 | uint16(buf[offset+1])
}

func readUint32BE(buf []byte, offset uint32) uint32 {
	return uint32(buf[offset])<<24 |
		uint32(buf[offset+1])<<16 |
		uint32(buf[offset+2])<<8 |
		uint32(buf[offset+3])
}

func writeBySize(buf []byte, offset uint32, size m68kemu.Size, value uint32) {
	switch size {
	case m68kemu.Byte:
		buf[offset] = byte(value)
	case m68kemu.Word:
		buf[offset] = byte(value >> 8)
		buf[offset+1] = byte(value)
	case m68kemu.Long:
		buf[offset] = byte(value >> 24)
		buf[offset+1] = byte(value >> 16)
		buf[offset+2] = byte(value >> 8)
		buf[offset+3] = byte(value)
	}
}

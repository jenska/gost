package devices

import (
	"fmt"

	"github.com/jenska/m68kemu"
)

// RAM is the main ST memory and doubles as the framebuffer backing store.
type RAM struct {
	base uint32
	data []byte
}

func NewRAM(base, size uint32) *RAM {
	return &RAM{
		base: base,
		data: make([]byte, size),
	}
}

func (r *RAM) Base() uint32 {
	return r.base
}

func (r *RAM) Size() uint32 {
	return uint32(len(r.data))
}

func (r *RAM) Bytes() []byte {
	return r.data
}

func (r *RAM) Contains(address uint32) bool {
	return address >= r.base && address < r.base+uint32(len(r.data))
}

func (r *RAM) WaitStates(m68kemu.Size, uint32) uint32 {
	return 0
}

func (r *RAM) rangeCheck(address uint32, size m68kemu.Size) bool {
	end := address + uint32(size) - 1
	return address >= r.base && end < r.base+uint32(len(r.data))
}

func (r *RAM) Read(size m68kemu.Size, address uint32) (uint32, error) {
	if !r.rangeCheck(address, size) {
		return 0, m68kemu.BusError(address)
	}

	offset := address - r.base
	switch size {
	case m68kemu.Byte:
		return uint32(r.data[offset]), nil
	case m68kemu.Word:
		return uint32(readUint16BE(r.data, offset)), nil
	case m68kemu.Long:
		return readUint32BE(r.data, offset), nil
	default:
		return 0, fmt.Errorf("unsupported RAM read size %d", size)
	}
}

func (r *RAM) Write(size m68kemu.Size, address uint32, value uint32) error {
	if !r.rangeCheck(address, size) {
		return m68kemu.BusError(address)
	}

	offset := address - r.base
	writeBySize(r.data, offset, size, value)
	return nil
}

func (r *RAM) LoadAt(address uint32, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	end := address + uint32(len(payload)) - 1
	if !r.Contains(address) || !r.Contains(end) {
		return m68kemu.BusError(address)
	}

	copy(r.data[address-r.base:], payload)
	return nil
}

func (r *RAM) Reset() {
	clear(r.data)
}

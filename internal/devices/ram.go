package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

// RAM is the main ST memory and doubles as the framebuffer backing store.
type RAM struct {
	base       uint32
	data       []byte
	mmu        *MemoryConfig
	contention RAMContentionSource
}

type RAMContentionSource interface {
	WaitStatesForRAMAccess(cpu.Size, uint32) uint32
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

func (r *RAM) SetMemoryConfig(mmu *MemoryConfig) {
	r.mmu = mmu
}

func (r *RAM) SetContentionSource(source RAMContentionSource) {
	r.contention = source
}

func (r *RAM) Contains(address uint32) bool {
	limit := uint32(len(r.data))
	if r.mmu != nil {
		limit = r.mmu.LogicalSize()
	}
	return address >= r.base && address < r.base+limit
}

func (r *RAM) WaitStates(size cpu.Size, address uint32) uint32 {
	if r.contention == nil {
		return 0
	}
	return r.contention.WaitStatesForRAMAccess(size, address)
}

func (r *RAM) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		offset, present, err := r.translate(address)
		if err != nil {
			return 0, err
		}
		if !present {
			return 0, nil
		}
		return uint32(r.data[offset]), nil
	case cpu.Word:
		hi, hiPresent, err := r.translate(address)
		if err != nil {
			return 0, err
		}
		lo, loPresent, err := r.translate(address + 1)
		if err != nil {
			return 0, err
		}
		var value uint32
		if hiPresent {
			value |= uint32(r.data[hi]) << 8
		}
		if loPresent {
			value |= uint32(r.data[lo])
		}
		return value, nil
	case cpu.Long:
		b0, b0Present, err := r.translate(address)
		if err != nil {
			return 0, err
		}
		b1, b1Present, err := r.translate(address + 1)
		if err != nil {
			return 0, err
		}
		b2, b2Present, err := r.translate(address + 2)
		if err != nil {
			return 0, err
		}
		b3, b3Present, err := r.translate(address + 3)
		if err != nil {
			return 0, err
		}
		var value uint32
		if b0Present {
			value |= uint32(r.data[b0]) << 24
		}
		if b1Present {
			value |= uint32(r.data[b1]) << 16
		}
		if b2Present {
			value |= uint32(r.data[b2]) << 8
		}
		if b3Present {
			value |= uint32(r.data[b3])
		}
		return value, nil
	default:
		return 0, fmt.Errorf("unsupported RAM read size %d", size)
	}
}

func (r *RAM) Peek(size cpu.Size, address uint32) (uint32, error) {
	return r.Read(size, address)
}

func (r *RAM) Write(size cpu.Size, address uint32, value uint32) error {
	switch size {
	case cpu.Byte:
		offset, present, err := r.translate(address)
		if err != nil {
			return err
		}
		if present {
			r.data[offset] = byte(value)
		}
		return nil
	case cpu.Word:
		hi, hiPresent, err := r.translate(address)
		if err != nil {
			return err
		}
		lo, loPresent, err := r.translate(address + 1)
		if err != nil {
			return err
		}
		if hiPresent {
			r.data[hi] = byte(value >> 8)
		}
		if loPresent {
			r.data[lo] = byte(value)
		}
		return nil
	case cpu.Long:
		b0, b0Present, err := r.translate(address)
		if err != nil {
			return err
		}
		b1, b1Present, err := r.translate(address + 1)
		if err != nil {
			return err
		}
		b2, b2Present, err := r.translate(address + 2)
		if err != nil {
			return err
		}
		b3, b3Present, err := r.translate(address + 3)
		if err != nil {
			return err
		}
		if b0Present {
			r.data[b0] = byte(value >> 24)
		}
		if b1Present {
			r.data[b1] = byte(value >> 16)
		}
		if b2Present {
			r.data[b2] = byte(value >> 8)
		}
		if b3Present {
			r.data[b3] = byte(value)
		}
		return nil
	default:
		return fmt.Errorf("unsupported RAM write size %d", size)
	}
}

func (r *RAM) LoadAt(address uint32, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	end := address + uint32(len(payload)) - 1
	if address < r.base || end >= r.base+uint32(len(r.data)) {
		return cpu.BusError(address)
	}

	copy(r.data[address-r.base:], payload)
	return nil
}

func (r *RAM) CopyOut(address uint32, dst []byte) error {
	for i := range dst {
		offset, present, err := r.translate(address + uint32(i))
		if err != nil {
			return err
		}
		if present {
			dst[i] = r.data[offset]
			continue
		}
		dst[i] = 0
	}
	return nil
}

func (r *RAM) Reset() {
	// The 68000 RESET instruction resets external devices but does not erase
	// system RAM on an Atari ST. Keep RAM contents intact for warm-reset paths.
}

func (r *RAM) ColdReset() {
	clear(r.data)
}

func (r *RAM) translate(address uint32) (uint32, bool, error) {
	if address < r.base {
		return 0, false, cpu.BusError(address)
	}

	logical := address - r.base
	if r.mmu != nil {
		offset, state := r.mmu.ResolveAddress(logical)
		switch state {
		case memoryAddressPresent:
			if offset >= uint32(len(r.data)) {
				return 0, false, cpu.BusError(address)
			}
			return offset, true, nil
		case memoryAddressAbsent:
			return 0, false, nil
		default:
			return 0, false, cpu.BusError(address)
		}
	}

	if logical >= uint32(len(r.data)) {
		return 0, false, cpu.BusError(address)
	}
	return logical, true, nil
}

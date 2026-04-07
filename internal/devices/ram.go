package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

// RAM is the main ST memory and doubles as the framebuffer backing store.
type RAM struct {
	base uint32
	data []byte
	mmu  *MemoryConfig
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

func (r *RAM) Contains(address uint32) bool {
	limit := uint32(len(r.data))
	if r.mmu != nil {
		limit = r.mmu.LogicalSize()
	}
	return address >= r.base && address < r.base+limit
}

func (r *RAM) WaitStates(cpu.Size, uint32) uint32 {
	return 0
}

func (r *RAM) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		offset, err := r.translate(address)
		if err != nil {
			return 0, err
		}
		return uint32(r.data[offset]), nil
	case cpu.Word:
		hi, err := r.translate(address)
		if err != nil {
			return 0, err
		}
		lo, err := r.translate(address + 1)
		if err != nil {
			return 0, err
		}
		return uint32(r.data[hi])<<8 | uint32(r.data[lo]), nil
	case cpu.Long:
		b0, err := r.translate(address)
		if err != nil {
			return 0, err
		}
		b1, err := r.translate(address + 1)
		if err != nil {
			return 0, err
		}
		b2, err := r.translate(address + 2)
		if err != nil {
			return 0, err
		}
		b3, err := r.translate(address + 3)
		if err != nil {
			return 0, err
		}
		return uint32(r.data[b0])<<24 |
			uint32(r.data[b1])<<16 |
			uint32(r.data[b2])<<8 |
			uint32(r.data[b3]), nil
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
		offset, err := r.translate(address)
		if err != nil {
			return err
		}
		r.data[offset] = byte(value)
		return nil
	case cpu.Word:
		hi, err := r.translate(address)
		if err != nil {
			return err
		}
		lo, err := r.translate(address + 1)
		if err != nil {
			return err
		}
		r.data[hi] = byte(value >> 8)
		r.data[lo] = byte(value)
		return nil
	case cpu.Long:
		b0, err := r.translate(address)
		if err != nil {
			return err
		}
		b1, err := r.translate(address + 1)
		if err != nil {
			return err
		}
		b2, err := r.translate(address + 2)
		if err != nil {
			return err
		}
		b3, err := r.translate(address + 3)
		if err != nil {
			return err
		}
		r.data[b0] = byte(value >> 24)
		r.data[b1] = byte(value >> 16)
		r.data[b2] = byte(value >> 8)
		r.data[b3] = byte(value)
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
		offset, err := r.translate(address + uint32(i))
		if err != nil {
			return err
		}
		dst[i] = r.data[offset]
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

func (r *RAM) translate(address uint32) (uint32, error) {
	if address < r.base {
		return 0, cpu.BusError(address)
	}

	logical := address - r.base
	if r.mmu != nil {
		offset, ok := r.mmu.TranslateAddress(logical)
		if !ok || offset >= uint32(len(r.data)) {
			return 0, cpu.BusError(address)
		}
		return offset, nil
	}

	if logical >= uint32(len(r.data)) {
		return 0, cpu.BusError(address)
	}
	return logical, nil
}

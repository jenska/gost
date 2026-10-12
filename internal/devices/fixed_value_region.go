package devices

import cpu "github.com/jenska/m68kemu"

// FixedValueRegion models an absent hardware window that returns a stable value.
type FixedValueRegion struct {
	window
	value uint32
}

func NewFixedValueRegion(value uint32, r AddressRange) *FixedValueRegion {
	return &FixedValueRegion{window: window(r), value: value}
}

func (r *FixedValueRegion) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		return r.value & 0xFF, nil
	case cpu.Word:
		return r.value & 0xFFFF, nil
	default:
		return r.value, nil
	}
}

func (r *FixedValueRegion) Peek(size cpu.Size, address uint32) (uint32, error) {
	return r.Read(size, address)
}

func (r *FixedValueRegion) Write(cpu.Size, uint32, uint32) error {
	return nil
}

func (r *FixedValueRegion) Reset() {}

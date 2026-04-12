package devices

import cpu "github.com/jenska/m68kemu"

// FixedValueRegion models absent hardware windows that return a stable value.
type FixedValueRegion struct {
	ranges []AddressRange
	value  uint32
}

func NewFixedValueRegion(value uint32, ranges ...AddressRange) *FixedValueRegion {
	return &FixedValueRegion{
		ranges: append([]AddressRange(nil), ranges...),
		value:  value,
	}
}

func (r *FixedValueRegion) Contains(address uint32) bool {
	for _, candidate := range r.ranges {
		if address >= candidate.Start && address < candidate.End {
			return true
		}
	}
	return false
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

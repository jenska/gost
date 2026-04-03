package devices

import cpu "github.com/jenska/m68kemu"

// AddressRange describes a half-open address interval [Start, End).
type AddressRange struct {
	Start uint32
	End   uint32
}

// OpenBus absorbs reads and writes in intentionally unmapped holes.
type OpenBus struct {
	ranges []AddressRange
}

// NewOpenBus creates a device that claims the supplied unmapped address ranges.
func NewOpenBus(ranges ...AddressRange) *OpenBus {
	return &OpenBus{ranges: append([]AddressRange(nil), ranges...)}
}

// Contains reports whether the address falls inside one of the configured open
// bus holes.
func (o *OpenBus) Contains(address uint32) bool {
	for _, r := range o.ranges {
		if address >= r.Start && address < r.End {
			return true
		}
	}
	return false
}

// Read models an unmapped bus read by returning zero for all access sizes.
func (o *OpenBus) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		return 0, nil
	case cpu.Word:
		return 0, nil
	case cpu.Long:
		return 0, nil
	default:
		return 0, nil
	}
}

// Peek mirrors Read because open bus state does not have side effects.
func (o *OpenBus) Peek(size cpu.Size, address uint32) (uint32, error) {
	return o.Read(size, address)
}

// Write discards writes to unmapped bus regions.
func (o *OpenBus) Write(cpu.Size, uint32, uint32) error {
	return nil
}

// Reset is a no-op because the open bus device has no mutable runtime state.
func (o *OpenBus) Reset() {}

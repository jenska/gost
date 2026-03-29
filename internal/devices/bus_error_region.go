package devices

import cpu "github.com/jenska/m68kemu"

// BusErrorRegion models unmapped probe windows that must fault instead of
// reading as generic open bus.
type BusErrorRegion struct {
	ranges []AddressRange
}

func NewBusErrorRegion(ranges ...AddressRange) *BusErrorRegion {
	return &BusErrorRegion{ranges: append([]AddressRange(nil), ranges...)}
}

func (r *BusErrorRegion) Contains(address uint32) bool {
	for _, candidate := range r.ranges {
		if address >= candidate.Start && address < candidate.End {
			return true
		}
	}
	return false
}

func (r *BusErrorRegion) Read(_ cpu.Size, address uint32) (uint32, error) {
	return 0, cpu.BusError(address)
}

func (r *BusErrorRegion) Peek(size cpu.Size, address uint32) (uint32, error) {
	return r.Read(size, address)
}

func (r *BusErrorRegion) Write(_ cpu.Size, address uint32, _ uint32) error {
	return cpu.BusError(address)
}

func (r *BusErrorRegion) Reset() {}

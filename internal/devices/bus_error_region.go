package devices

import cpu "github.com/jenska/m68kemu"

// BusErrorRegion models an unmapped probe window that must fault instead of
// reading as generic open bus.
type BusErrorRegion struct {
	window
}

func NewBusErrorRegion(r AddressRange) *BusErrorRegion {
	return &BusErrorRegion{window(r)}
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

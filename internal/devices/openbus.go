package devices

import "github.com/jenska/m68kemu"

type AddressRange struct {
	Start uint32
	End   uint32
}

// OpenBus absorbs reads and writes in intentionally unmapped holes.
type OpenBus struct {
	ranges []AddressRange
}

func NewOpenBus(ranges ...AddressRange) *OpenBus {
	return &OpenBus{ranges: append([]AddressRange(nil), ranges...)}
}

func (o *OpenBus) Contains(address uint32) bool {
	for _, r := range o.ranges {
		if address >= r.Start && address < r.End {
			return true
		}
	}
	return false
}

func (o *OpenBus) Read(size m68kemu.Size, address uint32) (uint32, error) {
	switch size {
	case m68kemu.Byte:
		return 0, nil
	case m68kemu.Word:
		return 0, nil
	case m68kemu.Long:
		return 0, nil
	default:
		return 0, nil
	}
}

func (o *OpenBus) Peek(size m68kemu.Size, address uint32) (uint32, error) {
	return o.Read(size, address)
}

func (o *OpenBus) Write(m68kemu.Size, uint32, uint32) error {
	return nil
}

func (o *OpenBus) Reset() {}

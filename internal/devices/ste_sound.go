package devices

import cpu "github.com/jenska/m68kemu"

const (
	steSoundBase = 0xFF8900
	steSoundSize = 0x40
)

// STESound models the absent STE DMA sound window on a plain ST.
// EmuTOS probes this area with bus-error detection during machine setup.
type STESound struct {
	region *BusErrorRegion
}

func NewSTESound() *STESound {
	return &STESound{
		region: NewBusErrorRegion(AddressRange{Start: steSoundBase, End: steSoundBase + steSoundSize}),
	}
}

func (s *STESound) Contains(address uint32) bool {
	return s.region.Contains(address)
}

func (s *STESound) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (s *STESound) Read(size cpu.Size, address uint32) (uint32, error) {
	return s.region.Read(size, address)
}

func (s *STESound) Peek(size cpu.Size, address uint32) (uint32, error) {
	return s.region.Peek(size, address)
}

func (s *STESound) Write(size cpu.Size, address uint32, value uint32) error {
	return s.region.Write(size, address, value)
}

func (s *STESound) Reset() {}

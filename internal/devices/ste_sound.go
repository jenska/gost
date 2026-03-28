package devices

import cpu "github.com/jenska/m68kemu"

const (
	steSoundBase = 0xFF8900
	steSoundSize = 0x40
)

// STESound provides stable reads for the STE DMA sound register window.
// On plain ST machines this block is absent, but returning all-ones is a
// closer hardware probe result than the emulator's generic zero-valued open bus.
type STESound struct{}

func NewSTESound() *STESound {
	return &STESound{}
}

func (s *STESound) Contains(address uint32) bool {
	return address >= steSoundBase && address < steSoundBase+steSoundSize
}

func (s *STESound) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (s *STESound) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		return 0xFF, nil
	case cpu.Word:
		return 0xFFFF, nil
	case cpu.Long:
		return 0xFFFFFFFF, nil
	default:
		return 0xFFFFFFFF, nil
	}
}

func (s *STESound) Peek(size cpu.Size, address uint32) (uint32, error) {
	return s.Read(size, address)
}

func (s *STESound) Write(cpu.Size, uint32, uint32) error {
	return nil
}

func (s *STESound) Reset() {}

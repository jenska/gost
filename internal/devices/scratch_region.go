package devices

import cpu "github.com/jenska/m68kemu"

// ScratchRegion is a plain read/write word with no side effects. It satisfies
// guest probes of system-control addresses that GoST does not otherwise model
// (for example EmuTOS's startup poke at $FF8006), returning whatever was last
// written.
type ScratchRegion struct {
	start uint32
	end   uint32 // exclusive
	value uint16
}

// NewScratchRegion covers the half-open range [start, end).
func NewScratchRegion(start, end uint32) *ScratchRegion {
	return &ScratchRegion{start: start, end: end}
}

func (r *ScratchRegion) AddressRange() (uint32, uint32) {
	return r.start, r.end - 1
}

func (r *ScratchRegion) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (r *ScratchRegion) Read(size cpu.Size, address uint32) (uint32, error) {
	if size == cpu.Byte {
		if address&1 == 0 {
			return uint32(r.value >> 8), nil
		}
		return uint32(r.value & 0xFF), nil
	}
	return uint32(r.value), nil
}

func (r *ScratchRegion) Peek(size cpu.Size, address uint32) (uint32, error) {
	return r.Read(size, address)
}

func (r *ScratchRegion) Write(size cpu.Size, address uint32, value uint32) error {
	if size == cpu.Byte {
		if address&1 == 0 {
			r.value = (r.value & 0x00FF) | uint16(value&0xFF)<<8
		} else {
			r.value = (r.value & 0xFF00) | uint16(value&0xFF)
		}
		return nil
	}
	r.value = uint16(value)
	return nil
}

func (r *ScratchRegion) Reset() {
	r.value = 0
}

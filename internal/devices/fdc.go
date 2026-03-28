package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

const (
	fdcBase         = 0xFF8600
	fdcSize         = 0x10
	fdcSectorSize   = 512
	fdcSectorsTrack = 9
)

// FDC provides a small sector-based WD1772 style surface.
type FDC struct {
	command byte
	status  byte
	track   byte
	sector  byte
	data    byte
	buffer  []byte
	index   int
	diskA   []byte
	pending []Interrupt
	vector  uint8
}

func NewFDC() *FDC {
	f := &FDC{vector: 0x46}
	f.Reset()
	return f
}

func (f *FDC) Contains(address uint32) bool {
	return address >= fdcBase && address < fdcBase+fdcSize
}

func (f *FDC) WaitStates(cpu.Size, uint32) uint32 {
	return 8
}

func (f *FDC) Reset() {
	f.command = 0
	f.status = 0
	f.track = 0
	f.sector = 1
	f.data = 0
	f.buffer = nil
	f.index = 0
	f.pending = f.pending[:0]
}

func (f *FDC) InsertDisk(image []byte) error {
	if len(image)%fdcSectorSize != 0 {
		return fmt.Errorf("disk image size %d is not a multiple of %d", len(image), fdcSectorSize)
	}
	f.diskA = append([]byte(nil), image...)
	return nil
}

func (f *FDC) Read(size cpu.Size, address uint32) (uint32, error) {
	switch address - fdcBase {
	case 0:
		return uint32(f.status), nil
	case 2:
		return uint32(f.track), nil
	case 4:
		return uint32(f.sector), nil
	case 6:
		if f.index < len(f.buffer) {
			value := f.buffer[f.index]
			f.index++
			if f.index >= len(f.buffer) {
				f.status &^= 0x02
			}
			return uint32(value), nil
		}
		return uint32(f.data), nil
	default:
		return 0, nil
	}
}

func (f *FDC) Write(size cpu.Size, address uint32, value uint32) error {
	switch address - fdcBase {
	case 0:
		f.command = byte(value)
		return f.execute(byte(value))
	case 2:
		f.track = byte(value)
	case 4:
		f.sector = byte(value)
	case 6:
		f.data = byte(value)
	}
	return nil
}

func (f *FDC) Advance(uint64) {}

func (f *FDC) DrainInterrupts() []Interrupt {
	if len(f.pending) == 0 {
		return nil
	}
	out := append([]Interrupt(nil), f.pending...)
	f.pending = f.pending[:0]
	return out
}

func (f *FDC) execute(cmd byte) error {
	// 0x80 roughly matches "read sector" in this simplified model.
	if cmd&0xF0 != 0x80 {
		f.status = 0
		return nil
	}
	if len(f.diskA) == 0 {
		f.status = 0x10
		return nil
	}

	lba := int(f.track)*fdcSectorsTrack + int(f.sector) - 1
	offset := lba * fdcSectorSize
	if lba < 0 || offset+fdcSectorSize > len(f.diskA) {
		f.status = 0x10
		return nil
	}

	f.buffer = append(f.buffer[:0], f.diskA[offset:offset+fdcSectorSize]...)
	f.index = 0
	f.status = 0x03
	vector := f.vector
	f.pending = append(f.pending, Interrupt{Level: 5, Vector: &vector})
	return nil
}

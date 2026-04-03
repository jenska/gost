package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

const (
	fdcBase = 0xFF8600
	fdcSize = 0x10

	fdcSectorSize   = 512
	fdcSectorsTrack = 9

	dmaA0        = 0x0002
	dmaA1        = 0x0004
	dmaCSACSI    = 0x0008
	dmaSCReg     = 0x0010
	dmaDRQFloppy = 0x0080
	dmaWriteBit  = 0x0100

	dmaStatusOK     = 0x0001
	dmaStatusSCNot0 = 0x0002
	dmaStatusDataRQ = 0x0004

	fdcCmdRestore = 0x00
	fdcCmdSeek    = 0x10
	fdcCmdRead    = 0x80
	fdcCmdForceI  = 0xD0

	fdcStatusDataRQ  = 0x02
	fdcStatusTrack0  = 0x04
	fdcStatusRNF     = 0x10
	fdcStatusMotorOn = 0x80

	fdcOffsetData     = 0x04
	fdcOffsetControl  = 0x06
	fdcOffsetAddrHigh = 0x09
	fdcOffsetAddrMed  = 0x0B
	fdcOffsetAddrLow  = 0x0D
	fdcOffsetModeCtl  = 0x0F
)

// FDC models the Atari ST DMA/FDC register window closely enough for BIOS
// sector reads. Commands are routed indirectly through the DMA control/data
// registers and read sectors DMA into ST RAM.
type FDC struct {
	ram         *RAM
	irq         func(bool)
	control     uint16
	sectorCount uint16
	dmaAddr     uint32
	modeCtl     byte
	dmaData     uint16

	status byte
	track  byte
	sector byte
	data   byte
	typeI  bool

	diskA   []byte
	pending []Interrupt
	vector  uint8
	dmaOK   bool
}

func NewFDC(ram *RAM, irq func(bool)) *FDC {
	f := &FDC{ram: ram, irq: irq, vector: 0x46}
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
	f.control = 0
	f.sectorCount = 0
	f.dmaAddr = 0
	f.modeCtl = 0
	f.dmaData = 0
	f.status = f.baseStatus()
	f.track = 0
	f.sector = 1
	f.data = 0
	f.typeI = true
	f.pending = f.pending[:0]
	f.dmaOK = true
	if f.irq != nil {
		f.irq(false)
	}
}

func (f *FDC) InsertDisk(image []byte) error {
	if len(image)%fdcSectorSize != 0 {
		return fmt.Errorf("disk image size %d is not a multiple of %d", len(image), fdcSectorSize)
	}
	f.diskA = append([]byte(nil), image...)
	f.status = f.baseStatus()
	return nil
}

func (f *FDC) Read(size cpu.Size, address uint32) (uint32, error) {
	offset := address - fdcBase
	switch size {
	case cpu.Byte:
		return uint32(f.readByte(offset)), nil
	case cpu.Word:
		return uint32(f.readWord(offset)), nil
	case cpu.Long:
		hi := f.readWord(offset)
		lo := f.readWord(offset + 2)
		return uint32(hi)<<16 | uint32(lo), nil
	default:
		return 0, nil
	}
}

func (f *FDC) Write(size cpu.Size, address uint32, value uint32) error {
	offset := address - fdcBase
	switch size {
	case cpu.Byte:
		return f.writeByte(offset, byte(value))
	case cpu.Word:
		return f.writeWord(offset, uint16(value))
	case cpu.Long:
		if err := f.writeWord(offset, uint16(value>>16)); err != nil {
			return err
		}
		return f.writeWord(offset+2, uint16(value))
	default:
		return nil
	}
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

func (f *FDC) readByte(offset uint32) byte {
	switch offset {
	case fdcOffsetData:
		return byte(f.currentDataWord() >> 8)
	case fdcOffsetData + 1:
		return byte(f.currentDataWord())
	case fdcOffsetControl:
		return byte(f.dmaStatusWord() >> 8)
	case fdcOffsetControl + 1:
		return byte(f.dmaStatusWord())
	case fdcOffsetAddrHigh:
		return byte(f.dmaAddr >> 16)
	case fdcOffsetAddrMed:
		return byte(f.dmaAddr >> 8)
	case fdcOffsetAddrLow:
		return byte(f.dmaAddr)
	case fdcOffsetModeCtl:
		return f.modeCtl
	default:
		return 0
	}
}

func (f *FDC) readWord(offset uint32) uint16 {
	switch offset {
	case fdcOffsetData:
		return f.currentDataWord()
	case fdcOffsetControl:
		return f.dmaStatusWord()
	default:
		return uint16(f.readByte(offset))<<8 | uint16(f.readByte(offset+1))
	}
}

func (f *FDC) writeByte(offset uint32, value byte) error {
	switch offset {
	case fdcOffsetAddrHigh:
		f.dmaAddr = (f.dmaAddr & 0x00FFFF) | (uint32(value) << 16)
	case fdcOffsetAddrMed:
		f.dmaAddr = (f.dmaAddr & 0xFF00FF) | (uint32(value) << 8)
	case fdcOffsetAddrLow:
		f.dmaAddr = (f.dmaAddr & 0xFFFF00) | uint32(value)
	case fdcOffsetModeCtl:
		f.modeCtl = value
	default:
		// Byte accesses to the word registers are not used in the current
		// floppy path; keep them benign.
	}
	return nil
}

func (f *FDC) writeWord(offset uint32, value uint16) error {
	switch offset {
	case fdcOffsetData:
		return f.writeDataWord(value)
	case fdcOffsetControl:
		f.control = value
	default:
		if err := f.writeByte(offset, byte(value>>8)); err != nil {
			return err
		}
		return f.writeByte(offset+1, byte(value))
	}
	return nil
}

func (f *FDC) currentDataWord() uint16 {
	if f.control&dmaSCReg != 0 {
		return f.sectorCount
	}
	if !f.floppySelected() {
		return f.dmaData
	}

	switch f.control & (dmaA1 | dmaA0) {
	case 0:
		if f.irq != nil {
			f.irq(false)
		}
		return uint16(f.status)
	case dmaA0:
		return uint16(f.track)
	case dmaA1:
		return uint16(f.sector)
	case dmaA1 | dmaA0:
		return uint16(f.data)
	default:
		return 0
	}
}

func (f *FDC) writeDataWord(value uint16) error {
	if f.control&dmaSCReg != 0 {
		f.sectorCount = value
		return nil
	}
	if !f.floppySelected() {
		f.dmaData = value
		return nil
	}

	switch f.control & (dmaA1 | dmaA0) {
	case 0:
		return f.execute(byte(value))
	case dmaA0:
		f.track = byte(value)
	case dmaA1:
		f.sector = byte(value)
	case dmaA1 | dmaA0:
		f.data = byte(value)
	}
	return nil
}

func (f *FDC) dmaStatusWord() uint16 {
	status := uint16(0)
	if f.dmaOK {
		status |= dmaStatusOK
	}
	if f.sectorCount != 0 {
		status |= dmaStatusSCNot0
	}
	if f.status&fdcStatusDataRQ != 0 {
		status |= dmaStatusDataRQ
	}
	return status
}

func (f *FDC) execute(cmd byte) error {
	f.dmaOK = true
	if f.irq != nil {
		f.irq(false)
	}

	switch {
	case cmd&0xF0 == fdcCmdForceI:
		f.typeI = true
		f.status = f.baseStatus()
		f.queueInterrupt()
		return nil
	case cmd&0xF0 == fdcCmdRestore:
		f.typeI = true
		f.track = 0
		f.status = f.baseStatus()
		f.queueInterrupt()
		return nil
	case cmd&0xF0 == fdcCmdSeek:
		f.typeI = true
		f.track = f.data
		f.status = f.baseStatus()
		f.queueInterrupt()
		return nil
	case cmd&0xE0 == fdcCmdRead:
		return f.readSectors(cmd)
	default:
		f.status = f.baseStatus()
		f.queueInterrupt()
		return nil
	}
}

func (f *FDC) readSectors(cmd byte) error {
	f.typeI = false
	if len(f.diskA) == 0 {
		f.dmaOK = false
		f.status = f.baseStatus() | fdcStatusRNF
		f.queueInterrupt()
		return nil
	}

	count := int(f.sectorCount)
	if count <= 0 {
		count = 1
	}

	baseSector := int(f.sector)
	if baseSector <= 0 {
		baseSector = 1
	}

	buffer := make([]byte, 0, count*fdcSectorSize)
	for i := 0; i < count; i++ {
		lba := int(f.track)*fdcSectorsTrack + (baseSector - 1) + i
		offset := lba * fdcSectorSize
		if lba < 0 || offset+fdcSectorSize > len(f.diskA) {
			f.dmaOK = false
			f.status = f.baseStatus() | fdcStatusRNF
			f.queueInterrupt()
			return nil
		}
		buffer = append(buffer, f.diskA[offset:offset+fdcSectorSize]...)
	}

	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, buffer); err != nil {
			f.dmaOK = false
			f.status = f.baseStatus() | fdcStatusRNF
			f.queueInterrupt()
			return nil
		}
	}
	f.dmaAddr += uint32(len(buffer))

	if cmd&0x10 != 0 {
		f.sector = byte(baseSector + count)
	}
	f.sectorCount = 0
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) queueInterrupt() {
	vector := f.vector
	f.pending = append(f.pending, Interrupt{Level: 5, Vector: &vector})
	if f.irq != nil {
		f.irq(true)
	}
}

func (f *FDC) baseStatus() byte {
	var status byte
	if f.typeI && f.track == 0 {
		status |= fdcStatusTrack0
	}
	if len(f.diskA) != 0 {
		status |= fdcStatusMotorOn
	}
	return status
}

func (f *FDC) floppySelected() bool {
	return f.control&dmaDRQFloppy != 0 && f.control&dmaCSACSI == 0
}

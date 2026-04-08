package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

const (
	blitterBase = 0xFF8A00
	blitterSize = 0x3E

	blitterBusy   = 0x80
	blitterHog    = 0x40
	blitterSmudge = 0x20
	blitterLineNo = 0x0F

	blitterFXSR = 0x80
	blitterNFSR = 0x40
	blitterSkew = 0x0F
)

// Blitter provides a first usable Atari ST blitter model. Transfers are
// executed immediately in software when BUSY is written.
type Blitter struct {
	ram  *RAM
	regs [blitterSize]byte
}

func NewBlitter(ram *RAM) *Blitter {
	b := &Blitter{ram: ram}
	b.Reset()
	return b
}

func (b *Blitter) Contains(address uint32) bool {
	return address >= blitterBase && address < blitterBase+blitterSize
}

func (b *Blitter) Read(size cpu.Size, address uint32) (uint32, error) {
	offset, err := b.offsetFor(address, size)
	if err != nil {
		return 0, err
	}
	switch size {
	case cpu.Byte:
		return uint32(b.regs[offset]), nil
	case cpu.Word:
		return uint32(readUint16BE(b.regs[:], offset)), nil
	case cpu.Long:
		return readUint32BE(b.regs[:], offset), nil
	default:
		return 0, fmt.Errorf("unsupported blitter read size %d", size)
	}
}

func (b *Blitter) Peek(size cpu.Size, address uint32) (uint32, error) {
	return b.Read(size, address)
}

func (b *Blitter) Write(size cpu.Size, address uint32, value uint32) error {
	offset, err := b.offsetFor(address, size)
	if err != nil {
		return err
	}

	prevStatus := b.regs[0x3C]
	writeBySize(b.regs[:], offset, size, value)
	b.handleStatusWrite(prevStatus, b.regs[0x3C])
	return nil
}

func (b *Blitter) Reset() {
	clear(b.regs[:])
}

func (b *Blitter) offsetFor(address uint32, size cpu.Size) (uint32, error) {
	if !b.Contains(address) {
		return 0, cpu.BusError(address)
	}
	offset := address - blitterBase
	if offset+uint32(size) > blitterSize {
		return 0, cpu.BusError(address)
	}
	return offset, nil
}

func (b *Blitter) handleStatusWrite(prev, next byte) {
	if next&blitterBusy == 0 {
		return
	}
	if b.xCount() == 0 || b.yCount() == 0 {
		b.regs[0x3C] &^= blitterBusy
		return
	}
	b.execute()
	if prev&blitterHog != 0 {
		b.regs[0x3C] |= blitterHog
	}
}

func (b *Blitter) execute() {
	xCount := b.xCount()
	yCount := b.yCount()
	srcXInc := b.srcXInc()
	srcYInc := b.srcYInc()
	dstXInc := b.dstXInc()
	dstYInc := b.dstYInc()
	srcAddr := b.srcAddr()
	dstAddr := b.dstAddr()
	status := b.regs[0x3C]
	skew := b.regs[0x3D]

	for y := uint16(0); y < yCount; y++ {
		xc := xCount
		first := true
		var srcIn uint32

		for word := uint16(0); word < xc; word++ {
			last := word == xc-1
			srcOut := uint16(0)

			if srcXInc >= 0 {
				if first && skew&blitterFXSR != 0 {
					srcWord, _ := b.readWordSafe(srcAddr)
					srcIn = uint32(srcWord)
					srcAddr = addSignedAddressDelta(srcAddr, int32(srcXInc))
				}
				srcIn <<= 16

				if last && skew&blitterNFSR != 0 {
					srcAddr = addSignedAddressDelta(srcAddr, -int32(srcXInc))
				} else {
					nextWord, _ := b.readWordSafe(srcAddr)
					srcIn |= uint32(nextWord)
					if !last {
						srcAddr = addSignedAddressDelta(srcAddr, int32(srcXInc))
					}
				}
			} else {
				if first && skew&blitterFXSR != 0 {
					srcWord, _ := b.readWordSafe(srcAddr)
					srcIn = uint32(srcWord)
					srcAddr = addSignedAddressDelta(srcAddr, int32(srcXInc))
				} else {
					srcIn >>= 16
				}
				if last && skew&blitterNFSR != 0 {
					srcAddr = addSignedAddressDelta(srcAddr, -int32(srcXInc))
				} else {
					nextWord, _ := b.readWordSafe(srcAddr)
					srcIn |= uint32(nextWord) << 16
					if !last {
						srcAddr = addSignedAddressDelta(srcAddr, int32(srcXInc))
					}
				}
			}

			srcOut = uint16(srcIn >> (skew & blitterSkew))
			halftone := b.halftoneWord(status & blitterLineNo)
			hopOut := applyBlitterHOP(b.hop(), halftone, srcOut)

			dstIn, _ := b.readWordSafe(dstAddr)
			dstOut := applyBlitterOP(b.op(), hopOut, dstIn)
			mask := b.wordMask(first, last)
			final := (dstOut & mask) | (dstIn &^ mask)
			_ = b.writeWordSafe(dstAddr, final)

			if !last {
				dstAddr = addSignedAddressDelta(dstAddr, int32(dstXInc))
			}
			first = false
		}

		if dstYInc >= 0 {
			status = (status &^ blitterLineNo) | ((status + 1) & blitterLineNo)
		} else {
			status = (status &^ blitterLineNo) | ((status + 15) & blitterLineNo)
		}
		srcAddr = addSignedAddressDelta(srcAddr, int32(srcYInc))
		dstAddr = addSignedAddressDelta(dstAddr, int32(dstYInc))
	}

	b.setSrcAddr(srcAddr)
	b.setDstAddr(dstAddr)
	b.setYCount(0)
	b.regs[0x3C] = (status &^ blitterBusy) | (b.regs[0x3C] & (blitterHog | blitterSmudge))
}

func (b *Blitter) readWordSafe(address uint32) (uint16, error) {
	if b.ram == nil {
		return 0, cpu.BusError(address)
	}
	value, err := b.ram.Read(cpu.Word, address)
	if err != nil {
		return 0, err
	}
	return uint16(value), nil
}

func (b *Blitter) writeWordSafe(address uint32, value uint16) error {
	if b.ram == nil {
		return cpu.BusError(address)
	}
	return b.ram.Write(cpu.Word, address, uint32(value))
}

func (b *Blitter) halftoneWord(line byte) uint16 {
	index := uint32(line&0x0F) * 2
	return readUint16BE(b.regs[:], index)
}

func (b *Blitter) srcXInc() int16 {
	return int16(readUint16BE(b.regs[:], 0x20))
}

func (b *Blitter) srcYInc() int16 {
	return int16(readUint16BE(b.regs[:], 0x22))
}

func (b *Blitter) srcAddr() uint32 {
	return readUint32BE(b.regs[:], 0x24)
}

func (b *Blitter) endMask1() uint16 {
	return readUint16BE(b.regs[:], 0x28)
}

func (b *Blitter) endMask2() uint16 {
	return readUint16BE(b.regs[:], 0x2A)
}

func (b *Blitter) endMask3() uint16 {
	return readUint16BE(b.regs[:], 0x2C)
}

func (b *Blitter) dstXInc() int16 {
	return int16(readUint16BE(b.regs[:], 0x2E))
}

func (b *Blitter) dstYInc() int16 {
	return int16(readUint16BE(b.regs[:], 0x30))
}

func (b *Blitter) dstAddr() uint32 {
	return readUint32BE(b.regs[:], 0x32)
}

func (b *Blitter) xCount() uint16 {
	return readUint16BE(b.regs[:], 0x36)
}

func (b *Blitter) yCount() uint16 {
	return readUint16BE(b.regs[:], 0x38)
}

func (b *Blitter) hop() byte {
	return b.regs[0x3A] & 0x03
}

func (b *Blitter) op() byte {
	return b.regs[0x3B] & 0x0F
}

func (b *Blitter) wordMask(first, last bool) uint16 {
	switch {
	case first:
		return b.endMask1()
	case last:
		return b.endMask3()
	default:
		return b.endMask2()
	}
}

func (b *Blitter) setSrcAddr(value uint32) {
	writeBySize(b.regs[:], 0x24, cpu.Long, value)
}

func (b *Blitter) setDstAddr(value uint32) {
	writeBySize(b.regs[:], 0x32, cpu.Long, value)
}

func (b *Blitter) setYCount(value uint16) {
	writeBySize(b.regs[:], 0x38, cpu.Word, uint32(value))
}

func addSignedAddressDelta(address uint32, delta int32) uint32 {
	if delta >= 0 {
		return address + uint32(delta)
	}
	return address - uint32(-delta)
}

func applyBlitterHOP(hop byte, halftone, source uint16) uint16 {
	switch hop & 0x03 {
	case 0:
		return 0xFFFF
	case 1:
		return halftone
	case 2:
		return source
	case 3:
		return source & halftone
	default:
		return source
	}
}

func applyBlitterOP(op byte, source, dest uint16) uint16 {
	switch op & 0x0F {
	case 0x0:
		return 0x0000
	case 0x1:
		return source & dest
	case 0x2:
		return source &^ dest
	case 0x3:
		return source
	case 0x4:
		return (^source) & dest
	case 0x5:
		return dest
	case 0x6:
		return source ^ dest
	case 0x7:
		return source | dest
	case 0x8:
		return ^(source | dest)
	case 0x9:
		return ^(source ^ dest)
	case 0xA:
		return ^dest
	case 0xB:
		return source | ^dest
	case 0xC:
		return ^source
	case 0xD:
		return (^source) | dest
	case 0xE:
		return ^(source & dest)
	case 0xF:
		return 0xFFFF
	default:
		return source
	}
}

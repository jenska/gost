package devices

import "github.com/jenska/gost/internal/config"

const stePaletteColorMask uint16 = 0x0FFF

type steShifterModel struct{}

func NewSTEShifter(cfg *config.Config, ram *RAM) *Shifter {
	s := &Shifter{
		cfg:    cfg,
		ram:    ram,
		model:  stShifterModel{},
		width:  320,
		height: 200,
	}
	s.framebuffer = make([]byte, s.width*s.height*4)
	s.model = steShifterModel{}

	return s
}

func (steShifterModel) containsRegister(address uint32) bool {
	switch address {
	case shifterRegBaseHigh - 1,
		shifterRegBaseHigh,
		shifterRegBaseMid - 1,
		shifterRegBaseMid,
		shifterRegVideoAddrHigh - 1,
		shifterRegVideoAddrHigh,
		shifterRegVideoAddrMid - 1,
		shifterRegVideoAddrMid,
		shifterRegSyncMode,
		shifterRegResolution,
		shifterRegVideoAddrLow - 1,
		shifterRegVideoAddrLow,
		shifterRegBaseLow - 1,
		shifterRegBaseLow,
		shifterRegLineOffset - 1,
		shifterRegLineOffset,
		shifterRegFineScroll - 1,
		shifterRegFineScroll:
		return true
	default:
		return false
	}
}

func (steShifterModel) screenBase(s *Shifter) uint32 {
	return uint32(s.baseHigh)<<16 | uint32(s.baseMid)<<8 | uint32(s.baseLow)
}

// lineFetchBytes includes the extra word per bitplane the STE prefetches for
// a line whenever horizontal scrolling is on, so the pixels shifted in at the
// right edge have data: 8 bytes in low, 4 in medium and 2 in high resolution.
func (steShifterModel) lineFetchBytes(state shifterLineState, mode byte) uint32 {
	fetch, prefetch := uint32(160), uint32(8)
	switch mode {
	case 1:
		prefetch = 4
	case 2:
		fetch, prefetch = 80, 2
	}
	if state.fineScroll&0x0F != 0 {
		fetch += prefetch
	}
	return fetch
}

func (m steShifterModel) lineStrideBytes(state shifterLineState, mode byte) uint32 {
	return m.lineFetchBytes(state, mode) + uint32(state.lineOffset)*2
}

func (steShifterModel) fineScroll(state shifterLineState) int {
	return int(state.fineScroll & 0x0F)
}

func (steShifterModel) paletteMask() uint16 {
	return stePaletteColorMask
}

func (steShifterModel) paletteColorChannels(colorValue uint16) (r, g, b byte) {
	r = steNibbleToIntensity((colorValue >> 8) & 0x0F)
	g = steNibbleToIntensity((colorValue >> 4) & 0x0F)
	b = steNibbleToIntensity(colorValue & 0x0F)
	return r, g, b
}

func (steShifterModel) readByte(s *Shifter, address uint32) (byte, bool) {
	switch address {
	case shifterRegLineOffset:
		return s.lineOffset, true
	case shifterRegFineScroll:
		return s.fineScroll & 0x0F, true
	case shifterRegBaseLow:
		return s.baseLow, true
	default:
		return 0, false
	}
}

func (steShifterModel) writeByte(s *Shifter, address uint32, value byte) bool {
	switch address {
	case shifterRegVideoAddrHigh:
		s.writeVideoCounterByte(16, value)
		return true
	case shifterRegVideoAddrMid:
		s.writeVideoCounterByte(8, value)
		return true
	case shifterRegVideoAddrLow:
		s.writeVideoCounterByte(0, value)
		return true
	case shifterRegBaseLow:
		s.baseLow = value
		return true
	case shifterRegLineOffset:
		s.setLineOffset(value)
		return true
	case shifterRegFineScroll:
		s.fineScroll = value & 0x0F
		return true
	default:
		return false
	}
}

func steNibbleToIntensity(nibble uint16) byte {
	// STE palette nibbles store the least-significant bit in bit 3 to preserve
	// ST-compatible 3-bit values in bits 0..2 while extending them to 4 bits.
	value := ((nibble & 0x07) << 1) | ((nibble >> 3) & 0x01)
	return byte(value * 255 / 15)
}

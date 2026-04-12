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
	ram.SetContentionSource(s)

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

func (steShifterModel) screenBaseFromState(state shifterLineState) uint32 {
	return uint32(state.baseHigh)<<16 | uint32(state.baseMid)<<8 | uint32(state.baseLow)
}

func (steShifterModel) sameScreenBase(a, b shifterLineState) bool {
	return a.baseHigh == b.baseHigh && a.baseMid == b.baseMid && a.baseLow == b.baseLow
}

func (steShifterModel) lineStrideBytes(state shifterLineState, mode byte) uint32 {
	base := uint32(160)
	if mode == 2 {
		base = 80
	}
	return base + uint32(state.lineOffset)*2
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
	case shifterRegVideoAddrLow:
		return byte(s.currentVideoAddress()), true
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
	case shifterRegBaseLow:
		s.baseLow = value
		return true
	case shifterRegLineOffset:
		s.lineOffset = value
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

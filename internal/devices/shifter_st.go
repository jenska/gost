package devices

import "github.com/jenska/gost/internal/config"

const stPaletteColorMask uint16 = 0x0777

type stShifterModel struct{}

func NewSTShifter(cfg *config.Config, ram *RAM) *Shifter {
	s := &Shifter{
		cfg:    cfg,
		ram:    ram,
		model:  stShifterModel{},
		width:  320,
		height: 200,
	}
	s.framebuffer = make([]byte, s.width*s.height*4)
	s.model = stShifterModel{}
	ram.SetContentionSource(s)
	return s
}

func (stShifterModel) containsRegister(address uint32) bool {
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
		shifterRegResolution:
		return true
	default:
		return false
	}
}

func (stShifterModel) screenBase(s *Shifter) uint32 {
	return uint32(s.baseHigh)<<16 | uint32(s.baseMid)<<8
}

func (stShifterModel) screenBaseFromState(state shifterLineState) uint32 {
	return uint32(state.baseHigh)<<16 | uint32(state.baseMid)<<8
}

func (stShifterModel) sameScreenBase(a, b shifterLineState) bool {
	return a.baseHigh == b.baseHigh && a.baseMid == b.baseMid
}

func (stShifterModel) lineStrideBytes(_ shifterLineState, mode byte) uint32 {
	if mode == 2 {
		return 80
	}
	return 160
}

func (stShifterModel) fineScroll(shifterLineState) int {
	return 0
}

func (stShifterModel) paletteMask() uint16 {
	return stPaletteColorMask
}

func (stShifterModel) paletteColorChannels(colorValue uint16) (r, g, b byte) {
	r = byte(((colorValue >> 8) & 0x07) * 255 / 7)
	g = byte(((colorValue >> 4) & 0x07) * 255 / 7)
	b = byte((colorValue & 0x07) * 255 / 7)
	return r, g, b
}

func (stShifterModel) readByte(*Shifter, uint32) (byte, bool) {
	return 0, false
}

func (stShifterModel) writeByte(*Shifter, uint32, byte) bool {
	return false
}

package devices

import "github.com/jenska/m68kemu"

const (
	shifterBase       = 0xFF8200
	shifterBaseEnd    = 0xFF8261
	paletteBase       = 0xFF8240
	paletteRegisterCt = 16
)

// Shifter implements a small but useful subset of the STF video hardware.
type Shifter struct {
	ram          *RAM
	baseHigh     byte
	baseMid      byte
	baseLow      byte
	resolution   byte
	palette      [paletteRegisterCt]uint16
	framebuffer  []byte
	width        int
	height       int
	lastRendered uint64
}

func NewShifter(ram *RAM) *Shifter {
	s := &Shifter{
		ram:    ram,
		width:  320,
		height: 200,
	}
	s.framebuffer = make([]byte, s.width*s.height*4)
	return s
}

func (s *Shifter) Contains(address uint32) bool {
	if address >= shifterBase && address < shifterBaseEnd {
		return true
	}
	return address >= paletteBase && address < paletteBase+paletteRegisterCt*2
}

func (s *Shifter) WaitStates(m68kemu.Size, uint32) uint32 {
	return 2
}

func (s *Shifter) Reset() {
	s.baseHigh = 0
	s.baseMid = 0
	s.baseLow = 0
	s.resolution = 0
	for i := range s.palette {
		s.palette[i] = 0
	}
	s.width = 320
	s.height = 200
	s.framebuffer = make([]byte, s.width*s.height*4)
	s.lastRendered = 0
}

func (s *Shifter) Read(size m68kemu.Size, address uint32) (uint32, error) {
	switch {
	case address == 0xFF8201:
		return uint32(s.baseHigh), nil
	case address == 0xFF8203:
		return uint32(s.baseMid), nil
	case address == 0xFF820D:
		return uint32(s.baseLow), nil
	case address == 0xFF8260:
		return uint32(s.resolution), nil
	case address >= paletteBase && address < paletteBase+paletteRegisterCt*2:
		index := (address - paletteBase) / 2
		value := s.palette[index]
		switch size {
		case m68kemu.Byte:
			if address&1 == 0 {
				return uint32(value >> 8), nil
			}
			return uint32(value & 0xFF), nil
		default:
			return uint32(value), nil
		}
	default:
		return 0, nil
	}
}

func (s *Shifter) Write(size m68kemu.Size, address uint32, value uint32) error {
	switch {
	case address == 0xFF8201:
		s.baseHigh = byte(value)
	case address == 0xFF8203:
		s.baseMid = byte(value)
	case address == 0xFF820D:
		s.baseLow = byte(value)
	case address == 0xFF8260:
		s.resolution = byte(value) & 0x03
	case address >= paletteBase && address < paletteBase+paletteRegisterCt*2:
		index := (address - paletteBase) / 2
		switch size {
		case m68kemu.Byte:
			current := s.palette[index]
			if address&1 == 0 {
				current = (current & 0x00FF) | uint16(value&0xFF)<<8
			} else {
				current = (current & 0xFF00) | uint16(value&0xFF)
			}
			s.palette[index] = current
		default:
			s.palette[index] = uint16(value)
		}
	}
	return nil
}

func (s *Shifter) FrameBuffer() []byte {
	return append([]byte(nil), s.framebuffer...)
}

func (s *Shifter) Dimensions() (int, int) {
	return s.width, s.height
}

func (s *Shifter) ScreenBase() uint32 {
	return uint32(s.baseHigh)<<16 | uint32(s.baseMid)<<8 | uint32(s.baseLow)
}

func (s *Shifter) Render(cpuCycles uint64) bool {
	if cpuCycles == s.lastRendered {
		return false
	}
	s.lastRendered = cpuCycles

	width, height := s.currentDimensions()
	if width != s.width || height != s.height || len(s.framebuffer) != width*height*4 {
		s.width = width
		s.height = height
		s.framebuffer = make([]byte, width*height*4)
	}

	switch s.resolution & 0x03 {
	case 0:
		s.renderLow()
	case 1:
		s.renderMedium()
	default:
		s.renderUnsupported()
	}
	return true
}

func (s *Shifter) currentDimensions() (int, int) {
	switch s.resolution & 0x03 {
	case 1:
		return 640, 200
	case 0:
		return 320, 200
	default:
		return 640, 400
	}
}

func (s *Shifter) renderLow() {
	base := s.ScreenBase() - s.ram.Base()
	stride := uint32(160)
	for y := 0; y < 200; y++ {
		line := base + uint32(y)*stride
		for group := 0; group < 20; group++ {
			offset := line + uint32(group*8)
			if int(offset+7) >= len(s.ram.data) {
				continue
			}
			p0 := readUint16BE(s.ram.data, offset)
			p1 := readUint16BE(s.ram.data, offset+2)
			p2 := readUint16BE(s.ram.data, offset+4)
			p3 := readUint16BE(s.ram.data, offset+6)
			for bit := 0; bit < 16; bit++ {
				mask := uint16(1 << (15 - bit))
				index := 0
				if p0&mask != 0 {
					index |= 1
				}
				if p1&mask != 0 {
					index |= 2
				}
				if p2&mask != 0 {
					index |= 4
				}
				if p3&mask != 0 {
					index |= 8
				}
				s.writePixel(group*16+bit, y, s.palette[index])
			}
		}
	}
}

func (s *Shifter) renderMedium() {
	base := s.ScreenBase() - s.ram.Base()
	stride := uint32(160)
	for y := 0; y < 200; y++ {
		line := base + uint32(y)*stride
		for group := 0; group < 40; group++ {
			offset := line + uint32(group*4)
			if int(offset+3) >= len(s.ram.data) {
				continue
			}
			p0 := readUint16BE(s.ram.data, offset)
			p1 := readUint16BE(s.ram.data, offset+2)
			for bit := 0; bit < 16; bit++ {
				mask := uint16(1 << (15 - bit))
				index := 0
				if p0&mask != 0 {
					index |= 1
				}
				if p1&mask != 0 {
					index |= 2
				}
				s.writePixel(group*16+bit, y, s.palette[index])
			}
		}
	}
}

func (s *Shifter) renderUnsupported() {
	for i := 0; i < len(s.framebuffer); i += 4 {
		s.framebuffer[i] = 0
		s.framebuffer[i+1] = 0
		s.framebuffer[i+2] = 0
		s.framebuffer[i+3] = 0xFF
	}
}

func (s *Shifter) writePixel(x, y int, colorValue uint16) {
	if x < 0 || x >= s.width || y < 0 || y >= s.height {
		return
	}
	offset := (y*s.width + x) * 4
	r := byte(((colorValue >> 8) & 0x07) * 255 / 7)
	g := byte(((colorValue >> 4) & 0x07) * 255 / 7)
	b := byte((colorValue & 0x07) * 255 / 7)
	s.framebuffer[offset] = r
	s.framebuffer[offset+1] = g
	s.framebuffer[offset+2] = b
	s.framebuffer[offset+3] = 0xFF
}

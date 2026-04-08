package devices

import (
	"time"

	cpu "github.com/jenska/m68kemu"
)

const (
	shifterBase       = 0xFF8200
	paletteBase       = 0xFF8240
	paletteRegisterCt = 16

	shifterRegBaseHigh             = 0xFF8201
	shifterRegBaseMid              = 0xFF8203
	shifterRegVideoAddrHigh        = 0xFF8205
	shifterRegVideoAddrMid         = 0xFF8207
	shifterRegSyncMode             = 0xFF820A
	shifterRegResolution           = 0xFF8260
	stPaletteColorMask      uint16 = 0x0777

	defaultShifterClockHz = 8_000_000
	defaultShifterFrameHz = 50

	shifterContentionLowMediumNumerator = 3
	shifterContentionLowMediumDenom     = 4
	shifterContentionHighNumerator      = 3
	shifterContentionHighDenom          = 8
	shifterContentionWaitStates         = 1

	shifterSyncBlankDisplayBit = 0x01
	shifterRasterSegments      = 8
	// Approximate STF color-border geometry for 50 Hz timing.
	// Medium resolution keeps roughly the same time-domain border,
	// therefore horizontal border is doubled in pixels.
	shifterDisplayBorderLowLeft  = 32
	shifterDisplayBorderLowRight = 32
	shifterDisplayBorderMedLeft  = 64
	shifterDisplayBorderMedRight = 64
	shifterDisplayBorderTop      = 28
	shifterDisplayBorderBottom   = 28
	defaultMidResYScale          = 1
)

var (
	lowModeNibbleIndices    [1 << 16][4]byte
	mediumModeNibbleIndices [1 << 8][4]byte
	monoByteRGBA            [1 << 8][32]byte
)

func init() {
	initShifterLookupTables()
}

func initShifterLookupTables() {
	for key := 0; key < len(lowModeNibbleIndices); key++ {
		p0 := byte((key >> 12) & 0x0F)
		p1 := byte((key >> 8) & 0x0F)
		p2 := byte((key >> 4) & 0x0F)
		p3 := byte(key & 0x0F)
		for pixel := 0; pixel < 4; pixel++ {
			mask := byte(1 << (3 - pixel))
			var index byte
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
			lowModeNibbleIndices[key][pixel] = index
		}
	}

	for key := 0; key < len(mediumModeNibbleIndices); key++ {
		p0 := byte((key >> 4) & 0x0F)
		p1 := byte(key & 0x0F)
		for pixel := 0; pixel < 4; pixel++ {
			mask := byte(1 << (3 - pixel))
			var index byte
			if p0&mask != 0 {
				index |= 1
			}
			if p1&mask != 0 {
				index |= 2
			}
			mediumModeNibbleIndices[key][pixel] = index
		}
	}

	for value := 0; value < len(monoByteRGBA); value++ {
		for pixel := 0; pixel < 8; pixel++ {
			dst := pixel * 4
			if value&(1<<(7-pixel)) != 0 {
				monoByteRGBA[value][dst] = 0x00
				monoByteRGBA[value][dst+1] = 0x00
				monoByteRGBA[value][dst+2] = 0x00
				monoByteRGBA[value][dst+3] = 0xFF
				continue
			}
			monoByteRGBA[value][dst] = 0xFF
			monoByteRGBA[value][dst+1] = 0xFF
			monoByteRGBA[value][dst+2] = 0xFF
			monoByteRGBA[value][dst+3] = 0xFF
		}
	}
}

type shifterLineState struct {
	baseHigh byte
	baseMid  byte
	palette  [paletteRegisterCt]uint16
}

// ShifterDebugStats exposes optional per-frame instrumentation for performance
// analysis and rendering behavior inspection.
type ShifterDebugStats struct {
	FramesRendered uint64

	LastMode        byte
	LastWidth       int
	LastHeight      int
	LastRenderNanos int64
	LastPixelsDrawn uint64
	LastBlankPixels uint64
	LastVideoWords  uint64
	LastReadFaults  uint64
	LastWaitHits    uint64

	TotalRenderNanos int64
	TotalPixelsDrawn uint64
	TotalBlankPixels uint64
	TotalVideoWords  uint64
	TotalReadFaults  uint64
	TotalWaitHits    uint64

	FrameActive   bool
	FrameCyclePos uint64
	FrameCycles   uint64
	ScreenBase    uint32
	VideoAddress  uint32
}

// Shifter implements a small but useful subset of the STF video hardware.
type Shifter struct {
	ram                *RAM
	baseHigh           byte
	baseMid            byte
	syncMode           byte
	resolution         byte
	palette            [paletteRegisterCt]uint16
	framebuffer        []byte
	width              int
	height             int
	videoCounter       uint32
	frameCycles        uint64
	frameCyclePos      uint64
	frameMode          byte
	frameActive        bool
	lineStates         []shifterLineState
	slotSyncModes      []byte
	lastRendered       uint64
	debugEnabled       bool
	debugStats         ShifterDebugStats
	framePixelsDrawn   uint64
	frameBlankPixels   uint64
	frameVideoWords    uint64
	frameReadFaults    uint64
	frameWaitHits      uint64
	colorBorderVisible bool
	displayBuffer      []byte
	displayWidth       int
	displayHeight      int
	displayOffsetX     int
	displayOffsetY     int
	displayViewportW   int
	displayViewportH   int
	midResYScale       int
}

func NewShifter(ram *RAM) *Shifter {
	s := &Shifter{
		ram:          ram,
		width:        320,
		height:       200,
		midResYScale: defaultMidResYScale,
	}
	s.framebuffer = make([]byte, s.width*s.height*4)
	s.SetTiming(defaultShifterClockHz, defaultShifterFrameHz)
	return s
}

func (s *Shifter) SetTiming(clockHz, frameHz uint64) {
	if clockHz == 0 {
		clockHz = defaultShifterClockHz
	}
	if frameHz == 0 {
		frameHz = defaultShifterFrameHz
	}
	s.frameCycles = clockHz / frameHz
	if s.frameCycles == 0 {
		s.frameCycles = 1
	}
}

func (s *Shifter) SetDebug(enabled bool) {
	s.debugEnabled = enabled
}

func (s *Shifter) SetColorBorderVisible(visible bool) {
	s.colorBorderVisible = visible
}

func (s *Shifter) SetMidResYScale(scale int) {
	if scale < 1 {
		scale = 1
	}
	s.midResYScale = scale
}

func (s *Shifter) DebugStats() ShifterDebugStats {
	stats := s.debugStats
	stats.FrameActive = s.frameActive
	stats.FrameCyclePos = s.frameCyclePos
	stats.FrameCycles = s.frameCycles
	stats.ScreenBase = s.ScreenBase()
	stats.VideoAddress = s.currentVideoAddress()
	if s.frameActive {
		stats.LastMode = s.frameMode
		stats.LastWidth = s.width
		stats.LastHeight = s.height
		stats.LastPixelsDrawn = s.framePixelsDrawn
		stats.LastBlankPixels = s.frameBlankPixels
		stats.LastVideoWords = s.frameVideoWords
		stats.LastReadFaults = s.frameReadFaults
		stats.LastWaitHits = s.frameWaitHits
	}
	return stats
}

func (s *Shifter) Contains(address uint32) bool {
	if isPaletteAddress(address) {
		return true
	}
	return isShifterRegisterAddress(address)
}

func (s *Shifter) WaitStates(cpu.Size, uint32) uint32 {
	return 2
}

func (s *Shifter) Reset() {
	s.baseHigh = 0
	s.baseMid = 0
	s.syncMode = 0
	s.resolution = 0
	for i := range s.palette {
		s.palette[i] = 0
	}
	s.width = 320
	s.height = 200
	s.framebuffer = make([]byte, s.width*s.height*4)
	s.videoCounter = 0
	s.frameCyclePos = 0
	s.frameMode = 0
	s.frameActive = false
	s.lineStates = s.lineStates[:0]
	s.slotSyncModes = s.slotSyncModes[:0]
	s.lastRendered = 0
	s.debugStats = ShifterDebugStats{}
	s.framePixelsDrawn = 0
	s.frameBlankPixels = 0
	s.frameVideoWords = 0
	s.frameReadFaults = 0
	s.frameWaitHits = 0
	s.displayBuffer = nil
	s.displayWidth = 0
	s.displayHeight = 0
	s.displayOffsetX = 0
	s.displayOffsetY = 0
	s.displayViewportW = 0
	s.displayViewportH = 0
	s.midResYScale = defaultMidResYScale
}

func (s *Shifter) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		return uint32(s.readByte(address)), nil
	case cpu.Word:
		hi := s.readByte(address)
		lo := s.readByte(address + 1)
		return uint32(hi)<<8 | uint32(lo), nil
	case cpu.Long:
		hi := s.readByte(address)
		mh := s.readByte(address + 1)
		ml := s.readByte(address + 2)
		lo := s.readByte(address + 3)
		return uint32(hi)<<24 | uint32(mh)<<16 | uint32(ml)<<8 | uint32(lo), nil
	default:
		return 0, nil
	}
}

func (s *Shifter) Peek(size cpu.Size, address uint32) (uint32, error) {
	return s.Read(size, address)
}

func (s *Shifter) Write(size cpu.Size, address uint32, value uint32) error {
	switch size {
	case cpu.Byte:
		s.writeByte(address, byte(value))
	case cpu.Word:
		s.writeByte(address, byte(value>>8))
		s.writeByte(address+1, byte(value))
	case cpu.Long:
		s.writeByte(address, byte(value>>24))
		s.writeByte(address+1, byte(value>>16))
		s.writeByte(address+2, byte(value>>8))
		s.writeByte(address+3, byte(value))
	}
	return nil
}

func (s *Shifter) FrameBuffer() []byte {
	return append([]byte(nil), s.framebuffer...)
}

func (s *Shifter) Dimensions() (int, int) {
	return s.width, s.height
}

func (s *Shifter) DisplayBuffer() []byte {
	if len(s.displayBuffer) == 0 {
		return s.FrameBuffer()
	}
	return append([]byte(nil), s.displayBuffer...)
}

func (s *Shifter) DisplayDimensions() (int, int) {
	if s.displayWidth == 0 || s.displayHeight == 0 {
		return s.Dimensions()
	}
	return s.displayWidth, s.displayHeight
}

func (s *Shifter) DisplayViewport() (x, y, width, height int) {
	guestWidth, guestHeight := s.Dimensions()
	if guestWidth <= 0 || guestHeight <= 0 {
		return 0, 0, 0, 0
	}
	if s.displayWidth == 0 || s.displayHeight == 0 {
		return 0, 0, guestWidth, guestHeight
	}
	if s.displayViewportW <= 0 || s.displayViewportH <= 0 {
		return s.displayOffsetX, s.displayOffsetY, guestWidth, guestHeight
	}
	return s.displayOffsetX, s.displayOffsetY, s.displayViewportW, s.displayViewportH
}

func (s *Shifter) ScreenBase() uint32 {
	return uint32(s.baseHigh)<<16 | uint32(s.baseMid)<<8
}

func (s *Shifter) Render(cpuCycles uint64) bool {
	if cpuCycles == s.lastRendered {
		return false
	}
	s.lastRendered = cpuCycles
	s.BeginFrame()
	s.AdvanceFrame(s.frameCycles)
	return s.EndFrame()
}

func (s *Shifter) BeginFrame() {
	mode := s.resolution & 0x03
	width, height := dimensionsForResolution(mode)
	if width != s.width || height != s.height || len(s.framebuffer) != width*height*4 {
		s.width = width
		s.height = height
		s.framebuffer = make([]byte, width*height*4)
	}
	s.frameMode = mode
	s.frameCyclePos = 0
	s.frameActive = true
	s.framePixelsDrawn = 0
	s.frameBlankPixels = 0
	s.frameVideoWords = 0
	s.frameReadFaults = 0
	s.frameWaitHits = 0

	lineCount := height
	if cap(s.lineStates) < lineCount {
		s.lineStates = make([]shifterLineState, lineCount)
	} else {
		s.lineStates = s.lineStates[:lineCount]
	}
	if lineCount > 0 {
		state := s.snapshotLineState()
		for i := range s.lineStates {
			s.lineStates[i] = state
		}
	}

	slotCount := lineCount * shifterRasterSegments
	if cap(s.slotSyncModes) < slotCount {
		s.slotSyncModes = make([]byte, slotCount)
	} else {
		s.slotSyncModes = s.slotSyncModes[:slotCount]
	}
	for i := range s.slotSyncModes {
		s.slotSyncModes[i] = s.syncMode
	}
}

func (s *Shifter) AdvanceFrame(cycles uint64) {
	if !s.frameActive || len(s.lineStates) == 0 || cycles == 0 {
		return
	}
	prevPos := s.frameCyclePos
	prevLine := s.frameLineIndex(prevPos)
	s.frameCyclePos += cycles
	if s.frameCyclePos > s.frameCycles {
		s.frameCyclePos = s.frameCycles
	}
	nextLine := s.frameLineIndex(s.frameCyclePos)
	for line := prevLine + 1; line <= nextLine && line < len(s.lineStates); line++ {
		s.lineStates[line] = s.snapshotLineState()
	}

	if len(s.slotSyncModes) == 0 {
		return
	}
	prevSlot := s.frameSlotIndex(prevPos)
	nextSlot := s.frameSlotIndex(s.frameCyclePos)
	for slot := prevSlot + 1; slot <= nextSlot && slot < len(s.slotSyncModes); slot++ {
		s.slotSyncModes[slot] = s.syncMode
	}
}

func (s *Shifter) EndFrame() bool {
	if !s.frameActive {
		return false
	}
	if s.frameCyclePos < s.frameCycles {
		s.AdvanceFrame(s.frameCycles - s.frameCyclePos)
	}

	var renderStart time.Time
	if s.debugEnabled {
		renderStart = time.Now()
	}
	switch s.frameMode {
	case 0:
		s.renderLow()
	case 1:
		s.renderMedium()
	case 2:
		s.renderHigh()
	default:
		s.renderUnsupported()
	}
	if s.debugEnabled {
		renderNanos := time.Since(renderStart).Nanoseconds()
		s.debugStats.FramesRendered++
		s.debugStats.LastMode = s.frameMode
		s.debugStats.LastWidth = s.width
		s.debugStats.LastHeight = s.height
		s.debugStats.LastRenderNanos = renderNanos
		s.debugStats.LastPixelsDrawn = s.framePixelsDrawn
		s.debugStats.LastBlankPixels = s.frameBlankPixels
		s.debugStats.LastVideoWords = s.frameVideoWords
		s.debugStats.LastReadFaults = s.frameReadFaults
		s.debugStats.LastWaitHits = s.frameWaitHits
		s.debugStats.TotalRenderNanos += renderNanos
		s.debugStats.TotalPixelsDrawn += s.framePixelsDrawn
		s.debugStats.TotalBlankPixels += s.frameBlankPixels
		s.debugStats.TotalVideoWords += s.frameVideoWords
		s.debugStats.TotalReadFaults += s.frameReadFaults
		s.debugStats.TotalWaitHits += s.frameWaitHits
	}
	s.composeDisplayFrame()
	s.frameActive = false
	return true
}

func (s *Shifter) WaitStatesForRAMAccess(cpu.Size, uint32) uint32 {
	if !s.frameActive || s.frameCycles == 0 {
		return 0
	}
	if len(s.lineStates) == 0 {
		return 0
	}

	lineCycles := s.frameCycles / uint64(len(s.lineStates))
	if lineCycles == 0 {
		lineCycles = 1
	}
	posInLine := s.frameCyclePos % lineCycles
	if posInLine >= s.contentionWindowCycles(lineCycles) {
		return 0
	}
	if s.debugEnabled {
		s.frameWaitHits++
	}
	return shifterContentionWaitStates
}

func dimensionsForResolution(resolution byte) (int, int) {
	switch resolution & 0x03 {
	case 1:
		return 640, 200
	case 0:
		return 320, 200
	default:
		return 640, 400
	}
}

func (s *Shifter) contentionWindowCycles(lineCycles uint64) uint64 {
	switch s.frameMode {
	case 0, 1:
		window := (lineCycles * shifterContentionLowMediumNumerator) / shifterContentionLowMediumDenom
		if window == 0 {
			return 1
		}
		return window
	case 2:
		window := (lineCycles * shifterContentionHighNumerator) / shifterContentionHighDenom
		if window == 0 {
			return 1
		}
		return window
	default:
		return 0
	}
}

func (s *Shifter) renderLow() {
	fb := s.framebuffer
	debug := s.debugEnabled
	var drawn uint64
	stride := uint32(160)
	videoData, linearVideo := s.linearVideoRAM()
	videoLen := uint32(len(videoData))
	for y := 0; y < 200; y++ {
		lineState := s.lineState(y)
		base := screenBaseFromState(lineState)
		rowOffset := y * s.width * 4
		var reds, greens, blues [paletteRegisterCt]byte
		for i := 0; i < paletteRegisterCt; i++ {
			reds[i], greens[i], blues[i] = stColorChannels(lineState.palette[i])
		}
		line := base + uint32(y)*stride
		for group := 0; group < 20; group++ {
			offset := line + uint32(group*8)
			var p0, p1, p2, p3 uint16
			if linearVideo {
				if videoLen < 8 || offset > videoLen-8 {
					if debug {
						s.frameReadFaults++
					}
					continue
				}
				idx := int(offset)
				p0 = uint16(videoData[idx])<<8 | uint16(videoData[idx+1])
				p1 = uint16(videoData[idx+2])<<8 | uint16(videoData[idx+3])
				p2 = uint16(videoData[idx+4])<<8 | uint16(videoData[idx+5])
				p3 = uint16(videoData[idx+6])<<8 | uint16(videoData[idx+7])
				if debug {
					s.frameVideoWords += 4
				}
			} else {
				var ok bool
				p0, ok = s.readVideoWord(offset)
				if !ok {
					continue
				}
				p1, ok = s.readVideoWord(offset + 2)
				if !ok {
					continue
				}
				p2, ok = s.readVideoWord(offset + 4)
				if !ok {
					continue
				}
				p3, ok = s.readVideoWord(offset + 6)
				if !ok {
					continue
				}
			}
			groupOffset := rowOffset + group*64
			for chunk := 0; chunk < 4; chunk++ {
				shift := uint(12 - chunk*4)
				key := ((p0 >> shift) & 0x000F) << 12
				key |= ((p1 >> shift) & 0x000F) << 8
				key |= ((p2 >> shift) & 0x000F) << 4
				key |= (p3 >> shift) & 0x000F
				indices := lowModeNibbleIndices[key]
				dst := groupOffset + chunk*16

				i0 := indices[0]
				fb[dst] = reds[i0]
				fb[dst+1] = greens[i0]
				fb[dst+2] = blues[i0]
				fb[dst+3] = 0xFF

				i1 := indices[1]
				fb[dst+4] = reds[i1]
				fb[dst+5] = greens[i1]
				fb[dst+6] = blues[i1]
				fb[dst+7] = 0xFF

				i2 := indices[2]
				fb[dst+8] = reds[i2]
				fb[dst+9] = greens[i2]
				fb[dst+10] = blues[i2]
				fb[dst+11] = 0xFF

				i3 := indices[3]
				fb[dst+12] = reds[i3]
				fb[dst+13] = greens[i3]
				fb[dst+14] = blues[i3]
				fb[dst+15] = 0xFF
			}
			if debug {
				drawn += 16
			}
		}
	}
	if debug {
		s.framePixelsDrawn += drawn
	}
	lastBase := screenBaseFromState(s.lineState(199))
	s.videoCounter = lastBase + 200*stride
	s.applyBlankSegmentsLowMedium()
}

func (s *Shifter) renderMedium() {
	fb := s.framebuffer
	debug := s.debugEnabled
	var drawn uint64
	stride := uint32(160)
	videoData, linearVideo := s.linearVideoRAM()
	videoLen := uint32(len(videoData))
	for y := 0; y < 200; y++ {
		lineState := s.lineState(y)
		base := screenBaseFromState(lineState)
		rowOffset := y * s.width * 4
		var reds, greens, blues [paletteRegisterCt]byte
		for i := 0; i < paletteRegisterCt; i++ {
			reds[i], greens[i], blues[i] = stColorChannels(lineState.palette[i])
		}
		line := base + uint32(y)*stride
		for group := 0; group < 40; group++ {
			offset := line + uint32(group*4)
			var p0, p1 uint16
			if linearVideo {
				if videoLen < 4 || offset > videoLen-4 {
					if debug {
						s.frameReadFaults++
					}
					continue
				}
				idx := int(offset)
				p0 = uint16(videoData[idx])<<8 | uint16(videoData[idx+1])
				p1 = uint16(videoData[idx+2])<<8 | uint16(videoData[idx+3])
				if debug {
					s.frameVideoWords += 2
				}
			} else {
				var ok bool
				p0, ok = s.readVideoWord(offset)
				if !ok {
					continue
				}
				p1, ok = s.readVideoWord(offset + 2)
				if !ok {
					continue
				}
			}
			groupOffset := rowOffset + group*64
			for chunk := 0; chunk < 4; chunk++ {
				shift := uint(12 - chunk*4)
				key := ((p0 >> shift) & 0x000F) << 4
				key |= (p1 >> shift) & 0x000F
				indices := mediumModeNibbleIndices[key]
				dst := groupOffset + chunk*16

				i0 := indices[0]
				fb[dst] = reds[i0]
				fb[dst+1] = greens[i0]
				fb[dst+2] = blues[i0]
				fb[dst+3] = 0xFF

				i1 := indices[1]
				fb[dst+4] = reds[i1]
				fb[dst+5] = greens[i1]
				fb[dst+6] = blues[i1]
				fb[dst+7] = 0xFF

				i2 := indices[2]
				fb[dst+8] = reds[i2]
				fb[dst+9] = greens[i2]
				fb[dst+10] = blues[i2]
				fb[dst+11] = 0xFF

				i3 := indices[3]
				fb[dst+12] = reds[i3]
				fb[dst+13] = greens[i3]
				fb[dst+14] = blues[i3]
				fb[dst+15] = 0xFF
			}
			if debug {
				drawn += 16
			}
		}
	}
	if debug {
		s.framePixelsDrawn += drawn
	}
	lastBase := screenBaseFromState(s.lineState(199))
	s.videoCounter = lastBase + 200*stride
	s.applyBlankSegmentsLowMedium()
}

func (s *Shifter) renderHigh() {
	fb := s.framebuffer
	debug := s.debugEnabled
	var drawn uint64
	stride := uint32(80)
	videoData, linearVideo := s.linearVideoRAM()
	videoLen := uint32(len(videoData))
	for y := 0; y < 400; y++ {
		base := screenBaseFromState(s.lineState(y))
		rowOffset := y * s.width * 4
		line := base + uint32(y)*stride
		for group := 0; group < 40; group++ {
			offset := line + uint32(group*2)
			var pixels uint16
			if linearVideo {
				if videoLen < 2 || offset > videoLen-2 {
					if debug {
						s.frameReadFaults++
					}
					continue
				}
				idx := int(offset)
				pixels = uint16(videoData[idx])<<8 | uint16(videoData[idx+1])
				if debug {
					s.frameVideoWords++
				}
			} else {
				var ok bool
				pixels, ok = s.readVideoWord(offset)
				if !ok {
					continue
				}
			}
			dst := rowOffset + group*64
			hi := byte(pixels >> 8)
			lo := byte(pixels)
			copy(fb[dst:dst+32], monoByteRGBA[hi][:])
			copy(fb[dst+32:dst+64], monoByteRGBA[lo][:])
			if debug {
				drawn += 16
			}
		}
	}
	if debug {
		s.framePixelsDrawn += drawn
	}
	lastBase := screenBaseFromState(s.lineState(399))
	s.videoCounter = lastBase + 400*stride
	s.applyBlankSegmentsHigh()
}

func (s *Shifter) renderUnsupported() {
	for i := 0; i < len(s.framebuffer); i += 4 {
		s.framebuffer[i] = 0
		s.framebuffer[i+1] = 0
		s.framebuffer[i+2] = 0
		s.framebuffer[i+3] = 0xFF
	}
	s.videoCounter = s.ScreenBase()
}

func stColorChannels(colorValue uint16) (r, g, b byte) {
	r = byte(((colorValue >> 8) & 0x07) * 255 / 7)
	g = byte(((colorValue >> 4) & 0x07) * 255 / 7)
	b = byte((colorValue & 0x07) * 255 / 7)
	return r, g, b
}

func displayBorderForMode(mode byte) (left, right, top, bottom int) {
	top = shifterDisplayBorderTop
	bottom = shifterDisplayBorderBottom
	switch mode {
	case 0:
		return shifterDisplayBorderLowLeft, shifterDisplayBorderLowRight, top, bottom
	case 1:
		return shifterDisplayBorderMedLeft, shifterDisplayBorderMedRight, top, bottom
	default:
		return 0, 0, 0, 0
	}
}

func (s *Shifter) readVideoWord(address uint32) (uint16, bool) {
	if s.ram != nil && s.ram.base == 0 && s.ram.mmu == nil {
		dataLen := uint32(len(s.ram.data))
		if dataLen < 2 || address > dataLen-2 {
			if s.debugEnabled {
				s.frameReadFaults++
			}
			return 0, false
		}
		if s.debugEnabled {
			s.frameVideoWords++
		}
		offset := int(address)
		return uint16(s.ram.data[offset])<<8 | uint16(s.ram.data[offset+1]), true
	}

	hi, err := s.ram.translate(address)
	if err != nil {
		if s.debugEnabled {
			s.frameReadFaults++
		}
		return 0, false
	}
	lo, err := s.ram.translate(address + 1)
	if err != nil {
		if s.debugEnabled {
			s.frameReadFaults++
		}
		return 0, false
	}
	if s.debugEnabled {
		s.frameVideoWords++
	}
	return uint16(s.ram.data[hi])<<8 | uint16(s.ram.data[lo]), true
}

func (s *Shifter) linearVideoRAM() ([]byte, bool) {
	if s.ram == nil || s.ram.base != 0 || s.ram.mmu != nil {
		return nil, false
	}
	return s.ram.data, true
}

func isShifterRegisterAddress(address uint32) bool {
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

func isPaletteAddress(address uint32) bool {
	return address >= paletteBase && address < paletteBase+paletteRegisterCt*2
}

func (s *Shifter) readByte(address uint32) byte {
	switch address {
	case shifterRegBaseHigh:
		return s.baseHigh
	case shifterRegBaseMid:
		return s.baseMid
	case shifterRegVideoAddrHigh:
		return byte(s.currentVideoAddress() >> 16)
	case shifterRegVideoAddrMid:
		return byte(s.currentVideoAddress() >> 8)
	case shifterRegSyncMode:
		return s.syncMode
	case shifterRegResolution:
		return s.resolution
	default:
		if isPaletteAddress(address) {
			index := int((address - paletteBase) / 2)
			if index >= paletteRegisterCt {
				return 0
			}
			value := s.palette[index]
			if address&1 == 0 {
				return byte(value >> 8)
			}
			return byte(value)
		}
		return 0
	}
}

func (s *Shifter) writeByte(address uint32, value byte) {
	switch address {
	case shifterRegBaseHigh:
		s.baseHigh = value
	case shifterRegBaseMid:
		s.baseMid = value
	case shifterRegSyncMode:
		s.syncMode = value & 0x03
	case shifterRegResolution:
		s.resolution = value & 0x03
	default:
		if isPaletteAddress(address) {
			index := int((address - paletteBase) / 2)
			if index >= paletteRegisterCt {
				return
			}
			current := s.palette[index]
			if address&1 == 0 {
				current = (current & 0x00FF) | uint16(value)<<8
			} else {
				current = (current & 0xFF00) | uint16(value)
			}
			s.palette[index] = current & stPaletteColorMask
		}
	}
}

func (s *Shifter) currentVideoAddress() uint32 {
	if s.videoCounter != 0 {
		return s.videoCounter
	}
	return s.ScreenBase()
}

func (s *Shifter) snapshotLineState() shifterLineState {
	return shifterLineState{
		baseHigh: s.baseHigh,
		baseMid:  s.baseMid,
		palette:  s.palette,
	}
}

func (s *Shifter) lineState(line int) shifterLineState {
	if line < 0 || line >= len(s.lineStates) {
		return s.snapshotLineState()
	}
	return s.lineStates[line]
}

func (s *Shifter) frameLineIndex(cycles uint64) int {
	if len(s.lineStates) == 0 || s.frameCycles == 0 {
		return 0
	}
	index := int(cycles * uint64(len(s.lineStates)) / s.frameCycles)
	if index > len(s.lineStates) {
		return len(s.lineStates)
	}
	return index
}

func (s *Shifter) frameSlotIndex(cycles uint64) int {
	if len(s.slotSyncModes) == 0 || s.frameCycles == 0 {
		return 0
	}
	index := int(cycles * uint64(len(s.slotSyncModes)) / s.frameCycles)
	if index > len(s.slotSyncModes) {
		return len(s.slotSyncModes)
	}
	return index
}

func (s *Shifter) slotSyncMode(line, segment int) byte {
	if line < 0 || segment < 0 {
		return s.syncMode
	}
	idx := line*shifterRasterSegments + segment
	if idx < 0 || idx >= len(s.slotSyncModes) {
		return s.syncMode
	}
	return s.slotSyncModes[idx]
}

func screenBaseFromState(state shifterLineState) uint32 {
	return uint32(state.baseHigh)<<16 | uint32(state.baseMid)<<8
}

func (s *Shifter) applyBlankSegmentsLowMedium() {
	if len(s.slotSyncModes) == 0 || s.width <= 0 || s.height <= 0 {
		return
	}
	fb := s.framebuffer
	debug := s.debugEnabled
	var blanked uint64
	for y := 0; y < s.height; y++ {
		lineState := s.lineState(y)
		border := lineState.palette[0]
		r, g, b := stColorChannels(border)
		rowOffset := y * s.width * 4
		for seg := 0; seg < shifterRasterSegments; seg++ {
			if s.slotSyncMode(y, seg)&shifterSyncBlankDisplayBit == 0 {
				continue
			}
			x0 := (seg * s.width) / shifterRasterSegments
			x1 := ((seg + 1) * s.width) / shifterRasterSegments
			for x := x0; x < x1; x++ {
				dst := rowOffset + x*4
				fb[dst] = r
				fb[dst+1] = g
				fb[dst+2] = b
				fb[dst+3] = 0xFF
			}
			if debug {
				blanked += uint64(x1 - x0)
			}
		}
	}
	if debug {
		s.frameBlankPixels += blanked
	}
}

func (s *Shifter) applyBlankSegmentsHigh() {
	if len(s.slotSyncModes) == 0 || s.width <= 0 || s.height <= 0 {
		return
	}
	fb := s.framebuffer
	debug := s.debugEnabled
	var blanked uint64
	for y := 0; y < s.height; y++ {
		rowOffset := y * s.width * 4
		for seg := 0; seg < shifterRasterSegments; seg++ {
			if s.slotSyncMode(y, seg)&shifterSyncBlankDisplayBit == 0 {
				continue
			}
			x0 := (seg * s.width) / shifterRasterSegments
			x1 := ((seg + 1) * s.width) / shifterRasterSegments
			for x := x0; x < x1; x++ {
				dst := rowOffset + x*4
				fb[dst] = 0xFF
				fb[dst+1] = 0xFF
				fb[dst+2] = 0xFF
				fb[dst+3] = 0xFF
			}
			if debug {
				blanked += uint64(x1 - x0)
			}
		}
	}
	if debug {
		s.frameBlankPixels += blanked
	}
}

func (s *Shifter) composeDisplayFrame() {
	s.displayOffsetX = 0
	s.displayOffsetY = 0
	s.displayWidth = s.width
	s.displayHeight = s.height
	s.displayViewportW = s.width
	s.displayViewportH = s.height

	if s.width <= 0 || s.height <= 0 {
		s.displayBuffer = s.displayBuffer[:0]
		return
	}

	yScale := 1
	if s.frameMode == 1 {
		yScale = s.midResYScale
		if yScale < 1 {
			yScale = 1
		}
	}

	left, right, top, bottom := 0, 0, 0, 0
	if s.colorBorderVisible && (s.frameMode == 0 || s.frameMode == 1) {
		left, right, top, bottom = displayBorderForMode(s.frameMode)
	}

	outW := s.width + left + right
	activeH := s.height * yScale
	outH := activeH + top + bottom
	if outW == s.width && outH == s.height {
		s.displayBuffer = s.displayBuffer[:0]
		return
	}
	if outW <= 0 || outH <= 0 {
		s.displayBuffer = append(s.displayBuffer[:0], s.framebuffer...)
		return
	}

	totalBytes := outW * outH * 4
	if cap(s.displayBuffer) < totalBytes {
		s.displayBuffer = make([]byte, totalBytes)
	} else {
		s.displayBuffer = s.displayBuffer[:totalBytes]
	}

	topColor := s.palette[0]
	if len(s.lineStates) > 0 {
		topColor = s.lineStates[0].palette[0]
	}
	bottomColor := topColor
	if len(s.lineStates) > 0 {
		bottomColor = s.lineStates[len(s.lineStates)-1].palette[0]
	}
	topRGBA := colorToRGBA(topColor)
	bottomRGBA := colorToRGBA(bottomColor)
	prevActiveSrcY := -1

	for y := 0; y < outH; y++ {
		row := s.displayBuffer[y*outW*4 : (y+1)*outW*4]
		switch {
		case y < top:
			fillRowRGBA(row, 0, outW, topRGBA)
		case y >= top+activeH:
			fillRowRGBA(row, 0, outW, bottomRGBA)
		default:
			srcY := (y - top) / yScale
			if srcY == prevActiveSrcY && y > 0 {
				prev := s.displayBuffer[(y-1)*outW*4 : y*outW*4]
				copy(row, prev)
				continue
			}
			borderColor := s.palette[0]
			if srcY < len(s.lineStates) {
				borderColor = s.lineStates[srcY].palette[0]
			}
			borderRGBA := colorToRGBA(borderColor)
			if left > 0 {
				fillRowRGBA(row, 0, left, borderRGBA)
			}
			copy(row[left*4:(left+s.width)*4], s.framebuffer[srcY*s.width*4:(srcY+1)*s.width*4])
			if right > 0 {
				fillRowRGBA(row, left+s.width, outW, borderRGBA)
			}
			prevActiveSrcY = srcY
		}
	}

	s.displayOffsetX = left
	s.displayOffsetY = top
	s.displayViewportW = s.width
	s.displayViewportH = activeH
	s.displayWidth = outW
	s.displayHeight = outH
}

func colorToRGBA(color uint16) [4]byte {
	r, g, b := stColorChannels(color)
	return [4]byte{r, g, b, 0xFF}
}

func fillRowRGBA(row []byte, x0, x1 int, rgba [4]byte) {
	start := x0 * 4
	end := x1 * 4
	if start >= len(row) {
		return
	}
	if end > len(row) {
		end = len(row)
	}
	if start >= end {
		return
	}

	segment := row[start:end]
	segment[0] = rgba[0]
	segment[1] = rgba[1]
	segment[2] = rgba[2]
	segment[3] = rgba[3]

	filled := 4
	for filled < len(segment) {
		copy(segment[filled:], segment[:filled])
		filled *= 2
	}
}

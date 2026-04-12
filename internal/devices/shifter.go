package devices

import (
	"time"

	"github.com/jenska/gost/internal/config"
	cpu "github.com/jenska/m68kemu"
)

const (
	shifterBase       = 0xFF8200
	paletteBase       = 0xFF8240
	paletteRegisterCt = 16

	shifterRegBaseHigh      = 0xFF8201
	shifterRegBaseMid       = 0xFF8203
	shifterRegVideoAddrHigh = 0xFF8205
	shifterRegVideoAddrMid  = 0xFF8207
	shifterRegVideoAddrLow  = 0xFF8209
	shifterRegSyncMode      = 0xFF820A
	shifterRegBaseLow       = 0xFF820D
	shifterRegLineOffset    = 0xFF820F
	shifterRegResolution    = 0xFF8260
	shifterRegFineScroll    = 0xFF8265

	defaultShifterClockHz = 8_000_000
	defaultShifterFrameHz = 50

	shifterContentionLowMediumNumerator = 3
	shifterContentionLowMediumDenom     = 4
	shifterContentionHighNumerator      = 3
	shifterContentionHighDenom          = 8
	shifterContentionWaitStates         = 1

	shifterSyncBlankDisplayBit = 0x01
	// Track blank/display state at a finer horizontal resolution than the
	// original coarse 8-way split so short border/blank pulses map to much
	// narrower spans on screen.
	shifterRasterSegments = 64
	// Approximate STF color-border geometry for 50 Hz timing.
	// Medium resolution keeps roughly the same time-domain border,
	// therefore horizontal border is doubled in pixels.
	shifterDisplayBorderLowLeft  = 32
	shifterDisplayBorderLowRight = 32
	shifterDisplayBorderMedLeft  = 64
	shifterDisplayBorderMedRight = 64
	shifterDisplayBorderTop      = 28
	shifterDisplayBorderBottom   = 28
)

type shifterModel interface {
	containsRegister(address uint32) bool
	screenBase(s *Shifter) uint32
	screenBaseFromState(state shifterLineState) uint32
	sameScreenBase(a, b shifterLineState) bool
	lineStrideBytes(state shifterLineState, mode byte) uint32
	fineScroll(state shifterLineState) int
	paletteMask() uint16
	paletteColorChannels(colorValue uint16) (r, g, b byte)
	readByte(s *Shifter, address uint32) (byte, bool)
	writeByte(s *Shifter, address uint32, value byte) bool
}

var (
	// Lookup tables pre-expand bitplane nibbles/bytes into palette indices or
	// RGBA spans to keep the inner render loops branch-light.
	lowModeNibbleIndices    [1 << 16][4]byte
	mediumModeNibbleIndices [1 << 8][4]byte
	monoByteRGBA            [1 << 8][32]byte
)

func init() {
	// Low resolution: 4 bitplanes produce 16 pixels per 4 words.
	// Each key represents one nibble from each plane.
	for key := range len(lowModeNibbleIndices) {
		p0 := byte((key >> 12) & 0x0F)
		p1 := byte((key >> 8) & 0x0F)
		p2 := byte((key >> 4) & 0x0F)
		p3 := byte(key & 0x0F)
		for pixel := range 4 {
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

	// Medium resolution: 2 bitplanes produce 16 pixels per 2 words.
	for key := range len(mediumModeNibbleIndices) {
		p0 := byte((key >> 4) & 0x0F)
		p1 := byte(key & 0x0F)
		for pixel := range 4 {
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

	// High resolution: expand one byte into 8 monochrome RGBA pixels.
	for value := range len(monoByteRGBA) {
		for pixel := range 8 {
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
	baseHigh   byte
	baseMid    byte
	baseLow    byte
	lineOffset byte
	fineScroll byte
	palette    [paletteRegisterCt]uint16
}

// ShifterDebugStats exposes optional per-frame instrumentation for performance
// analysis and rendering behavior inspection.
type ShifterDebugStats struct {
	FramesRendered uint64

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

type Shifter struct {
	cfg              *config.Config
	ram              *RAM
	model            shifterModel
	baseHigh         byte
	baseMid          byte
	baseLow          byte
	lineOffset       byte
	fineScroll       byte
	syncMode         byte
	resolution       byte
	palette          [paletteRegisterCt]uint16
	framebuffer      []byte
	width            int
	height           int
	videoCounter     uint32
	frameCyclePos    uint64
	frameRenderer    func(s *Shifter)
	frameActive      bool
	lineStates       []shifterLineState
	slotSyncModes    []byte
	lastRendered     uint64
	debugEnabled     bool
	debugStats       ShifterDebugStats
	framePixelsDrawn uint64
	frameBlankPixels uint64
	frameVideoWords  uint64
	frameReadFaults  uint64
	frameWaitHits    uint64
	displayBuffer    []byte
	displayWidth     int
	displayHeight    int
	displayOffsetX   int
	displayOffsetY   int
	displayViewportW int
	displayViewportH int
}

func (s *Shifter) SetDebug(enabled bool) {
	s.debugEnabled = enabled
}

func (s *Shifter) DebugStats() ShifterDebugStats {
	stats := s.debugStats
	stats.FrameActive = s.frameActive
	stats.FrameCyclePos = s.frameCyclePos
	stats.FrameCycles = s.frameCycles()
	stats.ScreenBase = s.ScreenBase()
	stats.VideoAddress = s.currentVideoAddress()
	if s.frameActive {
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

func (s *Shifter) frameCycles() uint64 {
	if s.cfg == nil {
		return 0
	}
	return s.cfg.FrameCycles()
}

func (s *Shifter) Contains(address uint32) bool {
	if isPaletteAddress(address) {
		return true
	}
	return s.model.containsRegister(address)
}

func (s *Shifter) WaitStates(cpu.Size, uint32) uint32 {
	return 2
}

func (s *Shifter) Reset() {
	s.baseHigh = 0
	s.baseMid = 0
	s.baseLow = 0
	s.lineOffset = 0
	s.fineScroll = 0
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
	s.frameRenderer = renderLow
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
	if guestWidth == 0 || guestHeight == 0 {
		return 0, 0, 0, 0
	}
	if s.displayWidth == 0 || s.displayHeight == 0 {
		return 0, 0, guestWidth, guestHeight
	}
	if s.displayViewportW == 0 || s.displayViewportH == 0 {
		return s.displayOffsetX, s.displayOffsetY, guestWidth, guestHeight
	}
	return s.displayOffsetX, s.displayOffsetY, s.displayViewportW, s.displayViewportH
}

func (s *Shifter) ScreenBase() uint32 {
	return s.model.screenBase(s)
}

func (s *Shifter) Render(cpuCycles uint64) bool {
	if cpuCycles == s.lastRendered {
		return false
	}
	s.lastRendered = cpuCycles
	s.BeginFrame()
	s.AdvanceFrame(s.frameCycles())
	return s.EndFrame()
}

func (s *Shifter) BeginFrame() {
	mode := s.resolution & 3
	width, height := dimensionsForResolution(mode)
	if width != s.width || height != s.height || len(s.framebuffer) != width*height*4 {
		s.width = width
		s.height = height
		s.framebuffer = make([]byte, width*height*4)
	}
	switch mode {
	case 0:
		s.frameRenderer = renderLow
	case 1:
		s.frameRenderer = renderMedium
	default:
		s.frameRenderer = renderHigh
	}
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
		// New lines inherit the current register/palette state until
		// AdvanceFrame snapshots a new state boundary.
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
	frameCycles := s.frameCycles()
	prevPos := s.frameCyclePos
	prevLine := s.frameLineIndex(prevPos)
	s.frameCyclePos += cycles
	if s.frameCyclePos > frameCycles {
		s.frameCyclePos = frameCycles
	}
	nextLine := s.frameLineIndex(s.frameCyclePos)
	// Snapshot state transitions per scanline so mid-frame register writes
	// affect only the lines rendered after the write.
	for line := prevLine + 1; line <= nextLine && line < len(s.lineStates); line++ {
		s.lineStates[line] = s.snapshotLineState()
	}

	if len(s.slotSyncModes) == 0 {
		return
	}
	prevSlot := s.frameSlotIndex(prevPos)
	nextSlot := s.frameSlotIndex(s.frameCyclePos)
	// Sync mode is captured in coarser horizontal raster segments.
	for slot := prevSlot + 1; slot <= nextSlot && slot < len(s.slotSyncModes); slot++ {
		s.slotSyncModes[slot] = s.syncMode
	}
}

func (s *Shifter) EndFrame() bool {
	if !s.frameActive {
		return false
	}
	frameCycles := s.frameCycles()
	if s.frameCyclePos < frameCycles {
		s.AdvanceFrame(frameCycles - s.frameCyclePos)
	}

	var renderStart time.Time
	if s.debugEnabled {
		renderStart = time.Now()
	}

	s.frameRenderer(s)

	if s.debugEnabled {
		renderNanos := time.Since(renderStart).Nanoseconds()
		s.debugStats.FramesRendered++
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
	frameCycles := s.frameCycles()
	if !s.frameActive || frameCycles == 0 {
		return 0
	}
	if len(s.lineStates) == 0 {
		return 0
	}

	lineCycles := frameCycles / uint64(len(s.lineStates))
	if lineCycles == 0 {
		lineCycles = 1
	}
	posInLine := s.frameCyclePos % lineCycles
	// The shifter only contends with CPU RAM access while fetching display data.
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
	switch s.resolution & 3 {
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

func renderLow(s *Shifter) {
	fb := s.framebuffer
	debug := s.debugEnabled
	model := s.model
	var drawn uint64
	videoData, linearVideo := s.linearVideoRAM()
	videoLen := uint32(len(videoData))
	var lineAddr uint32
	var prevState shifterLineState
	for y := range 200 {
		lineState := s.lineState(y)
		if y == 0 {
			lineAddr = model.screenBaseFromState(lineState)
		} else {
			if model.sameScreenBase(prevState, lineState) {
				lineAddr += model.lineStrideBytes(prevState, 0)
			} else {
				lineAddr = model.screenBaseFromState(lineState)
			}
		}
		rowOffset := y * s.width * 4
		row := fb[rowOffset : rowOffset+s.width*4]
		var reds, greens, blues [paletteRegisterCt]byte
		for i := range paletteRegisterCt {
			reds[i], greens[i], blues[i] = model.paletteColorChannels(lineState.palette[i])
		}
		scroll := model.fineScroll(lineState)
		groups := 20 + ((scroll + 15) / 16)
		for group := range groups {
			offset := lineAddr + uint32(group*8)
			var p0, p1, p2, p3 uint16
			if linearVideo {
				// Fast path when RAM is identity mapped.
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
				// Fallback path for MMU/translated reads.
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
			groupX := group*16 - scroll
			for chunk := range 4 {
				shift := uint(12 - chunk*4)
				key := ((p0 >> shift) & 0x000F) << 12
				key |= ((p1 >> shift) & 0x000F) << 8
				key |= ((p2 >> shift) & 0x000F) << 4
				key |= (p3 >> shift) & 0x000F
				indices := lowModeNibbleIndices[key]
				baseX := groupX + chunk*4

				for pixel := range 4 {
					x := baseX + pixel
					if x < 0 || x >= s.width {
						continue
					}
					dst := x * 4
					index := indices[pixel]
					row[dst] = reds[index]
					row[dst+1] = greens[index]
					row[dst+2] = blues[index]
					row[dst+3] = 0xFF
				}
			}
			if debug {
				drawn += 16
			}
		}
		prevState = lineState
	}
	if debug {
		s.framePixelsDrawn += drawn
	}
	s.videoCounter = lineAddr + model.lineStrideBytes(prevState, 0)
	s.applyBlankSegments()
}

func renderMedium(s *Shifter) {
	fb := s.framebuffer
	debug := s.debugEnabled
	model := s.model
	var drawn uint64
	videoData, linearVideo := s.linearVideoRAM()
	videoLen := uint32(len(videoData))
	var lineAddr uint32
	var prevState shifterLineState
	for y := range 200 {
		lineState := s.lineState(y)
		if y == 0 {
			lineAddr = model.screenBaseFromState(lineState)
		} else {
			if model.sameScreenBase(prevState, lineState) {
				lineAddr += model.lineStrideBytes(prevState, 1)
			} else {
				lineAddr = model.screenBaseFromState(lineState)
			}
		}
		rowOffset := y * s.width * 4
		row := fb[rowOffset : rowOffset+s.width*4]
		var reds, greens, blues [paletteRegisterCt]byte
		for i := range paletteRegisterCt {
			reds[i], greens[i], blues[i] = model.paletteColorChannels(lineState.palette[i])
		}
		scroll := model.fineScroll(lineState)
		groups := 40 + ((scroll + 15) / 16)
		for group := range groups {
			offset := lineAddr + uint32(group*4)
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
			groupX := group*16 - scroll
			for chunk := range 4 {
				shift := uint(12 - chunk*4)
				key := ((p0 >> shift) & 0x000F) << 4
				key |= (p1 >> shift) & 0x000F
				indices := mediumModeNibbleIndices[key]
				baseX := groupX + chunk*4

				for pixel := range 4 {
					x := baseX + pixel
					if x < 0 || x >= s.width {
						continue
					}
					dst := x * 4
					index := indices[pixel]
					row[dst] = reds[index]
					row[dst+1] = greens[index]
					row[dst+2] = blues[index]
					row[dst+3] = 0xFF
				}
			}
			if debug {
				drawn += 16
			}
		}
		prevState = lineState
	}
	if debug {
		s.framePixelsDrawn += drawn
	}
	s.videoCounter = lineAddr + model.lineStrideBytes(prevState, 1)
	s.applyBlankSegments()
}

func renderHigh(s *Shifter) {
	fb := s.framebuffer
	debug := s.debugEnabled
	model := s.model
	var drawn uint64
	videoData, linearVideo := s.linearVideoRAM()
	videoLen := uint32(len(videoData))
	var lineAddr uint32
	var prevState shifterLineState
	for y := range 400 {
		lineState := s.lineState(y)
		if y == 0 {
			lineAddr = model.screenBaseFromState(lineState)
		} else {
			if model.sameScreenBase(prevState, lineState) {
				lineAddr += model.lineStrideBytes(prevState, 2)
			} else {
				lineAddr = model.screenBaseFromState(lineState)
			}
		}
		rowOffset := y * s.width * 4
		for group := range 40 {
			offset := lineAddr + uint32(group*2)
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
		prevState = lineState
	}
	if debug {
		s.framePixelsDrawn += drawn
	}
	s.videoCounter = lineAddr + model.lineStrideBytes(prevState, 2)
	s.applyBlankSegments()
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
	// Fast path for plain RAM mapping; avoids per-byte translation overhead.
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

	hi, hiPresent, err := s.ram.translate(address)
	if err != nil {
		if s.debugEnabled {
			s.frameReadFaults++
		}
		return 0, false
	}
	lo, loPresent, err := s.ram.translate(address + 1)
	if err != nil {
		if s.debugEnabled {
			s.frameReadFaults++
		}
		return 0, false
	}
	if s.debugEnabled {
		s.frameVideoWords++
	}
	var value uint16
	if hiPresent {
		value |= uint16(s.ram.data[hi]) << 8
	}
	if loPresent {
		value |= uint16(s.ram.data[lo])
	}
	return value, true
}

func (s *Shifter) linearVideoRAM() ([]byte, bool) {
	if s.ram == nil || s.ram.base != 0 || s.ram.mmu != nil {
		return nil, false
	}
	return s.ram.data, true
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
		if value, handled := s.model.readByte(s, address); handled {
			return value
		}
		if isPaletteAddress(address) {
			index := (address - paletteBase) >> 1
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
		if s.model.writeByte(s, address, value) {
			return
		}
		if isPaletteAddress(address) {
			index := (address - paletteBase) >> 1
			current := s.palette[index]
			if address&1 == 0 {
				current = (current & 0x00FF) | uint16(value)<<8
			} else {
				current = (current & 0xFF00) | uint16(value)
			}
			s.palette[index] = current & s.model.paletteMask()
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
		baseHigh:   s.baseHigh,
		baseMid:    s.baseMid,
		baseLow:    s.baseLow,
		lineOffset: s.lineOffset,
		fineScroll: s.fineScroll,
		palette:    s.palette,
	}
}

func (s *Shifter) lineState(line int) shifterLineState {
	if line >= len(s.lineStates) {
		return s.snapshotLineState()
	}
	return s.lineStates[line]
}

func (s *Shifter) frameLineIndex(cycles uint64) int {
	return frameIndex(cycles, s.frameCycles(), len(s.lineStates))
}

func (s *Shifter) frameSlotIndex(cycles uint64) int {
	return frameIndex(cycles, s.frameCycles(), len(s.slotSyncModes))
}

func frameIndex(cycles, frameCycles uint64, count int) int {
	// Map frame cycle position to a [0,count] segment index.
	if count == 0 || frameCycles == 0 {
		return 0
	}
	index := int(cycles * uint64(count) / frameCycles)
	if index > count {
		return count
	}
	return index
}

func (s *Shifter) slotSyncMode(line, segment uint32) byte {
	if line >= uint32(len(s.lineStates)) || segment >= shifterRasterSegments {
		return s.syncMode
	}
	idx := line*shifterRasterSegments + segment
	if idx >= uint32(len(s.slotSyncModes)) {
		return s.syncMode
	}
	return s.slotSyncModes[idx]
}

func (s *Shifter) applyBlankSegments() {
	if len(s.slotSyncModes) == 0 || s.width == 0 || s.height == 0 {
		return
	}
	fb := s.framebuffer
	debug := s.debugEnabled
	model := s.model
	var blanked uint64
	mono := s.resolution&0x3 == 2
	monoBorder := [4]byte{0xFF, 0xFF, 0xFF, 0xFF}
	for y := range s.height {
		rowOffset := y * s.width * 4
		row := fb[rowOffset : rowOffset+s.width*4]
		borderRGBA := monoBorder
		if !mono {
			borderColor := s.lineState(y).palette[0]
			borderRGBA = colorToRGBA(borderColor, model)
		}
		blankStart := -1
		for seg := range shifterRasterSegments {
			// BLANK_DISPLAY is tracked per raster segment.
			blankedSegment := s.slotSyncMode(uint32(y), uint32(seg))&shifterSyncBlankDisplayBit != 0
			if blankedSegment {
				if blankStart < 0 {
					blankStart = seg
				}
				continue
			}
			if blankStart >= 0 {
				x0, x1 := blankSegmentSpan(blankStart, seg, s.width)
				fillRowRGBA(row, x0, x1, borderRGBA)
				if debug {
					blanked += uint64(x1 - x0)
				}
				blankStart = -1
			}
		}
		if blankStart >= 0 {
			x0, x1 := blankSegmentSpan(blankStart, shifterRasterSegments, s.width)
			fillRowRGBA(row, x0, x1, borderRGBA)
			if debug {
				blanked += uint64(x1 - x0)
			}
		}
	}
	if debug {
		s.frameBlankPixels += blanked
	}
}

func blankSegmentSpan(startSegment, endSegment, width int) (x0, x1 int) {
	x0 = (startSegment * width) / shifterRasterSegments
	x1 = (endSegment * width) / shifterRasterSegments
	return x0, x1
}

func (s *Shifter) composeDisplayFrame() {
	s.displayOffsetX = 0
	s.displayOffsetY = 0
	s.displayWidth = s.width
	s.displayHeight = s.height
	s.displayViewportW = s.width
	s.displayViewportH = s.height

	if s.width == 0 || s.height == 0 {
		s.displayBuffer = s.displayBuffer[:0]
		return
	}

	yScale := 1
	if s.resolution&3 == 1 {
		// Medium-resolution output can be vertically stretched without changing
		// guest timing or address generation.
		yScale = s.cfg.MidResYScale
		if yScale < 1 {
			yScale = 1
		}
	}

	left, right, top, bottom := 0, 0, 0, 0
	if s.cfg.ColorMonitor && (s.resolution&3 < 2) {
		left, right, top, bottom = displayBorderForMode(s.resolution & 3)
	}

	outW := s.width + left + right
	activeH := s.height * yScale
	outH := activeH + top + bottom
	if outW == s.width && outH == s.height {
		s.displayBuffer = s.displayBuffer[:0]
		return
	}
	if outW == 0 || outH == 0 {
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
	model := s.model
	topRGBA := colorToRGBA(topColor, model)
	bottomRGBA := colorToRGBA(bottomColor, model)
	prevActiveSrcY := -1

	for y := range outH {
		row := s.displayBuffer[y*outW*4 : (y+1)*outW*4]
		switch {
		case y < top:
			fillRowRGBA(row, 0, outW, topRGBA)
		case y >= top+activeH:
			fillRowRGBA(row, 0, outW, bottomRGBA)
		default:
			srcY := (y - top) / yScale
			// For scaled medium mode, duplicate the previous active row.
			if srcY == prevActiveSrcY && y > 0 {
				prev := s.displayBuffer[(y-1)*outW*4 : y*outW*4]
				copy(row, prev)
				continue
			}
			borderColor := s.palette[0]
			if srcY < len(s.lineStates) {
				borderColor = s.lineStates[srcY].palette[0]
			}
			borderRGBA := colorToRGBA(borderColor, model)
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

func colorToRGBA(color uint16, model shifterModel) [4]byte {
	r, g, b := model.paletteColorChannels(color)
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
	// Geometric copy fill is faster than writing RGBA per pixel.
	for filled < len(segment) {
		copy(segment[filled:], segment[:filled])
		filled *= 2
	}
}

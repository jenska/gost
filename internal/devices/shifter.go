package devices

import (
	"runtime"
	"sort"
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
	// lineFetchBytes is the number of bytes fetched across a line's fetch
	// window; lineStrideBytes adds the STE line offset applied after it.
	lineFetchBytes(state shifterLineState, mode byte) uint32
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
	monoWordRGBA            [1 << 16][64]byte
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

	// High resolution fast path: expand one word into 16 monochrome RGBA
	// pixels, avoiding two table copies in the hot render loop.
	for value := range len(monoWordRGBA) {
		for pixel := range 16 {
			dst := pixel * 4
			if value&(1<<(15-pixel)) != 0 {
				monoWordRGBA[value][dst] = 0x00
				monoWordRGBA[value][dst+1] = 0x00
				monoWordRGBA[value][dst+2] = 0x00
				monoWordRGBA[value][dst+3] = 0xFF
				continue
			}
			monoWordRGBA[value][dst] = 0xFF
			monoWordRGBA[value][dst+1] = 0xFF
			monoWordRGBA[value][dst+2] = 0xFF
			monoWordRGBA[value][dst+3] = 0xFF
		}
	}
}

type shifterLineState struct {
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

	TotalRenderNanos int64
	TotalPixelsDrawn uint64
	TotalBlankPixels uint64
	TotalVideoWords  uint64
	TotalReadFaults  uint64

	FrameActive   bool
	FrameCyclePos uint64
	FrameCycles   uint64
	ScreenBase    uint32
	VideoAddress  uint32
}

type Shifter struct {
	cfg             *config.Config
	ram             *RAM
	model           shifterModel
	baseHigh        byte
	baseMid         byte
	baseLow         byte
	lineOffset      byte
	fineScroll      byte
	syncMode        byte
	resolution      byte
	palette         [paletteRegisterCt]uint16
	framebuffer     []byte
	width           int
	height          int
	videoCounter    uint32
	frameCyclePos   uint64
	frameResolution byte
	frameActive     bool
	frameScanlines  uint64
	// fetchLine is the next displayed line whose fetch has not started, and
	// nextFetch the video counter value that line will start from.
	fetchLine int
	nextFetch uint32
	// frameClock reports the CPU's position in the frame in machine cycles,
	// so register accesses resolve at the cycle they happen; scheduledPos is
	// how far AdvanceFrame has taken the frame.
	frameClock       func() uint64
	scheduledPos     uint64
	paletteEvents    []shifterPaletteEvent
	lineStates       []shifterLineState
	lineAddrs        []uint32
	renderStats      []shifterRenderStats
	renderWorkers    []chan shifterRenderTask
	renderDone       chan struct{}
	slotSyncModes    []byte
	frameHasBlank    bool
	lastRendered     uint64
	debugEnabled     bool
	debugStats       ShifterDebugStats
	framePixelsDrawn uint64
	frameBlankPixels uint64
	frameVideoWords  uint64
	frameReadFaults  uint64
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
	}
	return stats
}

func (s *Shifter) frameCycles() uint64 {
	if s.cfg == nil {
		return 0
	}
	return s.cfg.FrameCycles()
}

// AddressRange bounds the shifter registers; Contains decodes them sparsely.
func (s *Shifter) AddressRange() (uint32, uint32) {
	return shifterRegBaseHigh - 1, shifterRegFineScroll
}

func (s *Shifter) Contains(address uint32) bool {
	if isPaletteAddress(address) {
		return true
	}
	return s.model.containsRegister(address)
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
	s.frameResolution = 0
	s.frameActive = false
	s.frameScanlines = 0
	s.fetchLine = 0
	s.nextFetch = 0
	s.scheduledPos = 0
	s.paletteEvents = s.paletteEvents[:0]
	s.lineStates = s.lineStates[:0]
	s.lineAddrs = s.lineAddrs[:0]
	s.renderStats = s.renderStats[:0]
	s.slotSyncModes = s.slotSyncModes[:0]
	s.frameHasBlank = false
	s.lastRendered = 0
	s.debugStats = ShifterDebugStats{}
	s.framePixelsDrawn = 0
	s.frameBlankPixels = 0
	s.frameVideoWords = 0
	s.frameReadFaults = 0
	s.displayBuffer = nil
	s.displayWidth = 0
	s.displayHeight = 0
	s.displayOffsetX = 0
	s.displayOffsetY = 0
	s.displayViewportW = 0
	s.displayViewportH = 0
}

func (s *Shifter) Read(size cpu.Size, address uint32) (uint32, error) {
	s.syncToClock()
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
	s.syncToClock()
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
	s.frameResolution = mode
	width, height := dimensionsForResolution(mode)
	if width != s.width || height != s.height || len(s.framebuffer) != width*height*4 {
		s.width = width
		s.height = height
		s.framebuffer = make([]byte, width*height*4)
	}
	s.frameCyclePos = 0
	s.frameActive = true
	s.framePixelsDrawn = 0
	s.frameBlankPixels = 0
	s.frameVideoWords = 0
	s.frameReadFaults = 0

	lineCount := height
	if cap(s.lineStates) < lineCount {
		s.lineStates = make([]shifterLineState, lineCount)
	} else {
		s.lineStates = s.lineStates[:lineCount]
	}
	if cap(s.lineAddrs) < lineCount {
		s.lineAddrs = make([]uint32, lineCount)
	} else {
		s.lineAddrs = s.lineAddrs[:lineCount]
	}
	// The displayed lines are the first ones after VBL, on the raster the GLUE
	// (HBL) and the MFP Timer B event counter run on.
	s.frameScanlines = max(VideoTimingFor(s.cfg).Scanlines, uint64(lineCount))
	// The video counter is reloaded from the screen base only between frames,
	// so a base write made during a frame shows from the next frame on.
	s.fetchLine = 0
	s.nextFetch = s.model.screenBase(s)
	s.scheduledPos = 0
	s.paletteEvents = s.paletteEvents[:0]

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

// SetFrameClock gives the shifter the CPU's current position in the frame,
// in machine cycles since BeginFrame. Register accesses then take effect at
// that cycle rather than at the end of the step AdvanceFrame is called for.
func (s *Shifter) SetFrameClock(clock func() uint64) {
	s.frameClock = clock
}

func (s *Shifter) AdvanceFrame(cycles uint64) {
	s.scheduledPos += cycles
	s.advanceTo(s.scheduledPos)
}

// syncToClock brings the raster up to the CPU's position before a register
// access.
func (s *Shifter) syncToClock() {
	if s.frameActive && s.frameClock != nil {
		s.advanceTo(s.frameClock())
	}
}

// advanceTo moves the raster forward to frame cycle pos.
func (s *Shifter) advanceTo(pos uint64) {
	pos = min(pos, s.frameCycles())
	if !s.frameActive || len(s.lineStates) == 0 || pos <= s.frameCyclePos {
		return
	}
	prevPos := s.frameCyclePos
	s.frameCyclePos = pos
	// A line takes the registers and the video counter as they are when its
	// fetch starts, so a write made in the border before it (where HBL and
	// Timer B handlers run) shows on that line and later writes on the next.
	for s.fetchLine < len(s.lineStates) {
		if start, _ := s.lineFetchWindow(s.fetchLine); start >= s.frameCyclePos {
			break
		}
		s.startLineFetch()
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
	if s.syncMode&shifterSyncBlankDisplayBit != 0 {
		s.frameHasBlank = true
	}
}

func (s *Shifter) EndFrame() bool {
	if !s.frameActive {
		return false
	}
	s.advanceTo(s.frameCycles())

	var renderStart time.Time
	if s.debugEnabled {
		renderStart = time.Now()
	}

	s.renderFrame()

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
		s.debugStats.TotalRenderNanos += renderNanos
		s.debugStats.TotalPixelsDrawn += s.framePixelsDrawn
		s.debugStats.TotalBlankPixels += s.frameBlankPixels
		s.debugStats.TotalVideoWords += s.frameVideoWords
		s.debugStats.TotalReadFaults += s.frameReadFaults
	}
	s.composeDisplayFrame()
	s.frameActive = false
	return true
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

// shifterPaletteEvent is a palette write made while line was shown: pixels
// from x on use value for colour index.
type shifterPaletteEvent struct {
	line  int32
	x     int32
	index uint8
	value uint16
}

type shifterRenderStats struct {
	pixelsDrawn uint64
	videoWords  uint64
	readFaults  uint64
}

type shifterPaletteRGB struct {
	reds   [paletteRegisterCt]byte
	greens [paletteRegisterCt]byte
	blues  [paletteRegisterCt]byte
}

type shifterRenderJob struct {
	framebuffer []byte
	width       int
	height      int
	mode        byte
	model       shifterModel
	states      []shifterLineState
	lineAddrs   []uint32
	events      []shifterPaletteEvent
	stats       []shifterRenderStats
	videoData   []byte
	videoLen    uint32
	linearVideo bool
	ram         *RAM
	debug       bool
}

type shifterRenderTask struct {
	job         shifterRenderJob
	workerIndex int
	startY      int
	endY        int
	done        chan<- struct{}
}

func (s *Shifter) renderFrame() {
	mode := s.frameResolution & 3
	// Lines the frame never reached (no frame timing) take the current state.
	for s.fetchLine < len(s.lineStates) {
		s.startLineFetch()
	}
	workers := shifterRenderWorkers(s.height, mode)
	if cap(s.renderStats) < workers {
		s.renderStats = make([]shifterRenderStats, workers)
	} else {
		s.renderStats = s.renderStats[:workers]
	}
	for i := range s.renderStats {
		s.renderStats[i] = shifterRenderStats{}
	}

	videoData, linearVideo := s.linearVideoRAM()
	job := shifterRenderJob{
		framebuffer: s.framebuffer,
		width:       s.width,
		height:      s.height,
		mode:        mode,
		model:       s.model,
		states:      s.lineStates,
		lineAddrs:   s.lineAddrs,
		events:      s.paletteEvents,
		stats:       s.renderStats,
		videoData:   videoData,
		videoLen:    uint32(len(videoData)),
		linearVideo: linearVideo,
		ram:         s.ram,
		debug:       s.debugEnabled,
	}

	var stats shifterRenderStats
	if workers <= 1 {
		stats = job.renderRange(0, s.height)
	} else {
		stats = s.renderParallel(job, workers)
	}
	if s.debugEnabled {
		s.framePixelsDrawn += stats.pixelsDrawn
		s.frameVideoWords += stats.videoWords
		s.frameReadFaults += stats.readFaults
	}
	if len(s.lineAddrs) > 0 {
		s.videoCounter = s.nextFetch
	}
	s.applyBlankSegments()
}

func (s *Shifter) renderParallel(job shifterRenderJob, workers int) shifterRenderStats {
	s.ensureRenderWorkers(workers)
	dispatched := 0
	for worker := range workers {
		start := worker * job.height / workers
		end := (worker + 1) * job.height / workers
		if start == end {
			continue
		}
		s.renderWorkers[worker] <- shifterRenderTask{
			job:         job,
			workerIndex: worker,
			startY:      start,
			endY:        end,
			done:        s.renderDone,
		}
		dispatched++
	}
	for range dispatched {
		<-s.renderDone
	}

	var total shifterRenderStats
	for _, stat := range job.stats {
		total.pixelsDrawn += stat.pixelsDrawn
		total.videoWords += stat.videoWords
		total.readFaults += stat.readFaults
	}
	return total
}

func (s *Shifter) ensureRenderWorkers(count int) {
	if cap(s.renderDone) < count {
		s.renderDone = make(chan struct{}, count)
	}
	for len(s.renderWorkers) < count {
		work := make(chan shifterRenderTask)
		s.renderWorkers = append(s.renderWorkers, work)
		go shifterRenderWorker(work)
	}
}

func shifterRenderWorker(work <-chan shifterRenderTask) {
	for task := range work {
		task.job.stats[task.workerIndex] = task.job.renderRange(task.startY, task.endY)
		task.done <- struct{}{}
	}
}

func shifterRenderWorkers(height int, mode byte) int {
	if mode == 2 {
		return 1
	}
	if height < 96 {
		return 1
	}
	workers := min(runtime.GOMAXPROCS(0), height/32)
	if workers < 1 {
		return 1
	}
	return workers
}

// renderRange draws lines [startY, endY). Runs of plain lines go through the
// per-mode fast paths; a line with mid-line palette changes, or a scrolled
// monochrome line, is drawn pixel by pixel.
func (j shifterRenderJob) renderRange(startY, endY int) shifterRenderStats {
	var stats shifterRenderStats
	event := sort.Search(len(j.events), func(i int) bool { return int(j.events[i].line) >= startY })
	runStart := startY
	for y := startY; y < endY; y++ {
		eventEnd := event
		for eventEnd < len(j.events) && int(j.events[eventEnd].line) == y {
			eventEnd++
		}
		scrolledMono := j.mode == 2 && j.model.fineScroll(j.states[y]) != 0
		if eventEnd == event && !scrolledMono {
			continue
		}
		stats.add(j.renderFastRange(runStart, y))
		stats.add(j.renderLinePixels(y, j.events[event:eventEnd]))
		event = eventEnd
		runStart = y + 1
	}
	stats.add(j.renderFastRange(runStart, endY))
	return stats
}

func (j shifterRenderJob) renderFastRange(startY, endY int) shifterRenderStats {
	if startY >= endY {
		return shifterRenderStats{}
	}
	switch j.mode {
	case 0:
		return j.renderLowRange(startY, endY)
	case 1:
		return j.renderMediumRange(startY, endY)
	default:
		return j.renderHighRange(startY, endY)
	}
}

func (stats *shifterRenderStats) add(other shifterRenderStats) {
	stats.pixelsDrawn += other.pixelsDrawn
	stats.videoWords += other.videoWords
	stats.readFaults += other.readFaults
}

// renderLinePixels draws line y one pixel at a time, applying the palette
// writes made while it was shown at the pixels they happened at.
func (j shifterRenderJob) renderLinePixels(y int, events []shifterPaletteEvent) shifterRenderStats {
	var stats shifterRenderStats
	var indices [640]byte
	state := j.states[y]
	planes := 4 >> j.mode
	scroll := j.model.fineScroll(state)
	for group := range j.width/16 + (scroll+15)/16 {
		offset := j.lineAddrs[y] + uint32(group*planes*2)
		var words [4]uint16
		ok := true
		for plane := 0; plane < planes && ok; plane++ {
			words[plane], ok = j.videoWord(offset+uint32(plane*2), &stats)
		}
		if !ok {
			continue
		}
		for bit := range 16 {
			x := group*16 + bit - scroll
			if x < 0 || x >= j.width {
				continue
			}
			var index byte
			for plane := range planes {
				if words[plane]&(0x8000>>bit) != 0 {
					index |= 1 << plane
				}
			}
			indices[x] = index
		}
	}

	var palette shifterPaletteRGB
	if j.mode == 2 {
		palette.reds[0], palette.greens[0], palette.blues[0] = 0xFF, 0xFF, 0xFF
	} else {
		for i := range paletteRegisterCt {
			palette.reds[i], palette.greens[i], palette.blues[i] = j.model.paletteColorChannels(state.palette[i])
		}
	}
	row := j.framebuffer[y*j.width*4 : (y+1)*j.width*4]
	for x := range j.width {
		for len(events) > 0 && int(events[0].x) <= x {
			if j.mode != 2 {
				e := events[0]
				palette.reds[e.index], palette.greens[e.index], palette.blues[e.index] = j.model.paletteColorChannels(e.value)
			}
			events = events[1:]
		}
		index := indices[x]
		row[x*4] = palette.reds[index]
		row[x*4+1] = palette.greens[index]
		row[x*4+2] = palette.blues[index]
		row[x*4+3] = 0xFF
	}
	if j.debug {
		stats.pixelsDrawn += uint64(j.width)
	}
	return stats
}

// videoWord reads one big-endian word of video memory.
func (j shifterRenderJob) videoWord(address uint32, stats *shifterRenderStats) (uint16, bool) {
	if !j.linearVideo {
		return j.readVideoWord(address, stats)
	}
	if j.videoLen < 2 || address > j.videoLen-2 {
		if j.debug {
			stats.readFaults++
		}
		return 0, false
	}
	if j.debug {
		stats.videoWords++
	}
	return uint16(j.videoData[address])<<8 | uint16(j.videoData[address+1]), true
}

func (j shifterRenderJob) renderLowRange(startY, endY int) shifterRenderStats {
	var stats shifterRenderStats
	var palette shifterPaletteRGB
	var prevPalette [paletteRegisterCt]uint16
	paletteValid := false
	for y := startY; y < endY; y++ {
		lineState := j.states[y]
		lineAddr := j.lineAddrs[y]
		rowOffset := y * j.width * 4
		row := j.framebuffer[rowOffset : rowOffset+j.width*4]
		if !paletteValid || lineState.palette != prevPalette {
			for i := range paletteRegisterCt {
				palette.reds[i], palette.greens[i], palette.blues[i] = j.model.paletteColorChannels(lineState.palette[i])
			}
			prevPalette = lineState.palette
			paletteValid = true
		}
		scroll := j.model.fineScroll(lineState)
		groups := 20 + ((scroll + 15) / 16)
		for group := range groups {
			offset := lineAddr + uint32(group*8)
			var p0, p1, p2, p3 uint16
			if j.linearVideo {
				if j.videoLen < 8 || offset > j.videoLen-8 {
					if j.debug {
						stats.readFaults++
					}
					continue
				}
				idx := int(offset)
				p0 = uint16(j.videoData[idx])<<8 | uint16(j.videoData[idx+1])
				p1 = uint16(j.videoData[idx+2])<<8 | uint16(j.videoData[idx+3])
				p2 = uint16(j.videoData[idx+4])<<8 | uint16(j.videoData[idx+5])
				p3 = uint16(j.videoData[idx+6])<<8 | uint16(j.videoData[idx+7])
				if j.debug {
					stats.videoWords += 4
				}
			} else {
				var ok bool
				p0, ok = j.readVideoWord(offset, &stats)
				if !ok {
					continue
				}
				p1, ok = j.readVideoWord(offset+2, &stats)
				if !ok {
					continue
				}
				p2, ok = j.readVideoWord(offset+4, &stats)
				if !ok {
					continue
				}
				p3, ok = j.readVideoWord(offset+6, &stats)
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
					if x < 0 || x >= j.width {
						continue
					}
					dst := x * 4
					index := indices[pixel]
					row[dst] = palette.reds[index]
					row[dst+1] = palette.greens[index]
					row[dst+2] = palette.blues[index]
					row[dst+3] = 0xFF
				}
			}
			if j.debug {
				stats.pixelsDrawn += 16
			}
		}
	}
	return stats
}

func (j shifterRenderJob) renderMediumRange(startY, endY int) shifterRenderStats {
	var stats shifterRenderStats
	var palette shifterPaletteRGB
	var prevPalette [paletteRegisterCt]uint16
	paletteValid := false
	for y := startY; y < endY; y++ {
		lineState := j.states[y]
		lineAddr := j.lineAddrs[y]
		rowOffset := y * j.width * 4
		row := j.framebuffer[rowOffset : rowOffset+j.width*4]
		if !paletteValid || lineState.palette != prevPalette {
			for i := range paletteRegisterCt {
				palette.reds[i], palette.greens[i], palette.blues[i] = j.model.paletteColorChannels(lineState.palette[i])
			}
			prevPalette = lineState.palette
			paletteValid = true
		}
		scroll := j.model.fineScroll(lineState)
		groups := 40 + ((scroll + 15) / 16)
		for group := range groups {
			offset := lineAddr + uint32(group*4)
			var p0, p1 uint16
			if j.linearVideo {
				if j.videoLen < 4 || offset > j.videoLen-4 {
					if j.debug {
						stats.readFaults++
					}
					continue
				}
				idx := int(offset)
				p0 = uint16(j.videoData[idx])<<8 | uint16(j.videoData[idx+1])
				p1 = uint16(j.videoData[idx+2])<<8 | uint16(j.videoData[idx+3])
				if j.debug {
					stats.videoWords += 2
				}
			} else {
				var ok bool
				p0, ok = j.readVideoWord(offset, &stats)
				if !ok {
					continue
				}
				p1, ok = j.readVideoWord(offset+2, &stats)
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
					if x < 0 || x >= j.width {
						continue
					}
					dst := x * 4
					index := indices[pixel]
					row[dst] = palette.reds[index]
					row[dst+1] = palette.greens[index]
					row[dst+2] = palette.blues[index]
					row[dst+3] = 0xFF
				}
			}
			if j.debug {
				stats.pixelsDrawn += 16
			}
		}
	}
	return stats
}

func (j shifterRenderJob) renderHighRange(startY, endY int) shifterRenderStats {
	var stats shifterRenderStats
	for y := startY; y < endY; y++ {
		lineAddr := j.lineAddrs[y]
		rowOffset := y * j.width * 4
		if j.linearVideo {
			if j.videoLen < 80 || lineAddr > j.videoLen-80 {
				if j.debug {
					stats.readFaults++
				}
				continue
			}
			src := j.videoData[int(lineAddr) : int(lineAddr)+80]
			for group := range 40 {
				srcOffset := group * 2
				pixels := uint16(src[srcOffset])<<8 | uint16(src[srcOffset+1])
				dst := rowOffset + group*64
				copy(j.framebuffer[dst:dst+64], monoWordRGBA[pixels][:])
			}
			if j.debug {
				stats.videoWords += 40
				stats.pixelsDrawn += 640
			}
			continue
		}
		for group := range 40 {
			offset := lineAddr + uint32(group*2)
			pixels, ok := j.readVideoWord(offset, &stats)
			if !ok {
				continue
			}
			dst := rowOffset + group*64
			copy(j.framebuffer[dst:dst+64], monoWordRGBA[pixels][:])
			if j.debug {
				stats.pixelsDrawn += 16
			}
		}
	}
	return stats
}

func (j shifterRenderJob) readVideoWord(address uint32, stats *shifterRenderStats) (uint16, bool) {
	if j.ram == nil {
		if j.debug {
			stats.readFaults++
		}
		return 0, false
	}

	hi, hiPresent, err := j.ram.translate(address)
	if err != nil {
		if j.debug {
			stats.readFaults++
		}
		return 0, false
	}
	lo, loPresent, err := j.ram.translate(address + 1)
	if err != nil {
		if j.debug {
			stats.readFaults++
		}
		return 0, false
	}
	if j.debug {
		stats.videoWords++
	}
	var value uint16
	if hiPresent {
		value |= uint16(j.ram.data[hi]) << 8
	}
	if loPresent {
		value |= uint16(j.ram.data[lo])
	}
	return value, true
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
	case shifterRegVideoAddrLow:
		return byte(s.currentVideoAddress())
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
			s.recordPaletteChange(int(index))
		}
	}
}

func (s *Shifter) currentVideoAddress() uint32 {
	if s.frameActive {
		return s.liveVideoCounter()
	}
	if s.videoCounter != 0 {
		return s.videoCounter
	}
	return s.ScreenBase()
}

// lineInFetch reports the displayed line whose fetch window contains the
// current frame position, with that window.
func (s *Shifter) lineInFetch() (line int, start, end uint64, ok bool) {
	line = s.fetchLine - 1
	if !s.frameActive || line < 0 {
		return 0, 0, 0, false
	}
	start, end = s.lineFetchWindow(line)
	if s.frameCyclePos >= end {
		return 0, 0, 0, false
	}
	return line, start, end, true
}

// liveVideoCounter is the video counter at the current frame position: inside
// a line's fetch window it walks through that line, elsewhere it holds the
// address the next line starts from.
func (s *Shifter) liveVideoCounter() uint32 {
	line, start, end, ok := s.lineInFetch()
	if !ok {
		return s.nextFetch
	}
	// Derived from nextFetch rather than the line's start address so that it
	// follows a counter write made during this line's fetch.
	stride := s.model.lineStrideBytes(s.lineStates[line], s.frameResolution)
	return s.nextFetch - stride + s.lineBytesFetched(line, start, end)
}

// writeVideoCounterByte replaces one byte of the video counter (writable on
// the STE only). Inside a line's fetch window the shifter carries on from the
// new address, so the next line starts where that leaves off; outside one the
// next line starts at the new address. Between frames the counter is reloaded
// from the screen base before the next frame's first fetch.
func (s *Shifter) writeVideoCounterByte(shift uint, value byte) {
	counter := s.currentVideoAddress()
	counter = (counter&^(0xFF<<shift) | uint32(value)<<shift) &^ 1
	if !s.frameActive {
		s.videoCounter = counter
		return
	}
	if line, start, end, ok := s.lineInFetch(); ok {
		stride := s.model.lineStrideBytes(s.lineStates[line], s.frameResolution)
		s.nextFetch = counter + stride - s.lineBytesFetched(line, start, end)
		return
	}
	s.nextFetch = counter
}

// setLineOffset writes the STE line offset. The shifter adds it to the video
// counter when a line's fetch ends, so a write during the fetch already
// applies to that line.
func (s *Shifter) setLineOffset(value byte) {
	if line, _, _, ok := s.lineInFetch(); ok {
		s.nextFetch += (uint32(value) - uint32(s.lineStates[line].lineOffset)) * 2
		s.lineStates[line].lineOffset = value
	}
	s.lineOffset = value
}

// recordPaletteChange notes a palette write made while a line is shown, so
// the rest of that line is drawn with the new colour. A write outside a fetch
// window reaches the next line through its register snapshot.
func (s *Shifter) recordPaletteChange(index int) {
	line, start, end, ok := s.lineInFetch()
	if !ok {
		return
	}
	s.paletteEvents = append(s.paletteEvents, shifterPaletteEvent{
		line:  int32(line),
		x:     int32((s.frameCyclePos - start) * uint64(s.width) / (end - start)),
		index: uint8(index),
		value: s.palette[index],
	})
}

// startLineFetch begins the fetch of the next displayed line from the current
// registers and video counter.
func (s *Shifter) startLineFetch() {
	line := s.fetchLine
	s.lineStates[line] = s.snapshotLineState()
	s.lineAddrs[line] = s.nextFetch
	s.nextFetch += s.model.lineStrideBytes(s.lineStates[line], s.frameResolution)
	s.fetchLine++
}

// lineBytesFetched is how far the fetch of a line has got at the current
// frame position, given the line's fetch window.
func (s *Shifter) lineBytesFetched(line int, start, end uint64) uint32 {
	fetch := s.model.lineFetchBytes(s.lineStates[line], s.frameResolution)
	elapsed := min(s.frameCyclePos, end) - min(s.frameCyclePos, start)
	return uint32(elapsed*uint64(fetch)/(end-start)) &^ 1
}

// rasterTiming is the raster the current frame runs on.
func (s *Shifter) rasterTiming() VideoTiming {
	return VideoTiming{FrameCycles: s.frameCycles(), Scanlines: s.frameScanlines, Mono: s.frameResolution == 2}
}

// lineStartCycle is the frame cycle at which raster line line begins.
func (s *Shifter) lineStartCycle(line int) uint64 {
	return s.rasterTiming().LineStartCycle(uint64(line))
}

// lineFetchWindow is the span of frame cycles in which the shifter fetches a
// displayed line.
func (s *Shifter) lineFetchWindow(line int) (start, end uint64) {
	return s.rasterTiming().DisplayWindow(uint64(line))
}

func (s *Shifter) snapshotLineState() shifterLineState {
	return shifterLineState{
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

// frameSlotIndex maps a frame cycle position to the raster segment of the
// displayed line it falls in; positions after the last displayed line map to
// len(s.slotSyncModes).
func (s *Shifter) frameSlotIndex(cycles uint64) int {
	frameCycles := s.frameCycles()
	if frameCycles == 0 || s.frameScanlines == 0 {
		return 0
	}
	line := int(cycles * s.frameScanlines / frameCycles)
	if s.lineStartCycle(line+1) <= cycles {
		line++
	}
	if line >= len(s.lineStates) {
		return len(s.slotSyncModes)
	}
	start := s.lineStartCycle(line)
	length := s.lineStartCycle(line+1) - start
	segment := 0
	if length > 0 {
		segment = min(int((cycles-start)*shifterRasterSegments/length), shifterRasterSegments-1)
	}
	return line*shifterRasterSegments + segment
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
	if !s.frameHasBlank || len(s.slotSyncModes) == 0 || s.width == 0 || s.height == 0 {
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
		yScale = max(s.cfg.MidResYScale, 1)
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

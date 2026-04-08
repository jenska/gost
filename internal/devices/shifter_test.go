package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestShifterLowResolutionPixelConversion(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)

	if err := shifter.Write(1, 0xFF8201, 0x00); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(1, 0xFF8203, 0x00); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(1, 0xFF820D, 0x00); err != nil {
		t.Fatalf("write base low: %v", err)
	}
	if err := shifter.Write(2, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write palette: %v", err)
	}
	if err := ram.LoadAt(0, []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}); err != nil {
		t.Fatalf("load bitplanes: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected render to report a change")
	}
	framebuffer := shifter.FrameBuffer()
	if got := framebuffer[:4]; got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("unexpected first pixel RGBA: %v", got)
	}
}

func TestShifterMediumResolutionPixelConversion(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)

	if err := shifter.Write(1, 0xFF8260, 0x01); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	if err := shifter.Write(2, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write palette 1: %v", err)
	}
	if err := shifter.Write(2, paletteBase+4, 0x0070); err != nil {
		t.Fatalf("write palette 2: %v", err)
	}
	if err := shifter.Write(2, paletteBase+6, 0x0770); err != nil {
		t.Fatalf("write palette 3: %v", err)
	}
	if err := ram.LoadAt(0, []byte{0x80, 0x00, 0x00, 0x00}); err != nil {
		t.Fatalf("load medium bitplanes: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected render to report a change")
	}

	width, height := shifter.Dimensions()
	if width != 640 || height != 200 {
		t.Fatalf("unexpected dimensions: got %dx%d want 640x200", width, height)
	}

	framebuffer := shifter.FrameBuffer()
	first := framebuffer[:4]
	if first[0] != 255 || first[1] != 0 || first[2] != 0 || first[3] != 255 {
		t.Fatalf("unexpected first medium pixel RGBA: %v", first)
	}
	second := framebuffer[4:8]
	if second[0] != 0 || second[1] != 0 || second[2] != 0 || second[3] != 255 {
		t.Fatalf("unexpected second medium pixel RGBA: %v", second)
	}
}

func TestShifterHighResolutionMonochromeConversion(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)

	if err := shifter.Write(1, 0xFF8260, 0x02); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	if err := ram.LoadAt(0, []byte{0x80, 0x00}); err != nil {
		t.Fatalf("load mono pixels: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected render to report a change")
	}

	width, height := shifter.Dimensions()
	if width != 640 || height != 400 {
		t.Fatalf("unexpected dimensions: got %dx%d want 640x400", width, height)
	}

	framebuffer := shifter.FrameBuffer()
	first := framebuffer[:4]
	if first[0] != 0 || first[1] != 0 || first[2] != 0 || first[3] != 255 {
		t.Fatalf("unexpected first mono pixel RGBA: %v", first)
	}

	second := framebuffer[4:8]
	if second[0] != 255 || second[1] != 255 || second[2] != 255 || second[3] != 255 {
		t.Fatalf("unexpected second mono pixel RGBA: %v", second)
	}
}

func TestShifterReadsFramebufferThroughMMUTranslation(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	rom := NewROM([]byte{0, 0, 0, 0, 0, 0, 0, 0}, 0xFC0000)
	overlay := NewOverlayROM(rom, ram)
	config := NewMemoryConfig(overlay, ram.Size())
	ram.SetMemoryConfig(config)

	shifter := NewShifter(ram)
	if err := config.Write(1, memoryConfigBase+1, 0x0A); err != nil {
		t.Fatalf("write MMU config: %v", err)
	}
	if err := shifter.Write(1, 0xFF8201, 0x04); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(1, 0xFF8203, 0x00); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(2, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write palette: %v", err)
	}
	if err := ram.LoadAt(0x00020000, []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}); err != nil {
		t.Fatalf("load translated bitplanes: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected render to report a change")
	}
	framebuffer := shifter.FrameBuffer()
	if got := framebuffer[:4]; got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("unexpected translated first pixel RGBA: %v", got)
	}
}

func TestShifterWordWritesAtEvenAddressesUpdateScreenBase(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)

	if err := shifter.Write(2, shifterBase, 0x0012); err != nil {
		t.Fatalf("write base high pair: %v", err)
	}
	if err := shifter.Write(2, 0xFF8202, 0x0034); err != nil {
		t.Fatalf("write base mid pair: %v", err)
	}

	if got, want := shifter.ScreenBase(), uint32(0x123400); got != want {
		t.Fatalf("unexpected screen base: got %06x want %06x", got, want)
	}

	value, err := shifter.Read(2, shifterBase)
	if err != nil {
		t.Fatalf("read base high pair: %v", err)
	}
	if value != 0x0012 {
		t.Fatalf("unexpected base high pair readback: got %04x want 0012", value)
	}
}

func TestShifterSyncModeRegisterMasksToLowBits(t *testing.T) {
	shifter := NewShifter(NewRAM(0, 1024*1024))
	if err := shifter.Write(1, 0xFF820A, 0xFF); err != nil {
		t.Fatalf("write sync mode: %v", err)
	}

	value, err := shifter.Read(1, 0xFF820A)
	if err != nil {
		t.Fatalf("read sync mode: %v", err)
	}
	if got := byte(value); got != 0x03 {
		t.Fatalf("unexpected sync mode readback: got %02x want 03", got)
	}
}

func TestShifterVideoCounterReadbackFollowsRenderedFrame(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	if err := shifter.Write(1, 0xFF8201, 0x12); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(1, 0xFF8203, 0x34); err != nil {
		t.Fatalf("write base mid: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected render to report a change")
	}

	high, err := shifter.Read(1, 0xFF8205)
	if err != nil {
		t.Fatalf("read video counter high: %v", err)
	}
	mid, err := shifter.Read(1, 0xFF8207)
	if err != nil {
		t.Fatalf("read video counter mid: %v", err)
	}
	if byte(high) != 0x12 || byte(mid) != 0xB1 {
		t.Fatalf("unexpected video counter readback: high=%02x mid=%02x want 12/b1", byte(high), byte(mid))
	}
}

func TestShifterPaletteWritesMaskToSTColorDepth(t *testing.T) {
	shifter := NewShifter(NewRAM(0, 1024*1024))
	if err := shifter.Write(2, paletteBase, 0xFFFF); err != nil {
		t.Fatalf("write palette word: %v", err)
	}

	value, err := shifter.Read(2, paletteBase)
	if err != nil {
		t.Fatalf("read palette word: %v", err)
	}
	if value != 0x0777 {
		t.Fatalf("unexpected masked palette value: got %04x want 0777", value)
	}
}

func TestShifterMidFramePaletteChangeAffectsSubsequentScanlines(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)

	if err := shifter.Write(1, 0xFF8201, 0x00); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(1, 0xFF8203, 0x00); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(2, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write initial palette: %v", err)
	}

	// Set bitplane 0 so the first pixel on each line uses palette index 1.
	stride := 160
	frame := make([]byte, 200*stride)
	for line := 0; line < 200; line++ {
		frame[line*stride] = 0x80
	}
	if err := ram.LoadAt(0, frame); err != nil {
		t.Fatalf("seed frame data: %v", err)
	}

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles / 2)
	if err := shifter.Write(2, paletteBase+2, 0x0070); err != nil {
		t.Fatalf("write updated palette: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles - shifter.frameCycles/2)
	if !shifter.EndFrame() {
		t.Fatalf("expected end-frame render to report changes")
	}

	fb := shifter.FrameBuffer()
	firstLine := fb[:4]
	if firstLine[0] != 255 || firstLine[1] != 0 || firstLine[2] != 0 {
		t.Fatalf("unexpected first-line color after split: %v", firstLine)
	}
	lastOffset := ((200-1)*320 + 0) * 4
	lastLine := fb[lastOffset : lastOffset+4]
	if lastLine[0] != 0 || lastLine[1] != 255 || lastLine[2] != 0 {
		t.Fatalf("unexpected last-line color after split: %v", lastLine)
	}
}

func TestShifterRAMContentionAddsWaitStatesDuringActiveFetchWindow(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)
	ram.SetContentionSource(shifter)

	shifter.BeginFrame()
	if got := ram.WaitStates(cpu.Word, 0x000100); got == 0 {
		t.Fatalf("expected non-zero wait states at frame start during shifter fetch window")
	}

	shifter.AdvanceFrame(shifter.frameCycles / 2)
	if got := ram.WaitStates(cpu.Word, 0x000100); got == 0 {
		t.Fatalf("expected non-zero wait states while frame is active")
	}

	if !shifter.EndFrame() {
		t.Fatalf("expected frame finalization")
	}
	if got := ram.WaitStates(cpu.Word, 0x000100); got != 0 {
		t.Fatalf("expected no wait states after frame ends, got %d", got)
	}
}

func TestShifterRAMContentionDropsOutsideFetchWindow(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)
	ram.SetContentionSource(shifter)

	shifter.BeginFrame()
	lineCycles := shifter.frameCycles / 200
	if lineCycles == 0 {
		lineCycles = 1
	}
	shifter.AdvanceFrame((lineCycles * 9) / 10)

	if got := ram.WaitStates(cpu.Word, 0x000100); got != 0 {
		t.Fatalf("expected no wait states near end of scanline fetch period, got %d", got)
	}
}

func TestShifterMidFrameBlankSegmentsCreateHorizontalBorderBand(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)

	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed video frame: %v", err)
	}

	lineCycles := shifter.frameCycles / 200
	if lineCycles == 0 {
		lineCycles = 1
	}
	segCycles := lineCycles / shifterRasterSegments
	if segCycles == 0 {
		segCycles = 1
	}

	shifter.BeginFrame()
	shifter.AdvanceFrame(segCycles * 2)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, shifterSyncBlankDisplayBit); err != nil {
		t.Fatalf("blank display segment: %v", err)
	}
	shifter.AdvanceFrame(segCycles * 2)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, 0x00); err != nil {
		t.Fatalf("re-enable display segment: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles)
	if !shifter.EndFrame() {
		t.Fatalf("expected completed frame")
	}

	fb := shifter.FrameBuffer()
	left := fb[:4]
	if left[0] != 255 || left[1] != 0 || left[2] != 0 {
		t.Fatalf("expected left segment to remain active video, got %v", left)
	}
	midOff := (0*320 + 140) * 4
	mid := fb[midOff : midOff+4]
	if mid[0] != 0 || mid[1] != 0 || mid[2] != 0 {
		t.Fatalf("expected middle segment to be blank border color, got %v", mid)
	}
	rightOff := (0*320 + 220) * 4
	right := fb[rightOff : rightOff+4]
	if right[0] != 255 || right[1] != 0 || right[2] != 0 {
		t.Fatalf("expected right segment to return to active video, got %v", right)
	}
}

func TestShifterMidFrameBlankCanBlankLowerScreenHalf(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)

	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed video frame: %v", err)
	}

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles / 2)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, shifterSyncBlankDisplayBit); err != nil {
		t.Fatalf("disable display for lower half: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles)
	if !shifter.EndFrame() {
		t.Fatalf("expected completed frame")
	}

	fb := shifter.FrameBuffer()
	top := fb[(20*320+10)*4 : (20*320+10)*4+4]
	if top[0] != 255 || top[1] != 0 || top[2] != 0 {
		t.Fatalf("expected upper half to keep active video, got %v", top)
	}
	bottom := fb[(180*320+10)*4 : (180*320+10)*4+4]
	if bottom[0] != 0 || bottom[1] != 0 || bottom[2] != 0 {
		t.Fatalf("expected lower half to be blank border color, got %v", bottom)
	}
}

func TestShifterDebugStatsCaptureFrameMetrics(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)
	shifter.SetDebug(true)
	ram.SetContentionSource(shifter)

	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed video frame: %v", err)
	}

	shifter.BeginFrame()
	if got := ram.WaitStates(cpu.Word, 0x000100); got == 0 {
		t.Fatalf("expected contention wait states while frame is active")
	}
	shifter.AdvanceFrame(shifter.frameCycles)
	if !shifter.EndFrame() {
		t.Fatalf("expected completed frame")
	}

	stats := shifter.DebugStats()
	if stats.FramesRendered != 1 {
		t.Fatalf("expected one rendered frame in debug stats, got %d", stats.FramesRendered)
	}
	if stats.LastPixelsDrawn == 0 {
		t.Fatalf("expected non-zero pixel count in debug stats")
	}
	if stats.LastVideoWords == 0 {
		t.Fatalf("expected non-zero video word reads in debug stats")
	}
	if stats.LastWaitHits == 0 {
		t.Fatalf("expected non-zero wait-state hits in debug stats")
	}
	if stats.TotalPixelsDrawn < stats.LastPixelsDrawn {
		t.Fatalf("expected total pixel count to include last frame")
	}
}

func TestShifterColorModeFrameBorderVisibleWhenEnabled(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetColorBorderVisible(true)

	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed video frame: %v", err)
	}
	if !shifter.Render(1) {
		t.Fatalf("expected rendered frame")
	}

	if w, h := shifter.Dimensions(); w != 320 || h != 200 {
		t.Fatalf("unexpected visible dimensions: got %dx%d want 320x200", w, h)
	}
	left, right, top, bottom := displayBorderForMode(0)
	if w, h := shifter.DisplayDimensions(); w != 320+left+right || h != 200+top+bottom {
		t.Fatalf("unexpected display dimensions: got %dx%d", w, h)
	}
	vx, vy, vw, vh := shifter.DisplayViewport()
	if vx != left || vy != top || vw != 320 || vh != 200 {
		t.Fatalf("unexpected display viewport: got (%d,%d,%d,%d)", vx, vy, vw, vh)
	}

	fb := shifter.DisplayBuffer()
	displayW, _ := shifter.DisplayDimensions()
	corner := fb[:4]
	if corner[0] != 0 || corner[1] != 0 || corner[2] != 0 {
		t.Fatalf("expected top-left border pixel to use border color, got %v", corner)
	}
	topCenterOff := (vy*displayW + vx + 160) * 4
	topCenter := fb[topCenterOff : topCenterOff+4]
	if topCenter[0] != 255 || topCenter[1] != 0 || topCenter[2] != 0 {
		t.Fatalf("expected top active row to remain video, got %v", topCenter)
	}

	centerOff := ((vy+100)*displayW + vx + 160) * 4
	center := fb[centerOff : centerOff+4]
	if center[0] != 255 || center[1] != 0 || center[2] != 0 {
		t.Fatalf("expected center pixel to remain active video, got %v", center)
	}
}

func TestShifterMediumModeUsesWiderHorizontalBorder(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetColorBorderVisible(true)

	if err := shifter.Write(cpu.Byte, shifterRegResolution, 0x01); err != nil {
		t.Fatalf("set medium resolution: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1MediumResFrame()); err != nil {
		t.Fatalf("seed medium video frame: %v", err)
	}
	if !shifter.Render(1) {
		t.Fatalf("expected rendered frame")
	}

	if w, h := shifter.Dimensions(); w != 640 || h != 200 {
		t.Fatalf("unexpected visible dimensions: got %dx%d want 640x200", w, h)
	}
	left, right, top, bottom := displayBorderForMode(1)
	if w, h := shifter.DisplayDimensions(); w != 640+left+right || h != 200+top+bottom {
		t.Fatalf("unexpected display dimensions: got %dx%d", w, h)
	}
	vx, vy, vw, vh := shifter.DisplayViewport()
	if vx != left || vy != top || vw != 640 || vh != 200 {
		t.Fatalf("unexpected display viewport: got (%d,%d,%d,%d)", vx, vy, vw, vh)
	}
}

func TestShifterMediumModeHeightScalingAffectsDisplayViewportOnly(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetColorBorderVisible(true)
	shifter.SetMidResYScale(2)

	if err := shifter.Write(cpu.Byte, shifterRegResolution, 0x01); err != nil {
		t.Fatalf("set medium resolution: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1MediumResFrame()); err != nil {
		t.Fatalf("seed medium video frame: %v", err)
	}
	if !shifter.Render(1) {
		t.Fatalf("expected rendered frame")
	}

	if w, h := shifter.Dimensions(); w != 640 || h != 200 {
		t.Fatalf("unexpected visible dimensions: got %dx%d want 640x200", w, h)
	}
	left, right, top, bottom := displayBorderForMode(1)
	if w, h := shifter.DisplayDimensions(); w != 640+left+right || h != 200*2+top+bottom {
		t.Fatalf("unexpected display dimensions: got %dx%d", w, h)
	}
	vx, vy, vw, vh := shifter.DisplayViewport()
	if vx != left || vy != top || vw != 640 || vh != 400 {
		t.Fatalf("unexpected scaled display viewport: got (%d,%d,%d,%d)", vx, vy, vw, vh)
	}
}

func TestShifterMediumModeHeightScalingWorksWithoutColorBorder(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetColorBorderVisible(false)
	shifter.SetMidResYScale(2)

	if err := shifter.Write(cpu.Byte, shifterRegResolution, 0x01); err != nil {
		t.Fatalf("set medium resolution: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1MediumResFrame()); err != nil {
		t.Fatalf("seed medium video frame: %v", err)
	}
	if !shifter.Render(1) {
		t.Fatalf("expected rendered frame")
	}

	if w, h := shifter.Dimensions(); w != 640 || h != 200 {
		t.Fatalf("unexpected visible dimensions: got %dx%d want 640x200", w, h)
	}
	if w, h := shifter.DisplayDimensions(); w != 640 || h != 400 {
		t.Fatalf("unexpected display dimensions without border: got %dx%d want 640x400", w, h)
	}
	vx, vy, vw, vh := shifter.DisplayViewport()
	if vx != 0 || vy != 0 || vw != 640 || vh != 400 {
		t.Fatalf("unexpected display viewport without border: got (%d,%d,%d,%d)", vx, vy, vw, vh)
	}
}

func solidIndex1LowResFrame() []byte {
	const (
		lines  = 200
		stride = 160
	)
	frame := make([]byte, lines*stride)
	for y := 0; y < lines; y++ {
		line := frame[y*stride : (y+1)*stride]
		for group := 0; group < 20; group++ {
			offset := group * 8
			line[offset] = 0xFF
			line[offset+1] = 0xFF
		}
	}
	return frame
}

func solidIndex1MediumResFrame() []byte {
	const (
		lines  = 200
		stride = 160
	)
	frame := make([]byte, lines*stride)
	for y := 0; y < lines; y++ {
		line := frame[y*stride : (y+1)*stride]
		for group := 0; group < 40; group++ {
			offset := group * 4
			line[offset] = 0xFF
			line[offset+1] = 0xFF
		}
	}
	return frame
}

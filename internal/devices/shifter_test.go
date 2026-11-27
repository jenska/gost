package devices

import (
	"testing"

	"github.com/jenska/gost/internal/config"
	cpu "github.com/jenska/m68kemu"
)

func testShifterConfig(frameCycles uint64) *config.Config {
	return &config.Config{
		ClockHz:      frameCycles,
		FrameHz:      1,
		ColorMonitor: true,
	}
}

func TestShifterLowResolutionPixelConversion(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTShifter(&config.Config{}, ram)

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
	shifter := NewSTShifter(&config.Config{}, ram)

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
	shifter := NewSTShifter(&config.Config{}, ram)

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
	mcfg := NewMemoryConfig(overlay, ram.Size())
	ram.SetMemoryConfig(mcfg)

	shifter := NewSTShifter(&config.Config{}, ram)
	if err := mcfg.Write(1, memoryConfigBase+1, 0x0A); err != nil {
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
	shifter := NewSTShifter(&config.Config{}, ram)

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

func TestShifterSTELowScreenBaseRegisterAffectsScreenBase(t *testing.T) {
	shifter := NewSTEShifter(&config.Config{}, NewRAM(0, 1024*1024))

	if err := shifter.Write(cpu.Byte, shifterRegBaseHigh, 0x12); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseMid, 0x34); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseLow, 0x56); err != nil {
		t.Fatalf("write base low: %v", err)
	}

	if got, want := shifter.ScreenBase(), uint32(0x123456); got != want {
		t.Fatalf("unexpected STE screen base: got %06x want %06x", got, want)
	}

	value, err := shifter.Read(cpu.Byte, shifterRegBaseLow)
	if err != nil {
		t.Fatalf("read base low: %v", err)
	}
	if byte(value) != 0x56 {
		t.Fatalf("unexpected base low readback: got %02x want 56", byte(value))
	}
}

func TestShifterSTModeIgnoresSTELowScreenBaseRegister(t *testing.T) {
	shifter := NewSTShifter(&config.Config{}, NewRAM(0, 1024*1024))

	if err := shifter.Write(cpu.Byte, shifterRegBaseHigh, 0x12); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseMid, 0x34); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseLow, 0x56); err != nil {
		t.Fatalf("write base low: %v", err)
	}

	if got, want := shifter.ScreenBase(), uint32(0x123400); got != want {
		t.Fatalf("unexpected ST screen base: got %06x want %06x", got, want)
	}
}

func TestShifterSTEFineHorizontalScrollShiftsVisiblePixels(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTEShifter(&config.Config{}, ram)

	if err := shifter.Write(cpu.Byte, shifterRegBaseHigh, 0x00); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseMid, 0x00); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0F00); err != nil {
		t.Fatalf("write palette index1: %v", err)
	}
	// Alternating 1/0 pixels on plane 0.
	if err := ram.LoadAt(0, []byte{0xAA, 0xAA, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}); err != nil {
		t.Fatalf("load source line: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected initial render")
	}
	fb := shifter.FrameBuffer()
	first := fb[:4]
	if first[0] != 255 || first[1] != 0 || first[2] != 0 {
		t.Fatalf("expected first pixel to be red without scroll, got %v", first)
	}

	if err := shifter.Write(cpu.Byte, shifterRegFineScroll, 0x01); err != nil {
		t.Fatalf("write fine scroll: %v", err)
	}
	if !shifter.Render(2) {
		t.Fatalf("expected render after fine-scroll write")
	}
	fb = shifter.FrameBuffer()
	first = fb[:4]
	if first[0] != 0 || first[1] != 0 || first[2] != 0 {
		t.Fatalf("expected first pixel to shift to black with fine scroll, got %v", first)
	}
}

func TestShifterSTELineOffsetAffectsNextScanlineAddress(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTEShifter(&config.Config{}, ram)

	if err := shifter.Write(cpu.Byte, shifterRegBaseHigh, 0x00); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseMid, 0x00); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0F00); err != nil {
		t.Fatalf("write palette index1: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+4, 0x00F0); err != nil {
		t.Fatalf("write palette index2: %v", err)
	}
	// Line 1 when stride is 160: first pixel index1 (red).
	line1Default := []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := ram.LoadAt(160, line1Default); err != nil {
		t.Fatalf("seed default line1 data: %v", err)
	}
	// Line 1 when stride is 160 + 6 words: first pixel index2 (green).
	line1Offset := []byte{0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := ram.LoadAt(172, line1Offset); err != nil {
		t.Fatalf("seed offset line1 data: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected initial render")
	}
	fb := shifter.FrameBuffer()
	line1 := fb[(1*320+0)*4 : (1*320+0)*4+4]
	if line1[0] != 255 || line1[1] != 0 || line1[2] != 0 {
		t.Fatalf("expected line1 pixel to be red without line offset, got %v", line1)
	}

	if err := shifter.Write(cpu.Byte, shifterRegLineOffset, 0x06); err != nil {
		t.Fatalf("write line offset: %v", err)
	}
	if !shifter.Render(2) {
		t.Fatalf("expected render after line-offset write")
	}
	fb = shifter.FrameBuffer()
	line1 = fb[(1*320+0)*4 : (1*320+0)*4+4]
	if line1[0] != 0 || line1[1] != 255 || line1[2] != 0 {
		t.Fatalf("expected line1 pixel to be green with line offset, got %v", line1)
	}
}

func TestShifterSyncModeRegisterMasksToLowBits(t *testing.T) {
	shifter := NewSTShifter(&config.Config{}, NewRAM(0, 1024*1024))
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
	shifter := NewSTShifter(&config.Config{}, ram)
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
	shifter := NewSTShifter(&config.Config{}, NewRAM(0, 1024*1024))
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

func TestShifterSTEPalettePreservesFourthIntensityBit(t *testing.T) {
	shifter := NewSTEShifter(&config.Config{}, NewRAM(0, 1024*1024))

	if err := shifter.Write(cpu.Word, paletteBase, 0x0FFF); err != nil {
		t.Fatalf("write STE palette word: %v", err)
	}

	value, err := shifter.Read(cpu.Word, paletteBase)
	if err != nil {
		t.Fatalf("read STE palette word: %v", err)
	}
	if value != 0x0FFF {
		t.Fatalf("unexpected STE palette value: got %04x want 0fff", value)
	}
}

func TestShifterSTEPaletteRendersExtendedIntensityBit(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTEShifter(&config.Config{}, ram)

	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0800); err != nil {
		t.Fatalf("write STE palette entry: %v", err)
	}
	if err := ram.LoadAt(0, []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}); err != nil {
		t.Fatalf("load bitplanes: %v", err)
	}

	if !shifter.Render(1) {
		t.Fatalf("expected render to report a change")
	}

	pixel := shifter.FrameBuffer()[:4]
	if pixel[0] != 17 || pixel[1] != 0 || pixel[2] != 0 || pixel[3] != 255 {
		t.Fatalf("unexpected STE extended-intensity pixel: got %v want [17 0 0 255]", pixel)
	}
}

func TestShifterMidFramePaletteChangeAffectsSubsequentScanlines(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTShifter(testShifterConfig(16000), ram)

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
	for line := range 200 {
		frame[line*stride] = 0x80
	}
	if err := ram.LoadAt(0, frame); err != nil {
		t.Fatalf("seed frame data: %v", err)
	}

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles() / 2)
	if err := shifter.Write(2, paletteBase+2, 0x0070); err != nil {
		t.Fatalf("write updated palette: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles() - shifter.frameCycles()/2)
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

func TestShifterMidFrameScreenBaseWriteShowsFromNextFrame(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTShifter, ram)

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles() / 4)
	if err := shifter.Write(cpu.Byte, shifterRegBaseHigh, (splitScreenSecondBase>>16)&0xFF); err != nil {
		t.Fatalf("write switched base high: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegBaseMid, (splitScreenSecondBase>>8)&0xFF); err != nil {
		t.Fatalf("write switched base mid: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	if !shifter.EndFrame() {
		t.Fatalf("expected end-frame render to report changes")
	}
	fb := shifter.FrameBuffer()
	assertLowResPixel(t, fb, 20, 255, 0, 0, "upper line of the frame the base was written in")
	assertLowResPixel(t, fb, 180, 255, 0, 0, "lower line of the frame the base was written in")

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles())
	if !shifter.EndFrame() {
		t.Fatalf("expected second frame to render")
	}
	fb = shifter.FrameBuffer()
	assertLowResPixel(t, fb, 0, 0, 255, 0, "first line of the next frame")
	assertLowResPixel(t, fb, 180, 0, 255, 0, "lower line of the next frame")
}

func TestShifterSTEVideoCounterWriteSplitsScreen(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTEShifter, ram)

	shifter.BeginFrame()
	// Load the counter in the border before line 100, as an HBL or Timer B
	// handler would.
	shifter.AdvanceFrame(shifter.lineStartCycle(100))
	writeVideoCounter(t, shifter, splitScreenSecondBase)
	shifter.AdvanceFrame(shifter.frameCycles())
	if !shifter.EndFrame() {
		t.Fatalf("expected end-frame render to report changes")
	}
	fb := shifter.FrameBuffer()
	assertLowResPixel(t, fb, 99, 255, 0, 0, "line before the counter write")
	assertLowResPixel(t, fb, 100, 0, 255, 0, "line after the counter write")
	assertLowResPixel(t, fb, 199, 0, 255, 0, "last line")
	if got, want := shifter.lineAddrs[101], uint32(splitScreenSecondBase+160); got != want {
		t.Fatalf("line after the split starts at %06x, want %06x", got, want)
	}

	// The next frame starts from the screen base again.
	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()
	assertLowResPixel(t, shifter.FrameBuffer(), 180, 255, 0, 0, "next frame")
}

func TestShifterSTEVideoCounterWriteDuringFetchContinuesFromNewAddress(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTEShifter, ram)

	shifter.BeginFrame()
	start, end := shifter.lineFetchWindow(50)
	shifter.AdvanceFrame((start + end) / 2)
	fetched := shifter.lineBytesFetched(50, start, end)
	if fetched == 0 || fetched >= 160 {
		t.Fatalf("expected a partly fetched line, got %d bytes", fetched)
	}
	writeVideoCounter(t, shifter, splitScreenSecondBase)
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()

	if got, want := shifter.lineAddrs[50], uint32(50*160); got != want {
		t.Fatalf("line 50 starts at %06x, want %06x", got, want)
	}
	if got, want := shifter.lineAddrs[51], splitScreenSecondBase+160-fetched; got != want {
		t.Fatalf("line 51 starts at %06x, want %06x", got, want)
	}
}

func TestShifterSTIgnoresVideoCounterWrites(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTShifter, ram)

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.lineStartCycle(100))
	writeVideoCounter(t, shifter, splitScreenSecondBase)
	if got, want := readVideoCounter(t, shifter), uint32(100*160); got != want {
		t.Fatalf("video counter after ST write: got %06x want %06x", got, want)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()
	assertLowResPixel(t, shifter.FrameBuffer(), 180, 255, 0, 0, "line after ignored write")
}

func TestShifterVideoCounterReadbackTracksRaster(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTShifter, ram)
	const base = 0x010000
	if err := shifter.Write(cpu.Byte, shifterRegBaseHigh, base>>16); err != nil {
		t.Fatalf("write base high: %v", err)
	}

	shifter.BeginFrame()
	if got := readVideoCounter(t, shifter); got != base {
		t.Fatalf("counter at frame start: got %06x want %06x", got, base)
	}
	shifter.AdvanceFrame(shifter.lineStartCycle(100))
	if got, want := readVideoCounter(t, shifter), uint32(base+100*160); got != want {
		t.Fatalf("counter before line 100: got %06x want %06x", got, want)
	}

	start, end := shifter.lineFetchWindow(100)
	shifter.AdvanceFrame((start+end)/2 - shifter.frameCyclePos)
	mid := readVideoCounter(t, shifter)
	if mid <= base+100*160 || mid >= base+101*160 || mid&1 != 0 {
		t.Fatalf("counter inside line 100 fetch: got %06x, want an even address within the line", mid)
	}

	shifter.AdvanceFrame(end - shifter.frameCyclePos)
	if got, want := readVideoCounter(t, shifter), uint32(base+101*160); got != want {
		t.Fatalf("counter after line 100 fetch: got %06x want %06x", got, want)
	}

	shifter.AdvanceFrame(shifter.lineStartCycle(250) - shifter.frameCyclePos)
	if got, want := readVideoCounter(t, shifter), uint32(base+200*160); got != want {
		t.Fatalf("counter in the lower border: got %06x want %06x", got, want)
	}
}

func TestShifterLineTakesRegistersWrittenBeforeItsFetch(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTShifter, ram)

	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.lineStartCycle(100))
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0007); err != nil {
		t.Fatalf("write palette: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()

	fb := shifter.FrameBuffer()
	assertLowResPixel(t, fb, 99, 255, 0, 0, "line before the palette write")
	assertLowResPixel(t, fb, 100, 0, 0, 255, "line after the palette write")
}

func TestShifterFrameClockPlacesPaletteWriteWithinLine(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTShifter, ram)
	var now uint64
	shifter.SetFrameClock(func() uint64 { return now })

	// One AdvanceFrame step covers the whole frame, as a CPU quantum would
	// cover several lines; the clock places the write in the middle of line 10.
	shifter.BeginFrame()
	start, end := shifter.lineFetchWindow(10)
	now = (start + end) / 2
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x000F); err != nil {
		t.Fatalf("write palette: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()

	fb := shifter.FrameBuffer()
	pixel := func(x, y int) []byte { return fb[(y*320+x)*4 : (y*320+x)*4+3] }
	if got := pixel(100, 10); got[0] != 255 || got[2] != 0 {
		t.Fatalf("left half of line 10: got %v, want red", got)
	}
	if got := pixel(220, 10); got[0] != 0 || got[2] != 255 {
		t.Fatalf("right half of line 10: got %v, want blue", got)
	}
	if got := pixel(0, 11); got[0] != 0 || got[2] != 255 {
		t.Fatalf("line 11: got %v, want blue", got)
	}
	if got := pixel(319, 9); got[0] != 255 || got[2] != 0 {
		t.Fatalf("line 9: got %v, want red", got)
	}
}

func TestShifterFrameClockResolvesVideoCounterAtAccessCycle(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTShifter, ram)
	var now uint64
	shifter.SetFrameClock(func() uint64 { return now })

	shifter.BeginFrame()
	_, end := shifter.lineFetchWindow(41)
	now = end
	if got, want := readVideoCounter(t, shifter), uint32(42*160); got != want {
		t.Fatalf("counter after line 41: got %06x want %06x", got, want)
	}
	if shifter.frameCyclePos != now {
		t.Fatalf("raster at %d, want the access cycle %d", shifter.frameCyclePos, now)
	}
	// AdvanceFrame never moves the raster back behind the CPU.
	shifter.AdvanceFrame(10)
	if shifter.frameCyclePos != now {
		t.Fatalf("raster moved to %d after a short AdvanceFrame, want %d", shifter.frameCyclePos, now)
	}
}

func TestShifterSTELineOffsetWriteDuringFetchAppliesToThatLine(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := newSplitScreenShifter(t, NewSTEShifter, ram)

	shifter.BeginFrame()
	start, end := shifter.lineFetchWindow(20)
	shifter.AdvanceFrame((start + end) / 2)
	if err := shifter.Write(cpu.Byte, shifterRegLineOffset, 4); err != nil {
		t.Fatalf("write line offset: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()

	if got, want := shifter.lineAddrs[20], uint32(20*160); got != want {
		t.Fatalf("line 20 starts at %06x, want %06x", got, want)
	}
	if got, want := shifter.lineAddrs[21], uint32(21*160+8); got != want {
		t.Fatalf("line 21 starts at %06x, want %06x", got, want)
	}
	if got, want := shifter.lineAddrs[22], uint32(22*160+16); got != want {
		t.Fatalf("line 22 starts at %06x, want %06x", got, want)
	}
}

func TestShifterSTEHorizontalScrollPrefetchesOneWordPerPlane(t *testing.T) {
	for _, tc := range []struct {
		name       string
		resolution uint32
		stride     uint32
	}{{"low", 0, 168}, {"medium", 1, 164}, {"high", 2, 82}} {
		t.Run(tc.name, func(t *testing.T) {
			ram := NewRAM(0, 1024*1024)
			shifter := NewSTEShifter(testShifterConfig(defaultShifterClockHz/defaultShifterFrameHz), ram)
			if err := shifter.Write(cpu.Byte, shifterRegResolution, tc.resolution); err != nil {
				t.Fatalf("write resolution: %v", err)
			}
			if err := shifter.Write(cpu.Byte, shifterRegFineScroll, 3); err != nil {
				t.Fatalf("write fine scroll: %v", err)
			}
			shifter.BeginFrame()
			shifter.AdvanceFrame(shifter.frameCycles())
			shifter.EndFrame()
			if got := shifter.lineAddrs[1]; got != tc.stride {
				t.Fatalf("line 1 starts at %d, want %d", got, tc.stride)
			}
			if got, want := shifter.videoCounter, uint32(len(shifter.lineAddrs))*tc.stride; got != want {
				t.Fatalf("counter after frame: got %d want %d", got, want)
			}
		})
	}
}

func TestShifterSTEHighResolutionHorizontalScrollShiftsPixels(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	cfg := testShifterConfig(defaultShifterClockHz / defaultShifterFrameHz)
	cfg.ColorMonitor = false
	shifter := NewSTEShifter(cfg, ram)
	if err := shifter.Write(cpu.Byte, shifterRegResolution, 2); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegFineScroll, 4); err != nil {
		t.Fatalf("write fine scroll: %v", err)
	}
	// First word of line 0: only pixel 4 set, which a scroll of 4 moves to x=0.
	if err := ram.LoadAt(0, []byte{0x08, 0x00}); err != nil {
		t.Fatalf("seed line: %v", err)
	}
	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles())
	shifter.EndFrame()

	fb := shifter.FrameBuffer()
	if got := fb[0]; got != 0 {
		t.Fatalf("pixel 0: got %d, want black", got)
	}
	if got := fb[4*4]; got != 0xFF {
		t.Fatalf("pixel 4: got %d, want white", got)
	}
}

const splitScreenSecondBase = 0x008000

// newSplitScreenShifter returns a shifter on real 50 Hz frame timing, showing a
// solid red frame at address 0 and a solid green one at splitScreenSecondBase.
func newSplitScreenShifter(t *testing.T, newShifter func(*config.Config, *RAM) *Shifter, ram *RAM) *Shifter {
	t.Helper()
	shifter := newShifter(testShifterConfig(defaultShifterClockHz/defaultShifterFrameHz), ram)
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0F00); err != nil {
		t.Fatalf("write red palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+4, 0x00F0); err != nil {
		t.Fatalf("write green palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed first frame: %v", err)
	}
	if err := ram.LoadAt(splitScreenSecondBase, solidIndex2LowResFrame()); err != nil {
		t.Fatalf("seed second frame: %v", err)
	}
	return shifter
}

func writeVideoCounter(t *testing.T, shifter *Shifter, address uint32) {
	t.Helper()
	for _, reg := range []struct {
		addr  uint32
		shift uint
	}{{shifterRegVideoAddrHigh, 16}, {shifterRegVideoAddrMid, 8}, {shifterRegVideoAddrLow, 0}} {
		if err := shifter.Write(cpu.Byte, reg.addr, (address>>reg.shift)&0xFF); err != nil {
			t.Fatalf("write video counter %06x: %v", reg.addr, err)
		}
	}
}

func readVideoCounter(t *testing.T, shifter *Shifter) uint32 {
	t.Helper()
	var counter uint32
	for _, addr := range []uint32{shifterRegVideoAddrHigh, shifterRegVideoAddrMid, shifterRegVideoAddrLow} {
		value, err := shifter.Read(cpu.Byte, addr)
		if err != nil {
			t.Fatalf("read video counter %06x: %v", addr, err)
		}
		counter = counter<<8 | value
	}
	return counter
}

func assertLowResPixel(t *testing.T, fb []byte, line int, r, g, b byte, what string) {
	t.Helper()
	offset := line * 320 * 4
	if got := fb[offset : offset+3]; got[0] != r || got[1] != g || got[2] != b {
		t.Fatalf("%s (line %d): got %v want [%d %d %d]", what, line, got, r, g, b)
	}
}

// rasterSegmentCycle is the first frame cycle inside the given raster segment
// of a displayed line.
func rasterSegmentCycle(shifter *Shifter, line, segment int) uint64 {
	start := shifter.lineStartCycle(line)
	length := shifter.lineStartCycle(line+1) - start
	return start + (length*uint64(segment)+shifterRasterSegments-1)/shifterRasterSegments
}

func TestShifterMidFrameBlankSegmentsCreateHorizontalBorderBand(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTShifter(testShifterConfig(defaultShifterClockHz/defaultShifterFrameHz), ram)

	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed video frame: %v", err)
	}

	// A sync mode write applies to the raster segments the frame then advances
	// through: blank during segments 2 and 3 of the first line.
	shifter.BeginFrame()
	shifter.AdvanceFrame(rasterSegmentCycle(shifter, 0, 1))
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, shifterSyncBlankDisplayBit); err != nil {
		t.Fatalf("blank display segment: %v", err)
	}
	shifter.AdvanceFrame(rasterSegmentCycle(shifter, 0, 3) - shifter.frameCyclePos)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, 0x00); err != nil {
		t.Fatalf("re-enable display segment: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	if !shifter.EndFrame() {
		t.Fatalf("expected completed frame")
	}

	fb := shifter.FrameBuffer()
	left := fb[:4]
	if left[0] != 255 || left[1] != 0 || left[2] != 0 {
		t.Fatalf("expected left segment to remain active video, got %v", left)
	}
	x0, x1 := blankSegmentSpan(2, 4, 320)
	midX := (x0 + x1) / 2
	midOff := (0*320 + midX) * 4
	mid := fb[midOff : midOff+4]
	if mid[0] != 0 || mid[1] != 0 || mid[2] != 0 {
		t.Fatalf("expected middle segment to be blank border color, got %v", mid)
	}
	rightX := x1 + 5
	rightOff := (0*320 + rightX) * 4
	right := fb[rightOff : rightOff+4]
	if right[0] != 255 || right[1] != 0 || right[2] != 0 {
		t.Fatalf("expected right segment to return to active video, got %v", right)
	}
}

func TestShifterFineBlankPulseAffectsOnlyNarrowHorizontalSpan(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTShifter(testShifterConfig(defaultShifterClockHz/defaultShifterFrameHz), ram)

	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		t.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		t.Fatalf("write pixel palette: %v", err)
	}
	if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
		t.Fatalf("seed video frame: %v", err)
	}

	// Blank for raster segment 1 of the first line only.
	shifter.BeginFrame()
	shifter.AdvanceFrame(1)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, shifterSyncBlankDisplayBit); err != nil {
		t.Fatalf("blank display segment: %v", err)
	}
	shifter.AdvanceFrame(rasterSegmentCycle(shifter, 0, 1) - shifter.frameCyclePos)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, 0x00); err != nil {
		t.Fatalf("re-enable display segment: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles())
	if !shifter.EndFrame() {
		t.Fatalf("expected completed frame")
	}

	fb := shifter.FrameBuffer()
	narrowBlank := fb[(0*320+7)*4 : (0*320+7)*4+4]
	if narrowBlank[0] != 0 || narrowBlank[1] != 0 || narrowBlank[2] != 0 {
		t.Fatalf("expected narrow blank pulse near the left edge, got %v", narrowBlank)
	}
	outsidePulse := fb[(0*320+20)*4 : (0*320+20)*4+4]
	if outsidePulse[0] != 255 || outsidePulse[1] != 0 || outsidePulse[2] != 0 {
		t.Fatalf("expected blank pulse to stay narrow, got %v at x=20", outsidePulse)
	}
}

func TestShifterMidFrameBlankCanBlankLowerScreenHalf(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTShifter(testShifterConfig(defaultShifterClockHz/defaultShifterFrameHz), ram)

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
	shifter.AdvanceFrame(shifter.frameCycles() / 2)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, shifterSyncBlankDisplayBit); err != nil {
		t.Fatalf("disable display for lower half: %v", err)
	}
	shifter.AdvanceFrame(shifter.frameCycles() / 2)
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
	shifter := NewSTShifter(testShifterConfig(16000), ram)
	shifter.SetDebug(true)

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
	shifter.AdvanceFrame(shifter.frameCycles())
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
	if stats.TotalPixelsDrawn < stats.LastPixelsDrawn {
		t.Fatalf("expected total pixel count to include last frame")
	}
}

func TestShifterColorModeFrameBorderVisibleWhenEnabled(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	shifter := NewSTShifter(&config.Config{ColorMonitor: true}, ram)

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
	shifter := NewSTShifter(&config.Config{ColorMonitor: true}, ram)

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
	shifter := NewSTShifter(&config.Config{ColorMonitor: true, MidResYScale: 2}, ram)

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
	shifter := NewSTShifter(&config.Config{ColorMonitor: false, MidResYScale: 2}, ram)

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
	for y := range lines {
		line := frame[y*stride : (y+1)*stride]
		for group := range 20 {
			offset := group * 8
			line[offset] = 0xFF
			line[offset+1] = 0xFF
		}
	}
	return frame
}

func solidIndex2LowResFrame() []byte {
	const (
		lines  = 200
		stride = 160
	)
	frame := make([]byte, lines*stride)
	for y := range lines {
		line := frame[y*stride : (y+1)*stride]
		for group := range 20 {
			offset := group * 8
			line[offset+2] = 0xFF
			line[offset+3] = 0xFF
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
	for y := range lines {
		line := frame[y*stride : (y+1)*stride]
		for group := range 40 {
			offset := group * 4
			line[offset] = 0xFF
			line[offset+1] = 0xFF
		}
	}
	return frame
}

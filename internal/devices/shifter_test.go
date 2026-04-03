package devices

import "testing"

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

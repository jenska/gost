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

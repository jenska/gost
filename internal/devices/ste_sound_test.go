package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestAbsentSTESoundFaultsOnDMAWindow(t *testing.T) {
	sound := NewAbsentSTESound()

	if _, err := sound.Read(cpu.Byte, 0xFF8901); err == nil {
		t.Fatalf("expected byte read to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}

	if err := sound.Write(cpu.Word, 0xFF8900, 0x1234); err == nil {
		t.Fatalf("expected word write to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}
}

func TestSTESoundRegisterReadback(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	sound := NewSTESound(ram, 8_000_000)

	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8903, 0x12)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8905, 0x34)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8907, 0x57)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF890F, 0x16)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8911, 0x78)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8913, 0x9B)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8921, 0x83)
	mustWriteSTESound(t, sound, cpu.Word, 0xFF8922, 0x0ABC)
	mustWriteSTESound(t, sound, cpu.Word, 0xFF8924, 0x07FF)

	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8903); got != 0x12 {
		t.Fatalf("frame base high = %02x, want 12", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8905); got != 0x34 {
		t.Fatalf("frame base mid = %02x, want 34", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8907); got != 0x56 {
		t.Fatalf("frame base low = %02x, want 56", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF890F); got != 0x16 {
		t.Fatalf("frame end high = %02x, want 16", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8911); got != 0x78 {
		t.Fatalf("frame end mid = %02x, want 78", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8913); got != 0x9A {
		t.Fatalf("frame end low = %02x, want 9a", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8921); got != 0x83 {
		t.Fatalf("mode = %02x, want 83", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Word, 0xFF8922); got != 0x0ABC {
		t.Fatalf("microwire data = %04x, want 0abc", got)
	}
	if got := mustReadSTESound(t, sound, cpu.Word, 0xFF8924); got != 0x07FF {
		t.Fatalf("microwire mask = %04x, want 07ff", got)
	}
}

func TestSTESoundDMAMonoPlaybackStopsAtFrameEnd(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	if err := ram.LoadAt(0x1000, []byte{0x00, 0x40, 0x7F, 0x80}); err != nil {
		t.Fatalf("seed RAM: %v", err)
	}
	sound := NewSTESound(ram, 8_000_000)
	setSTESoundFrame(t, sound, 0x1000, 0x1004)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8921, 0x83) // mono, 50066 Hz
	mustWriteSTESound(t, sound, cpu.Word, 0xFF8900, 0x0001)

	sound.Advance(2000)
	samples := make([]float32, 64)
	n := sound.DrainMonoF32(samples)
	if n == 0 {
		t.Fatalf("expected DMA sound samples")
	}
	if !containsAudibleSample(samples[:n]) {
		t.Fatalf("expected non-zero PCM output")
	}
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8901); got != 0 {
		t.Fatalf("control should clear after one-shot playback, got %02x", got)
	}
}

func TestSTESoundDMARepeatLoopsFrame(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	if err := ram.LoadAt(0x2000, []byte{0x7F, 0x81}); err != nil {
		t.Fatalf("seed RAM: %v", err)
	}
	sound := NewSTESound(ram, 8_000_000)
	setSTESoundFrame(t, sound, 0x2000, 0x2002)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8921, 0x83) // mono, 50066 Hz
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8901, 0x03)

	sound.Advance(4000)
	if got := mustReadSTESound(t, sound, cpu.Byte, 0xFF8901); got != 0x03 {
		t.Fatalf("repeat playback control = %02x, want 03", got)
	}
	counter := mustReadSTESound(t, sound, cpu.Byte, 0xFF8909) << 16
	counter |= mustReadSTESound(t, sound, cpu.Byte, 0xFF890B) << 8
	counter |= mustReadSTESound(t, sound, cpu.Byte, 0xFF890D)
	if counter < 0x2000 || counter > 0x2002 {
		t.Fatalf("counter outside looping frame: %06x", counter)
	}
}

func TestSTESoundStereoPlaybackMixesLeftRightToMono(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	if err := ram.LoadAt(0x3000, []byte{0x7F, 0x81}); err != nil {
		t.Fatalf("seed RAM: %v", err)
	}
	sound := NewSTESound(ram, 8_000_000)
	setSTESoundFrame(t, sound, 0x3000, 0x3002)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8921, 0x03) // stereo, 50066 Hz
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8901, 0x01)

	sound.Advance(1000)
	samples := make([]float32, 16)
	n := sound.DrainMonoF32(samples)
	if n == 0 {
		t.Fatalf("expected stereo DMA samples")
	}
	for _, sample := range samples[:n] {
		if sample < -0.01 || sample > 0.01 {
			t.Fatalf("stereo pair should average near silence, got %.4f", sample)
		}
	}
}

func setSTESoundFrame(t *testing.T, sound *STESound, start, end uint32) {
	t.Helper()
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8903, start>>16)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8905, start>>8)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8907, start)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF890F, end>>16)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8911, end>>8)
	mustWriteSTESound(t, sound, cpu.Byte, 0xFF8913, end)
}

func mustReadSTESound(t *testing.T, sound *STESound, size cpu.Size, address uint32) uint32 {
	t.Helper()
	value, err := sound.Read(size, address)
	if err != nil {
		t.Fatalf("read %06x: %v", address, err)
	}
	return value
}

func mustWriteSTESound(t *testing.T, sound *STESound, size cpu.Size, address uint32, value uint32) {
	t.Helper()
	if err := sound.Write(size, address, value); err != nil {
		t.Fatalf("write %06x=%x: %v", address, value, err)
	}
}

func containsAudibleSample(samples []float32) bool {
	for _, sample := range samples {
		if sample > 0.001 || sample < -0.001 {
			return true
		}
	}
	return false
}

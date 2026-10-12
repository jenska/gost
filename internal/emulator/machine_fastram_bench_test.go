package emulator

import (
	"testing"

	"github.com/jenska/gost/internal/assets"
)

// benchmarkBootFrames steps a freshly booted EmuTOS machine for a fixed number
// of frames. Most of that work is the CPU interpreting TOS code out of ROM with
// data in low RAM, so it is a reasonable proxy for overall emulation throughput.
func benchmarkBootFrames(b *testing.B, frames int, disableFastMem bool) {
	b.Helper()
	rom := assets.DefaultROM()

	b.ReportAllocs()
	for b.Loop() {
		machine, err := newMachine(DefaultConfig(), rom, nil, machineOptions{noFastMemory: disableFastMem})
		if err != nil {
			b.Fatalf("newMachine: %v", err)
		}
		for frame := 0; frame < frames; frame++ {
			if _, err := machine.StepFrame(); err != nil {
				b.Fatalf("StepFrame %d: %v", frame, err)
			}
		}
	}
}

func BenchmarkBootFramesFastMem(b *testing.B) { benchmarkBootFrames(b, 120, false) }
func BenchmarkBootFramesBusOnly(b *testing.B) { benchmarkBootFrames(b, 120, true) }

package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

type shifterBenchmarkCase struct {
	name             string
	resolution       byte
	colorBorder      bool
	midResYScale     int
	syncMode         byte
	debug            bool
	enableContention bool
}

func BenchmarkShifterRender(b *testing.B) {
	cases := []shifterBenchmarkCase{
		{name: "low_res_active"},
		{name: "medium_res_active", resolution: 1},
		{name: "high_res_active", resolution: 2},
		{name: "low_res_blank_sync", syncMode: shifterSyncBlankDisplayBit},
		{name: "low_res_color_border", colorBorder: true},
		{name: "medium_res_color_border_scaled", resolution: 1, colorBorder: true, midResYScale: 2},
		{name: "low_res_debug_stats", debug: true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			shifter, _, bytesPerFrame := newBenchmarkShifter(b, tc)
			b.ReportAllocs()
			b.SetBytes(bytesPerFrame)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if !shifter.Render(uint64(i + 2)) {
					b.Fatalf("render unexpectedly reported no change at iteration %d", i)
				}
			}
		})
	}
}

func BenchmarkShifterFrameLifecycle(b *testing.B) {
	b.Run("low_res_midframe_blank_toggle", func(b *testing.B) {
		shifter, _, bytesPerFrame := newBenchmarkShifter(b, shifterBenchmarkCase{})
		if err := shifter.Write(cpu.Byte, shifterRegSyncMode, 0x00); err != nil {
			b.Fatalf("reset sync mode: %v", err)
		}

		warmLowResMidFrameBlankToggle(b, shifter)

		b.ReportAllocs()
		b.SetBytes(bytesPerFrame)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			runLowResMidFrameBlankToggle(b, shifter)
		}
	})

	b.Run("medium_res_midframe_palette_split", func(b *testing.B) {
		shifter, _, bytesPerFrame := newBenchmarkShifter(b, shifterBenchmarkCase{
			resolution:   1,
			colorBorder:  true,
			midResYScale: 2,
		})

		warmMediumResMidFramePaletteSplit(b, shifter)

		b.ReportAllocs()
		b.SetBytes(bytesPerFrame)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			runMediumResMidFramePaletteSplit(b, shifter, i)
		}
	})
}

func BenchmarkShifterRAMContention(b *testing.B) {
	shifter, ram, _ := newBenchmarkShifter(b, shifterBenchmarkCase{
		enableContention: true,
	})

	shifter.BeginFrame()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = ram.WaitStates(cpu.Word, 0x000100)
		shifter.AdvanceFrame(8)
		if shifter.frameCyclePos >= shifter.frameCycles {
			if !shifter.EndFrame() {
				b.Fatalf("expected completed frame at iteration %d", i)
			}
			shifter.BeginFrame()
		}
	}
}

func newBenchmarkShifter(b *testing.B, tc shifterBenchmarkCase) (*Shifter, *RAM, int64) {
	b.Helper()

	ram := NewRAM(0, 1024*1024)
	shifter := NewShifter(ram)
	shifter.SetTiming(8_000_000, 50)
	shifter.SetDebug(tc.debug)
	shifter.SetColorBorderVisible(tc.colorBorder)
	if tc.midResYScale > 0 {
		shifter.SetMidResYScale(tc.midResYScale)
	}
	if tc.enableContention {
		ram.SetContentionSource(shifter)
	}

	if err := shifter.Write(cpu.Byte, shifterRegResolution, uint32(tc.resolution)); err != nil {
		b.Fatalf("write resolution: %v", err)
	}
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, uint32(tc.syncMode)); err != nil {
		b.Fatalf("write sync mode: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase, 0x0000); err != nil {
		b.Fatalf("write border palette: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, 0x0700); err != nil {
		b.Fatalf("write pixel palette 1: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+4, 0x0070); err != nil {
		b.Fatalf("write pixel palette 2: %v", err)
	}
	if err := shifter.Write(cpu.Word, paletteBase+6, 0x0770); err != nil {
		b.Fatalf("write pixel palette 3: %v", err)
	}

	switch tc.resolution {
	case 0:
		if err := ram.LoadAt(0, solidIndex1LowResFrame()); err != nil {
			b.Fatalf("load low-resolution frame: %v", err)
		}
	case 1:
		if err := ram.LoadAt(0, solidIndex1MediumResFrame()); err != nil {
			b.Fatalf("load medium-resolution frame: %v", err)
		}
	case 2:
		if err := ram.LoadAt(0, solidMonoHighResFrame()); err != nil {
			b.Fatalf("load high-resolution frame: %v", err)
		}
	default:
		b.Fatalf("unsupported benchmark resolution %d", tc.resolution)
	}

	if !shifter.Render(1) {
		b.Fatalf("expected warmup render to report change")
	}
	displayW, displayH := shifter.DisplayDimensions()
	return shifter, ram, int64(displayW * displayH * 4)
}

func warmLowResMidFrameBlankToggle(b *testing.B, shifter *Shifter) {
	b.Helper()
	runLowResMidFrameBlankToggle(b, shifter)
}

func runLowResMidFrameBlankToggle(b *testing.B, shifter *Shifter) {
	b.Helper()
	shifter.BeginFrame()

	lineCycles := shifter.frameCycles / 200
	if lineCycles == 0 {
		lineCycles = 1
	}
	segCycles := lineCycles / shifterRasterSegments
	if segCycles == 0 {
		segCycles = 1
	}

	shifter.AdvanceFrame(segCycles * 2)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, shifterSyncBlankDisplayBit); err != nil {
		b.Fatalf("enable blank sync: %v", err)
	}
	shifter.AdvanceFrame(segCycles * 2)
	if err := shifter.Write(cpu.Byte, shifterRegSyncMode, 0x00); err != nil {
		b.Fatalf("disable blank sync: %v", err)
	}
	if shifter.frameCyclePos < shifter.frameCycles {
		shifter.AdvanceFrame(shifter.frameCycles - shifter.frameCyclePos)
	}
	if !shifter.EndFrame() {
		b.Fatalf("expected frame completion")
	}
}

func warmMediumResMidFramePaletteSplit(b *testing.B, shifter *Shifter) {
	b.Helper()
	runMediumResMidFramePaletteSplit(b, shifter, 0)
}

func runMediumResMidFramePaletteSplit(b *testing.B, shifter *Shifter, i int) {
	b.Helper()
	shifter.BeginFrame()
	shifter.AdvanceFrame(shifter.frameCycles / 2)

	color := uint16(0x0070)
	if i&1 == 1 {
		color = 0x0700
	}
	if err := shifter.Write(cpu.Word, paletteBase+2, uint32(color)); err != nil {
		b.Fatalf("mid-frame palette write: %v", err)
	}

	if shifter.frameCyclePos < shifter.frameCycles {
		shifter.AdvanceFrame(shifter.frameCycles - shifter.frameCyclePos)
	}
	if !shifter.EndFrame() {
		b.Fatalf("expected frame completion")
	}
}

func solidMonoHighResFrame() []byte {
	const (
		lines  = 400
		stride = 80
	)
	frame := make([]byte, lines*stride)
	for y := 0; y < lines; y++ {
		line := frame[y*stride : (y+1)*stride]
		for group := 0; group < 40; group++ {
			offset := group * 2
			line[offset] = 0xFF
			line[offset+1] = 0xFF
		}
	}
	return frame
}

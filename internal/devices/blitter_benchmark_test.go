package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

type blitterBenchmarkCase struct {
	name         string
	setup        func(*testing.B, *RAM, *Blitter)
	bytesPerBlit int64
	templateRegs [blitterSize]byte
}

func BenchmarkBlitterExecute(b *testing.B) {
	cases := []blitterBenchmarkCase{
		newBlitterCopyBenchmarkCase(),
		newBlitterHalftoneFillBenchmarkCase(),
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			ram := NewRAM(0, 1024*1024)
			blitter := NewBlitter(ram)
			tc.setup(b, ram, blitter)
			template := tc.templateRegs

			b.ReportAllocs()
			b.SetBytes(tc.bytesPerBlit)
			b.ResetTimer()

			for range b.N {
				blitter.regs = template
				blitter.execute()
			}
		})
	}
}

func newBlitterCopyBenchmarkCase() blitterBenchmarkCase {
	var regs [blitterSize]byte
	writeBySize(regs[:], 0x20, cpu.Word, 2)
	writeBySize(regs[:], 0x22, cpu.Word, 2)
	writeBySize(regs[:], 0x24, cpu.Long, 0x00010000)
	writeBySize(regs[:], 0x28, cpu.Word, 0xFFFF)
	writeBySize(regs[:], 0x2A, cpu.Word, 0xFFFF)
	writeBySize(regs[:], 0x2C, cpu.Word, 0xFFFF)
	writeBySize(regs[:], 0x2E, cpu.Word, 2)
	writeBySize(regs[:], 0x30, cpu.Word, 2)
	writeBySize(regs[:], 0x32, cpu.Long, 0x00020000)
	writeBySize(regs[:], 0x36, cpu.Word, 80)
	writeBySize(regs[:], 0x38, cpu.Word, 200)
	writeBySize(regs[:], 0x3A, cpu.Byte, 2)
	writeBySize(regs[:], 0x3B, cpu.Byte, 3)
	writeBySize(regs[:], 0x3C, cpu.Byte, blitterBusy)

	return blitterBenchmarkCase{
		name:         "copy_replace_80x200_words",
		bytesPerBlit: 80 * 200 * 2,
		templateRegs: regs,
		setup: func(b *testing.B, ram *RAM, _ *Blitter) {
			b.Helper()
			pattern := make([]byte, 80*200*2)
			for i := 0; i < len(pattern); i += 2 {
				pattern[i] = byte(i >> 1)
				pattern[i+1] = byte(0xFF - i)
			}
			if err := ram.LoadAt(0x00010000, pattern); err != nil {
				b.Fatalf("load blitter copy source: %v", err)
			}
		},
	}
}

func newBlitterHalftoneFillBenchmarkCase() blitterBenchmarkCase {
	var regs [blitterSize]byte
	for i := range 16 {
		writeBySize(regs[:], uint32(i*2), cpu.Word, 0xA55A)
	}
	writeBySize(regs[:], 0x28, cpu.Word, 0xFFFF)
	writeBySize(regs[:], 0x2A, cpu.Word, 0xFFFF)
	writeBySize(regs[:], 0x2C, cpu.Word, 0xFFFF)
	writeBySize(regs[:], 0x2E, cpu.Word, 2)
	writeBySize(regs[:], 0x30, cpu.Word, 160)
	writeBySize(regs[:], 0x32, cpu.Long, 0x00030000)
	writeBySize(regs[:], 0x36, cpu.Word, 80)
	writeBySize(regs[:], 0x38, cpu.Word, 200)
	writeBySize(regs[:], 0x3A, cpu.Byte, 1)
	writeBySize(regs[:], 0x3B, cpu.Byte, 3)
	writeBySize(regs[:], 0x3C, cpu.Byte, blitterBusy)

	return blitterBenchmarkCase{
		name:         "halftone_fill_80x200_words",
		bytesPerBlit: 80 * 200 * 2,
		templateRegs: regs,
		setup: func(b *testing.B, ram *RAM, _ *Blitter) {
			b.Helper()
			zeroes := make([]byte, 80*200*2)
			if err := ram.LoadAt(0x00030000, zeroes); err != nil {
				b.Fatalf("load blitter fill destination: %v", err)
			}
		},
	}
}

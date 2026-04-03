package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestBlitterStatusRegisterIsReadable(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	blitter := NewBlitter(ram)

	value, err := blitter.Read(cpu.Byte, blitterBase+0x3C)
	if err != nil {
		t.Fatalf("read status register: %v", err)
	}
	if value != 0 {
		t.Fatalf("unexpected initial status: got %02x want 00", value)
	}
}

func TestBlitterCopiesSingleWordSourceOnly(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	blitter := NewBlitter(ram)

	if err := ram.Write(cpu.Word, 0x000100, 0xA55A); err != nil {
		t.Fatalf("write source word: %v", err)
	}
	if err := ram.Write(cpu.Word, 0x000200, 0x0000); err != nil {
		t.Fatalf("write destination word: %v", err)
	}

	mustWriteBlitter(t, blitter, 0x20, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x22, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x24, cpu.Long, 0x00000100)
	mustWriteBlitter(t, blitter, 0x28, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2A, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2C, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2E, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x30, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x32, cpu.Long, 0x00000200)
	mustWriteBlitter(t, blitter, 0x36, cpu.Word, 1)
	mustWriteBlitter(t, blitter, 0x38, cpu.Word, 1)
	mustWriteBlitter(t, blitter, 0x3A, cpu.Byte, 2)
	mustWriteBlitter(t, blitter, 0x3B, cpu.Byte, 3)
	mustWriteBlitter(t, blitter, 0x3D, cpu.Byte, 0)
	mustWriteBlitter(t, blitter, 0x3C, cpu.Byte, blitterBusy)

	value, err := ram.Read(cpu.Word, 0x000200)
	if err != nil {
		t.Fatalf("read copied destination word: %v", err)
	}
	if value != 0xA55A {
		t.Fatalf("unexpected copied destination word: got %04x want a55a", value)
	}

	status, err := blitter.Read(cpu.Byte, blitterBase+0x3C)
	if err != nil {
		t.Fatalf("read status after blit: %v", err)
	}
	if status&blitterBusy != 0 {
		t.Fatalf("expected BUSY to clear after software blit, got %02x", status)
	}
}

func TestBlitterAppliesEndMask(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	blitter := NewBlitter(ram)

	if err := ram.Write(cpu.Word, 0x000100, 0xFFFF); err != nil {
		t.Fatalf("write source word: %v", err)
	}
	if err := ram.Write(cpu.Word, 0x000200, 0x1234); err != nil {
		t.Fatalf("write destination word: %v", err)
	}

	mustWriteBlitter(t, blitter, 0x20, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x22, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x24, cpu.Long, 0x00000100)
	mustWriteBlitter(t, blitter, 0x28, cpu.Word, 0x00FF)
	mustWriteBlitter(t, blitter, 0x2A, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2C, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2E, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x30, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x32, cpu.Long, 0x00000200)
	mustWriteBlitter(t, blitter, 0x36, cpu.Word, 1)
	mustWriteBlitter(t, blitter, 0x38, cpu.Word, 1)
	mustWriteBlitter(t, blitter, 0x3A, cpu.Byte, 2)
	mustWriteBlitter(t, blitter, 0x3B, cpu.Byte, 3)
	mustWriteBlitter(t, blitter, 0x3D, cpu.Byte, 0)
	mustWriteBlitter(t, blitter, 0x3C, cpu.Byte, blitterBusy)

	value, err := ram.Read(cpu.Word, 0x000200)
	if err != nil {
		t.Fatalf("read masked destination word: %v", err)
	}
	if value != 0x12FF {
		t.Fatalf("unexpected masked destination word: got %04x want 12ff", value)
	}
}

func TestBlitterHalftoneOnlyFillUsesLineNumber(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	blitter := NewBlitter(ram)

	mustWriteBlitter(t, blitter, 0x02, cpu.Word, 0xF0F0)
	mustWriteBlitter(t, blitter, 0x04, cpu.Word, 0x0F0F)
	if err := ram.Write(cpu.Word, 0x000200, 0x0000); err != nil {
		t.Fatalf("write destination word: %v", err)
	}
	if err := ram.Write(cpu.Word, 0x000210, 0x0000); err != nil {
		t.Fatalf("write destination word row 2: %v", err)
	}

	mustWriteBlitter(t, blitter, 0x28, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2A, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2C, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2E, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x30, cpu.Word, 0x0010)
	mustWriteBlitter(t, blitter, 0x32, cpu.Long, 0x00000200)
	mustWriteBlitter(t, blitter, 0x36, cpu.Word, 1)
	mustWriteBlitter(t, blitter, 0x38, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x3A, cpu.Byte, 1)
	mustWriteBlitter(t, blitter, 0x3B, cpu.Byte, 3)
	mustWriteBlitter(t, blitter, 0x3D, cpu.Byte, 0)
	mustWriteBlitter(t, blitter, 0x3C, cpu.Byte, blitterBusy|1)

	first, err := ram.Read(cpu.Word, 0x000200)
	if err != nil {
		t.Fatalf("read first halftone row: %v", err)
	}
	second, err := ram.Read(cpu.Word, 0x000210)
	if err != nil {
		t.Fatalf("read second halftone row: %v", err)
	}
	if first != 0xF0F0 {
		t.Fatalf("unexpected first halftone row: got %04x want f0f0", first)
	}
	if second != 0x0F0F {
		t.Fatalf("unexpected second halftone row: got %04x want 0f0f", second)
	}
}

func TestBlitterFillsMultipleWordsAcrossRows(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	blitter := NewBlitter(ram)

	for i := 0; i < 16; i++ {
		mustWriteBlitter(t, blitter, uint32(i*2), cpu.Word, 0xFFFF)
	}

	mustWriteBlitter(t, blitter, 0x28, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2A, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2C, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2E, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x30, cpu.Word, 0x000E)
	mustWriteBlitter(t, blitter, 0x32, cpu.Long, 0x00000300)
	mustWriteBlitter(t, blitter, 0x36, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x38, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x3A, cpu.Byte, 1)
	mustWriteBlitter(t, blitter, 0x3B, cpu.Byte, 3)
	mustWriteBlitter(t, blitter, 0x3D, cpu.Byte, 0)
	mustWriteBlitter(t, blitter, 0x3C, cpu.Byte, blitterBusy|blitterHog)

	wantAddrs := []uint32{0x000300, 0x000302, 0x000310, 0x000312}
	for _, addr := range wantAddrs {
		value, err := ram.Read(cpu.Word, addr)
		if err != nil {
			t.Fatalf("read filled word at %06x: %v", addr, err)
		}
		if value != 0xFFFF {
			t.Fatalf("unexpected filled value at %06x: got %04x want ffff", addr, value)
		}
	}

	status, err := blitter.Read(cpu.Byte, blitterBase+0x3C)
	if err != nil {
		t.Fatalf("read status after fill: %v", err)
	}
	if status&blitterHog == 0 {
		t.Fatalf("expected HOG bit to remain set after BUSY write, got %02x", status)
	}
}

func TestBlitterCopiesReverseDirectionForOverlap(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	blitter := NewBlitter(ram)

	if err := ram.Write(cpu.Word, 0x000404, 0x1111); err != nil {
		t.Fatalf("write source word 0: %v", err)
	}
	if err := ram.Write(cpu.Word, 0x000406, 0x2222); err != nil {
		t.Fatalf("write source word 1: %v", err)
	}

	mustWriteBlitter(t, blitter, 0x20, cpu.Word, 0xFFFE)
	mustWriteBlitter(t, blitter, 0x22, cpu.Word, 0xFFFC)
	mustWriteBlitter(t, blitter, 0x24, cpu.Long, 0x00000406)
	mustWriteBlitter(t, blitter, 0x28, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2A, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2C, cpu.Word, 0xFFFF)
	mustWriteBlitter(t, blitter, 0x2E, cpu.Word, 0xFFFE)
	mustWriteBlitter(t, blitter, 0x30, cpu.Word, 0xFFFC)
	mustWriteBlitter(t, blitter, 0x32, cpu.Long, 0x0000040A)
	mustWriteBlitter(t, blitter, 0x36, cpu.Word, 2)
	mustWriteBlitter(t, blitter, 0x38, cpu.Word, 1)
	mustWriteBlitter(t, blitter, 0x3A, cpu.Byte, 2)
	mustWriteBlitter(t, blitter, 0x3B, cpu.Byte, 3)
	mustWriteBlitter(t, blitter, 0x3D, cpu.Byte, blitterFXSR)
	mustWriteBlitter(t, blitter, 0x3C, cpu.Byte, blitterBusy)

	value0, err := ram.Read(cpu.Word, 0x000408)
	if err != nil {
		t.Fatalf("read copied overlap word 0: %v", err)
	}
	value1, err := ram.Read(cpu.Word, 0x00040A)
	if err != nil {
		t.Fatalf("read copied overlap word 1: %v", err)
	}
	if value0 != 0x1111 || value1 != 0x2222 {
		t.Fatalf("unexpected reverse copy result: got %04x %04x want 1111 2222", value0, value1)
	}
}

func mustWriteBlitter(t *testing.T, blitter *Blitter, offset uint32, size cpu.Size, value uint32) {
	t.Helper()
	if err := blitter.Write(size, blitterBase+offset, value); err != nil {
		t.Fatalf("write blitter register %02x: %v", offset, err)
	}
}

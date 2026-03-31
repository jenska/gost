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

func mustWriteBlitter(t *testing.T, blitter *Blitter, offset uint32, size cpu.Size, value uint32) {
	t.Helper()
	if err := blitter.Write(size, blitterBase+offset, value); err != nil {
		t.Fatalf("write blitter register %02x: %v", offset, err)
	}
}

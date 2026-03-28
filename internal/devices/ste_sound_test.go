package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestSTESoundReturnsStableAllOnesReads(t *testing.T) {
	sound := NewSTESound()

	byteValue, err := sound.Read(cpu.Byte, 0xFF8901)
	if err != nil {
		t.Fatalf("read byte: %v", err)
	}
	if byteValue != 0xFF {
		t.Fatalf("unexpected byte read: got %02x want ff", byteValue)
	}

	wordValue, err := sound.Read(cpu.Word, 0xFF8900)
	if err != nil {
		t.Fatalf("read word: %v", err)
	}
	if wordValue != 0xFFFF {
		t.Fatalf("unexpected word read: got %04x want ffff", wordValue)
	}

	longValue, err := sound.Read(cpu.Long, 0xFF8900)
	if err != nil {
		t.Fatalf("read long: %v", err)
	}
	if longValue != 0xFFFFFFFF {
		t.Fatalf("unexpected long read: got %08x want ffffffff", longValue)
	}
}

package devices

import (
	"testing"

	"github.com/jenska/m68kemu"
)

func TestGLUEConfigRegisterDefaultsToZero(t *testing.T) {
	glue := NewGLUE()

	value, err := glue.Read(m68kemu.Word, glueBase)
	if err != nil {
		t.Fatalf("read glue register: %v", err)
	}
	if got := uint16(value); got != 0 {
		t.Fatalf("unexpected default glue value: got %04x want 0000", got)
	}
}

func TestGLUEByteWritesUpdateHighAndLowBytes(t *testing.T) {
	glue := NewGLUE()

	if err := glue.Write(m68kemu.Byte, glueBase, 0x12); err != nil {
		t.Fatalf("write glue high byte: %v", err)
	}
	if err := glue.Write(m68kemu.Byte, glueBase+1, 0x34); err != nil {
		t.Fatalf("write glue low byte: %v", err)
	}

	value, err := glue.Read(m68kemu.Word, glueBase)
	if err != nil {
		t.Fatalf("read glue word: %v", err)
	}
	if got := uint16(value); got != 0x1234 {
		t.Fatalf("unexpected glue word: got %04x want 1234", got)
	}
}

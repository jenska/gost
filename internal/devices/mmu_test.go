package devices

import (
	"testing"

	"github.com/jenska/m68kemu"
)

func TestMemoryConfigDefaultsToOneMegSTLayout(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	overlay := NewOverlayROM(NewROM(make([]byte, 16), 0xFC0000), ram)
	config := NewMemoryConfig(overlay, ram.Size())

	if got := config.LogicalSize(); got != ram.Size() {
		t.Fatalf("unexpected logical size: got %d want %d", got, ram.Size())
	}

	value, err := config.Read(m68kemu.Byte, memoryConfigBase+1)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if got := byte(value); got != 0x05 {
		t.Fatalf("unexpected default MMU config: got %02x want 05", got)
	}
}

func TestRAMTranslatesBankedMMUAddresses(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	overlay := NewOverlayROM(NewROM(make([]byte, 16), 0xFC0000), ram)
	config := NewMemoryConfig(overlay, ram.Size())
	ram.SetMemoryConfig(config)

	if err := ram.LoadAt(0x000008, []byte{0x12, 0x34}); err != nil {
		t.Fatalf("load bank0 payload: %v", err)
	}
	if err := ram.LoadAt(0x080008, []byte{0xBE, 0xEF}); err != nil {
		t.Fatalf("load bank1 payload: %v", err)
	}

	if err := config.Write(m68kemu.Byte, memoryConfigBase+1, 0x0A); err != nil {
		t.Fatalf("write mmu config: %v", err)
	}

	value, err := ram.Read(m68kemu.Word, 0x200008)
	if err != nil {
		t.Fatalf("read translated address: %v", err)
	}
	if got := uint16(value); got != 0xBEEF {
		t.Fatalf("unexpected translated value: got %04x want beef", got)
	}

	if err := ram.Write(m68kemu.Word, 0x200010, 0xCAFE); err != nil {
		t.Fatalf("write translated address: %v", err)
	}
	if got := readUint16BE(ram.Bytes(), 0x080010); got != 0xCAFE {
		t.Fatalf("unexpected translated write: got %04x want cafe", got)
	}
}

func TestWarmResetPreservesRAMAndMMUState(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	rom := NewROM(make([]byte, 16), 0xFC0000)
	overlay := NewOverlayROM(rom, ram)
	config := NewMemoryConfig(overlay, ram.Size())
	ram.SetMemoryConfig(config)
	bus := m68kemu.NewBus(overlay, ram, config)

	if err := ram.Write(m68kemu.Word, 0x000008, 0xAA55); err != nil {
		t.Fatalf("seed RAM: %v", err)
	}
	if err := config.Write(m68kemu.Byte, memoryConfigBase+1, 0x0A); err != nil {
		t.Fatalf("write MMU config: %v", err)
	}
	if overlay.Enabled() {
		t.Fatalf("expected overlay to be disabled after MMU write")
	}

	bus.Reset()

	value, err := ram.Read(m68kemu.Word, 0x000008)
	if err != nil {
		t.Fatalf("read RAM after warm reset: %v", err)
	}
	if got := uint16(value); got != 0xAA55 {
		t.Fatalf("warm reset cleared RAM: got %04x want aa55", got)
	}

	configValue, err := config.Read(m68kemu.Byte, memoryConfigBase+1)
	if err != nil {
		t.Fatalf("read MMU config after warm reset: %v", err)
	}
	if got := byte(configValue); got != 0x0A {
		t.Fatalf("warm reset changed MMU config: got %02x want 0a", got)
	}
	if overlay.Enabled() {
		t.Fatalf("warm reset re-enabled ROM overlay")
	}
}

func TestShifterDoesNotExposeSTELowScreenBaseRegister(t *testing.T) {
	shifter := NewShifter(NewRAM(0, 1024*1024))
	if shifter.Contains(0xFF820D) {
		t.Fatalf("ST shifter should not expose STE low-byte screen base register")
	}
}

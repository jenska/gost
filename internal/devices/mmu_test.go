package devices

import (
	"testing"

	"github.com/jenska/gost/internal/config"
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

func TestMemoryConfigSupportsTwoMegSTLayout(t *testing.T) {
	ram := NewRAM(0, 2*1024*1024)
	overlay := NewOverlayROM(NewROM(make([]byte, 16), 0xFC0000), ram)
	config := NewMemoryConfig(overlay, ram.Size())

	if got := config.LogicalSize(); got != ram.Size() {
		t.Fatalf("unexpected logical size: got %d want %d", got, ram.Size())
	}

	value, err := config.Read(m68kemu.Byte, memoryConfigBase+1)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if got := byte(value); got != 0x0B {
		t.Fatalf("unexpected default 2MB MMU config: got %02x want 0b", got)
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

func TestRAMTwoMegLayoutAllowsAccessAboveOneMeg(t *testing.T) {
	ram := NewRAM(0, 2*1024*1024)
	overlay := NewOverlayROM(NewROM(make([]byte, 16), 0xFC0000), ram)
	config := NewMemoryConfig(overlay, ram.Size())
	ram.SetMemoryConfig(config)

	const addr = 0x00180000
	if err := ram.Write(m68kemu.Word, addr, 0xCAFE); err != nil {
		t.Fatalf("write high RAM word: %v", err)
	}

	value, err := ram.Read(m68kemu.Word, addr)
	if err != nil {
		t.Fatalf("read high RAM word: %v", err)
	}
	if got := uint16(value); got != 0xCAFE {
		t.Fatalf("unexpected high RAM value: got %04x want cafe", got)
	}
}

func TestRAMAbsentBankReadsAsZeroAndIgnoresWrites(t *testing.T) {
	ram := NewRAM(0, 2*1024*1024)
	overlay := NewOverlayROM(NewROM(make([]byte, 16), 0xFC0000), ram)
	config := NewMemoryConfig(overlay, ram.Size())
	ram.SetMemoryConfig(config)

	if err := config.Write(m68kemu.Byte, memoryConfigBase+1, 0x0A); err != nil {
		t.Fatalf("write probing mmu config: %v", err)
	}

	if err := ram.Write(m68kemu.Long, 0x200008, 0xA55AA55A); err != nil {
		t.Fatalf("write absent bank: %v", err)
	}

	value, err := ram.Read(m68kemu.Long, 0x200008)
	if err != nil {
		t.Fatalf("read absent bank: %v", err)
	}
	if value != 0 {
		t.Fatalf("unexpected absent-bank readback: got %08x want 00000000", value)
	}

	if got := readUint32BE(ram.Bytes(), 0x000008); got == 0xA55AA55A {
		t.Fatalf("absent-bank write incorrectly aliased into backing RAM")
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
	shifter := NewSTShifter(&config.Config{}, NewRAM(0, 1024*1024))
	if shifter.Contains(0xFF820D) {
		t.Fatalf("ST shifter should not expose STE low-byte screen base register")
	}
}

func TestShifterSTEExposesLowAddressRegisters(t *testing.T) {
	shifter := NewSTEShifter(&config.Config{}, NewRAM(0, 1024*1024))
	if !shifter.Contains(0xFF820D) {
		t.Fatalf("STE shifter should expose low-byte screen base register")
	}
	if !shifter.Contains(0xFF8209) {
		t.Fatalf("STE shifter should expose low-byte video counter register")
	}
	if !shifter.Contains(0xFF820F) {
		t.Fatalf("STE shifter should expose line-offset register")
	}
	if !shifter.Contains(0xFF8265) {
		t.Fatalf("STE shifter should expose fine-scroll register")
	}
}

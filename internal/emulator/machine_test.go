package emulator

import (
	"encoding/binary"
	"testing"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/devices"
	"github.com/jenska/m68kemu"
)

func TestMachineResetVectors(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))
	regs := machine.Registers()

	if regs.PC != defaultROMHighAlias+8 {
		t.Fatalf("unexpected reset PC: got %06x want %06x", regs.PC, defaultROMHighAlias+8)
	}
	if regs.A[7] != 0x00080000 {
		t.Fatalf("unexpected reset SSP: got %08x want %08x", regs.A[7], 0x00080000)
	}
}

func TestSTBusAlignmentAndMapping(t *testing.T) {
	ram := devices.NewRAM(0, 1024*1024)
	rom := devices.NewROM(loopROM(nil), defaultROMHighAlias, secondaryROMAlias)
	bus := NewSTBus(devices.NewOverlayROM(rom, ram), ram, rom)

	if _, err := bus.Read(m68kemu.Word, 1); err == nil {
		t.Fatalf("expected address error for odd word access")
	} else if _, ok := err.(m68kemu.AddressError); !ok {
		t.Fatalf("expected AddressError, got %T", err)
	}

	if _, err := bus.Read(m68kemu.Byte, 0x400000); err == nil {
		t.Fatalf("expected bus error for unmapped access")
	} else if _, ok := err.(m68kemu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}

	value, err := bus.Read(m68kemu.Long, 0)
	if err != nil {
		t.Fatalf("read reset vector: %v", err)
	}
	if value != 0x00080000 {
		t.Fatalf("unexpected reset vector: got %08x", value)
	}
}

func TestOverlayWritesPassThroughToRAM(t *testing.T) {
	ram := devices.NewRAM(0, 1024*1024)
	rom := devices.NewROM(loopROM(nil), defaultROMHighAlias, secondaryROMAlias)
	overlay := devices.NewOverlayROM(rom, ram)
	bus := NewSTBus(overlay, ram, rom)

	if err := bus.Write(m68kemu.Long, 0x04, 0x12345678); err != nil {
		t.Fatalf("write through overlay: %v", err)
	}

	got, err := ram.Read(m68kemu.Long, 0x04)
	if err != nil {
		t.Fatalf("read RAM after overlay write: %v", err)
	}
	if got != 0x12345678 {
		t.Fatalf("unexpected RAM value: got %08x want %08x", got, 0x12345678)
	}
}

func TestMachineInterruptHandling(t *testing.T) {
	rom := loopROM([]byte{
		0x46, 0xFC, 0x20, 0x00, // move #$2000,sr
		0x4E, 0x71, // nop
		0x60, 0xFE, // bra.s -2
	})
	machine := mustMachine(t, rom)

	handlerAddress := uint32(0x00001000)
	vectorOffset := uint32((24 + 6) * 4)
	vectorBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(vectorBytes, handlerAddress)
	if err := machine.LoadIntoRAM(vectorOffset, vectorBytes); err != nil {
		t.Fatalf("load interrupt vector: %v", err)
	}
	if err := machine.LoadIntoRAM(handlerAddress, []byte{
		0x70, 0x01, // moveq #1,d0
		0x4E, 0x73, // rte
	}); err != nil {
		t.Fatalf("load interrupt handler: %v", err)
	}

	if err := machine.RequestInterrupt(6, nil); err != nil {
		t.Fatalf("request interrupt: %v", err)
	}

	for i := 0; i < 4; i++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", i, err)
		}
	}

	if got := machine.Registers().D[0]; got != 1 {
		t.Fatalf("interrupt handler did not run: D0=%08x", uint32(got))
	}
}

func TestHeadlessMachineStepsFrames(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))

	for i := 0; i < 3; i++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", i, err)
		}
	}

	if machine.Cycles() == 0 {
		t.Fatalf("expected CPU cycles to advance")
	}
}

func TestBundledEmuTOSCreatesMachine(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine with bundled EmuTOS: %v", err)
	}
	if machine.Registers().PC == 0 {
		t.Fatalf("expected non-zero reset PC from bundled EmuTOS")
	}
}

func TestVBLInterruptRunsHandler(t *testing.T) {
	rom := loopROM([]byte{
		0x46, 0xFC, 0x20, 0x00, // move #$2000,sr
		0x4E, 0x71, // nop
		0x60, 0xFA, // bra.s -6
	})
	machine := mustMachine(t, rom)

	handlerAddress := uint32(0x00002000)
	vectorOffset := uint32((24 + 4) * 4)
	vectorBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(vectorBytes, handlerAddress)
	if err := machine.LoadIntoRAM(vectorOffset, vectorBytes); err != nil {
		t.Fatalf("load vbl vector: %v", err)
	}
	if err := machine.LoadIntoRAM(handlerAddress, []byte{
		0x72, 0x01, // moveq #1,d1
		0x4E, 0x73, // rte
	}); err != nil {
		t.Fatalf("load vbl handler: %v", err)
	}

	if _, err := machine.StepFrame(); err != nil {
		t.Fatalf("step frame 1: %v", err)
	}
	if _, err := machine.StepFrame(); err != nil {
		t.Fatalf("step frame: %v", err)
	}

	if got := machine.Registers().D[1]; got != 1 {
		t.Fatalf("VBL handler did not run: D1=%08x", uint32(got))
	}
}

func mustMachine(t *testing.T, rom []byte) *Machine {
	t.Helper()
	machine, err := NewMachine(DefaultConfig(), rom)
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	return machine
}

func loopROM(code []byte) []byte {
	if len(code) == 0 {
		code = []byte{0x4E, 0x71, 0x60, 0xFE}
	}
	rom := make([]byte, 8+len(code))
	binary.BigEndian.PutUint32(rom[0:4], 0x00080000)
	binary.BigEndian.PutUint32(rom[4:8], defaultROMHighAlias+8)
	copy(rom[8:], code)
	return rom
}

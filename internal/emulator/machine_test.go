package emulator

import (
	"encoding/binary"
	"testing"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/devices"
	cpu "github.com/jenska/m68kemu"
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

func TestMachineSupportsTwoMegRAMWithoutBusError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RAMSize = 2 * 1024 * 1024
	machine, err := NewMachine(cfg, loopROM(nil))
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if _, err := machine.StepFrame(); err != nil {
		t.Fatalf("step frame with 2MB RAM: %v", err)
	}
}

func TestMachineTwoMegRAMBootsBundledROMToActiveScreenBase(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RAMSize = 2 * 1024 * 1024
	cfg.Model = config.MachineModelST

	machine, err := NewMachine(cfg, assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := range 200 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if machine.shifter.ScreenBase() != 0 {
			phystop, err := machine.ram.Read(cpu.Long, 0x042E)
			if err != nil {
				t.Fatalf("read phystop: %v", err)
			}
			if phystop != 0x00200000 {
				t.Fatalf("unexpected phystop after 2MB boot: got %08x want 00200000", phystop)
			}
			return
		}
	}

	t.Fatalf("expected bundled ROM boot to program non-zero screen base within 200 frames")
}

func TestSTBusAlignmentAndMapping(t *testing.T) {
	ram := devices.NewRAM(0, 1024*1024)
	rom := devices.NewROM(loopROM(nil), defaultROMHighAlias, secondaryROMAlias)
	overlay := devices.NewOverlayROM(rom, ram)
	memoryConfig := devices.NewMemoryConfig(overlay, ram.Size())
	blitter := devices.NewBlitter(ram)
	monsterProbe := devices.NewBusErrorRegion(devices.AddressRange{Start: 0xFFFE00, End: 0xFFFE10})
	openBus := devices.NewOpenBus(devices.AddressRange{Start: 0xFF8000, End: 0x1000000})
	bus := cpu.NewBus(overlay, ram, memoryConfig, devices.NewGLUE(), blitter, monsterProbe, openBus, rom)
	bus.SetWaitStates(4)

	if _, err := bus.Read(cpu.Word, 1); err == nil {
		t.Fatalf("expected address error for odd word access")
	} else if _, ok := err.(cpu.AddressError); !ok {
		t.Fatalf("expected AddressError, got %T", err)
	}

	if _, err := bus.Read(cpu.Byte, 0x400000); err == nil {
		t.Fatalf("expected bus error for unmapped access")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError, got %T", err)
	}

	value, err := bus.Read(cpu.Long, 0)
	if err != nil {
		t.Fatalf("read reset vector: %v", err)
	}
	if value != 0x00080000 {
		t.Fatalf("unexpected reset vector: got %08x", value)
	}

	value, err = bus.Read(cpu.Word, 0xFF8006)
	if err != nil {
		t.Fatalf("read glue register: %v", err)
	}
	if value != 0 {
		t.Fatalf("expected glue register to win over ROM alias: got %04x want 0000", value)
	}

	value, err = bus.Read(cpu.Byte, 0xFF820D)
	if err != nil {
		t.Fatalf("read unmapped io hole: %v", err)
	}
	if value != 0 {
		t.Fatalf("expected unmapped io hole to read open bus, got %02x want 00", value)
	}

	value, err = bus.Read(cpu.Byte, 0xFF8A3C)
	if err != nil {
		t.Fatalf("read blitter status register: %v", err)
	}
	if value != 0 {
		t.Fatalf("expected blitter status register to reset to 00, got %02x", value)
	}

	if _, err := bus.Read(cpu.Byte, 0xFFFE00); err == nil {
		t.Fatalf("expected MonSTer probe window to bus-error")
	} else if _, ok := err.(cpu.BusError); !ok {
		t.Fatalf("expected BusError for MonSTer probe window, got %T", err)
	}
}

func TestOverlayWritesPassThroughToRAM(t *testing.T) {
	ram := devices.NewRAM(0, 1024*1024)
	rom := devices.NewROM(loopROM(nil), defaultROMHighAlias, secondaryROMAlias)
	overlay := devices.NewOverlayROM(rom, ram)
	bus := cpu.NewBus(overlay, ram, rom)

	if err := bus.Write(cpu.Long, 0x04, 0x12345678); err != nil {
		t.Fatalf("write through overlay: %v", err)
	}

	got, err := ram.Read(cpu.Long, 0x04)
	if err != nil {
		t.Fatalf("read RAM after overlay write: %v", err)
	}
	if got != 0x12345678 {
		t.Fatalf("unexpected RAM value: got %08x want %08x", got, 0x12345678)
	}
}

func TestMachineSTEModelEnablesShifterLowScreenBaseRegister(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model = config.MachineModelSTE
	machine, err := NewMachine(cfg, loopROM(nil))
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	if err := machine.bus.Write(cpu.Byte, 0xFF8201, 0x12); err != nil {
		t.Fatalf("write base high: %v", err)
	}
	if err := machine.bus.Write(cpu.Byte, 0xFF8203, 0x34); err != nil {
		t.Fatalf("write base mid: %v", err)
	}
	if err := machine.bus.Write(cpu.Byte, 0xFF820D, 0x56); err != nil {
		t.Fatalf("write base low: %v", err)
	}

	if got, want := machine.shifter.ScreenBase(), uint32(0x123456); got != want {
		t.Fatalf("unexpected STE screen base: got %06x want %06x", got, want)
	}

	value, err := machine.bus.Read(cpu.Byte, 0xFF8209)
	if err != nil {
		t.Fatalf("read STE video counter low: %v", err)
	}
	if byte(value) != 0x56 {
		t.Fatalf("unexpected STE video counter low: got %02x want 56", byte(value))
	}

	if err := machine.bus.Write(cpu.Byte, 0xFF820F, 0x03); err != nil {
		t.Fatalf("write STE line offset: %v", err)
	}
	value, err = machine.bus.Read(cpu.Byte, 0xFF820F)
	if err != nil {
		t.Fatalf("read STE line offset: %v", err)
	}
	if byte(value) != 0x03 {
		t.Fatalf("unexpected STE line offset readback: got %02x want 03", byte(value))
	}

	if err := machine.bus.Write(cpu.Byte, 0xFF8265, 0x0F); err != nil {
		t.Fatalf("write STE fine scroll: %v", err)
	}
	value, err = machine.bus.Read(cpu.Byte, 0xFF8265)
	if err != nil {
		t.Fatalf("read STE fine scroll: %v", err)
	}
	if byte(value) != 0x0F {
		t.Fatalf("unexpected STE fine scroll readback: got %02x want 0f", byte(value))
	}
}

func TestMachineInterruptHandling(t *testing.T) {
	rom := loopROM([]byte{
		0x46, 0xFC, 0x20, 0x00, // move #$2000,sr
		0x4E, 0x71, // nop
		0x60, 0xFE, // bra.s -2
	})
	machine := mustMachine(t, rom)
	// Isolate this test to explicit CPU interrupt requests and avoid
	// unrelated device IRQ traffic (for example frame-driven VBL interrupts).
	machine.irqSources = nil

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

	for i := range 4 {
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

	for i := range 3 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", i, err)
		}
	}

	if machine.Cycles() == 0 {
		t.Fatalf("expected CPU cycles to advance")
	}
}

func TestMachineRunUntilStopAtPC(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))

	targetPC := uint32(defaultROMHighAlias + 0x0A)
	result, err := machine.RunUntil(cpu.RunUntilOptions{
		StopAtPC: []uint32{targetPC},
	})
	if err != nil {
		t.Fatalf("run until stop-at-pc: %v", err)
	}

	if result.PC != targetPC {
		t.Fatalf("unexpected stop PC: got %06x want %06x", result.PC, targetPC)
	}
	if result.Instructions == 0 {
		t.Fatalf("expected at least one executed instruction")
	}
}

func TestMachineRunUntilPropagatesBusAccessStop(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))
	resetPC := machine.Registers().PC

	result, err := machine.RunUntil(cpu.RunUntilOptions{
		StopOnBusAccess: func(info cpu.BusAccessInfo) bool {
			return info.InstructionFetch && info.Address == resetPC
		},
	})
	if err != nil {
		t.Fatalf("run until bus access: %v", err)
	}

	if !result.HasBusAccess {
		t.Fatalf("expected bus-access stop metadata to be preserved")
	}
	if !result.BusAccess.InstructionFetch {
		t.Fatalf("expected stopping access to be an instruction fetch")
	}
	if result.BusAccess.Address != resetPC {
		t.Fatalf("unexpected stopping address: got %06x want %06x", result.BusAccess.Address, resetPC)
	}
}

func TestMachineDebugStateMirrorsCPUState(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))

	debug := machine.DebugState()
	regs := machine.Registers()
	if debug.Registers.PC != regs.PC {
		t.Fatalf("debug-state PC mismatch: got %06x want %06x", debug.Registers.PC, regs.PC)
	}
	if debug.InterruptMask != uint8((regs.SR>>8)&0x7) {
		t.Fatalf("debug-state interrupt mask mismatch: got %d want %d", debug.InterruptMask, (regs.SR>>8)&0x7)
	}
}

func TestMachineDefaultConfigCreates30MBVirtualHardDisk(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))
	if got, want := machine.HardDiskSizeBytes(), 30*1024*1024; got != want {
		t.Fatalf("unexpected default virtual hard disk size: got %d want %d", got, want)
	}
}

func TestMachineSetHardDiskImageReplacesVirtualDisk(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))
	custom := make([]byte, 2*512)
	custom[0] = 0xAB

	if err := machine.SetHardDiskImage(custom); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}

	image := machine.HardDiskImage()
	if len(image) != len(custom) {
		t.Fatalf("hard disk image size = %d, want %d", len(image), len(custom))
	}
	if image[0] != 0xAB {
		t.Fatalf("hard disk image content mismatch: got %02x want AB", image[0])
	}
}

func TestMachineCPUOverclockScalesCPUOnly(t *testing.T) {
	rom := loopROM([]byte{0x4E, 0x71, 0x60, 0xFE})

	baseCfg := config.DefaultConfig()
	baseCfg.CPUClockHz = baseCfg.ClockHz
	base, err := NewMachine(baseCfg, rom)
	if err != nil {
		t.Fatalf("create base machine: %v", err)
	}

	overCfg := config.DefaultConfig()
	overCfg.CPUClockHz = overCfg.ClockHz * 2
	over, err := NewMachine(overCfg, rom)
	if err != nil {
		t.Fatalf("create overclocked machine: %v", err)
	}

	if base.frameCycles != over.frameCycles {
		t.Fatalf("hardware frame cycles changed by CPU overclock: base=%d over=%d", base.frameCycles, over.frameCycles)
	}

	baseStartCycles := base.Cycles()
	overStartCycles := over.Cycles()
	if _, err := base.StepFrame(); err != nil {
		t.Fatalf("step base frame: %v", err)
	}
	if _, err := over.StepFrame(); err != nil {
		t.Fatalf("step overclocked frame: %v", err)
	}

	baseDelta := base.Cycles() - baseStartCycles
	overDelta := over.Cycles() - overStartCycles
	if overDelta <= baseDelta {
		t.Fatalf("expected overclock to increase CPU work per frame: base_delta=%d over_delta=%d", baseDelta, overDelta)
	}
	ratio := float64(overDelta) / float64(baseDelta)
	if ratio < 1.9 || ratio > 2.1 {
		t.Fatalf("unexpected CPU cycle scaling ratio: base_delta=%d over_delta=%d ratio=%.3f want ~2.0", baseDelta, overDelta, ratio)
	}
}

func TestMachineColorMonitorEnablesColorBorderOverlay(t *testing.T) {
	rom := loopROM([]byte{0x4E, 0x71, 0x60, 0xFE})

	colorCfg := config.DefaultConfig()
	colorCfg.ColorMonitor = true
	colorMachine, err := NewMachine(colorCfg, rom)
	if err != nil {
		t.Fatalf("create color machine: %v", err)
	}
	if _, err := colorMachine.StepFrame(); err != nil {
		t.Fatalf("step color machine frame: %v", err)
	}
	visibleW, visibleH := colorMachine.Dimensions()
	displayW, displayH := colorMachine.DisplayDimensions()
	if displayW <= visibleW || displayH <= visibleH {
		t.Fatalf("expected color monitor display area to include outer border: visible=%dx%d display=%dx%d", visibleW, visibleH, displayW, displayH)
	}
	vx, vy, vw, vh := colorMachine.DisplayViewport()
	if vx <= 0 || vy <= 0 || vw != visibleW || vh != visibleH {
		t.Fatalf("unexpected color monitor display viewport: got (%d,%d,%d,%d), visible=%dx%d", vx, vy, vw, vh, visibleW, visibleH)
	}

	monoCfg := config.DefaultConfig()
	monoCfg.ColorMonitor = false
	monoMachine, err := NewMachine(monoCfg, rom)
	if err != nil {
		t.Fatalf("create mono machine: %v", err)
	}
	if _, err := monoMachine.StepFrame(); err != nil {
		t.Fatalf("step mono machine frame: %v", err)
	}
	monoVisibleW, monoVisibleH := monoMachine.Dimensions()
	monoDisplayW, monoDisplayH := monoMachine.DisplayDimensions()
	if monoDisplayW != monoVisibleW || monoDisplayH != monoVisibleH {
		t.Fatalf("expected monochrome monitor display to match visible area: visible=%dx%d display=%dx%d", monoVisibleW, monoVisibleH, monoDisplayW, monoDisplayH)
	}
	mvx, mvy, mvw, mvh := monoMachine.DisplayViewport()
	if mvx != 0 || mvy != 0 || mvw != monoVisibleW || mvh != monoVisibleH {
		t.Fatalf("unexpected monochrome display viewport: got (%d,%d,%d,%d), visible=%dx%d", mvx, mvy, mvw, mvh, monoVisibleW, monoVisibleH)
	}
}

func TestMachineMidResYScaleAppliesToDisplayViewport(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ColorMonitor = true
	cfg.MidResYScale = 2
	machine, err := NewMachine(cfg, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	if err := machine.shifter.Write(cpu.Byte, 0xFF8260, 0x01); err != nil {
		t.Fatalf("set medium resolution: %v", err)
	}
	if _, err := machine.StepFrame(); err != nil {
		t.Fatalf("step frame: %v", err)
	}

	if w, h := machine.Dimensions(); w != 640 || h != 200 {
		t.Fatalf("unexpected guest dimensions: got %dx%d want 640x200", w, h)
	}
	_, _, vw, vh := machine.DisplayViewport()
	if vw != 640 || vh != 400 {
		t.Fatalf("unexpected display viewport size: got %dx%d want 640x400", vw, vh)
	}
}

func TestMachineResetRestoresColdBootState(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))
	initialConfigValue, err := machine.memoryConfig.Read(cpu.Byte, 0xFF8001)
	if err != nil {
		t.Fatalf("read initial MMU config: %v", err)
	}

	if err := machine.LoadIntoRAM(0x000008, []byte{0xAA, 0x55}); err != nil {
		t.Fatalf("seed RAM: %v", err)
	}
	if err := machine.memoryConfig.Write(cpu.Byte, 0xFF8001, 0x0A); err != nil {
		t.Fatalf("write MMU config: %v", err)
	}
	if machine.overlayROM.Enabled() {
		t.Fatalf("expected overlay to be disabled after MMU write")
	}

	if err := machine.Reset(); err != nil {
		t.Fatalf("machine reset: %v", err)
	}

	value, err := machine.ram.Read(cpu.Word, 0x000008)
	if err != nil {
		t.Fatalf("read RAM after reset: %v", err)
	}
	if value != 0 {
		t.Fatalf("expected cold reset to clear RAM: got %04x", value)
	}

	configValue, err := machine.memoryConfig.Read(cpu.Byte, 0xFF8001)
	if err != nil {
		t.Fatalf("read MMU config after reset: %v", err)
	}
	if got, want := byte(configValue), byte(initialConfigValue); got != want {
		t.Fatalf("expected cold reset MMU config %02x, got %02x", want, got)
	}
	if !machine.overlayROM.Enabled() {
		t.Fatalf("expected cold reset to re-enable ROM overlay")
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
	machine, err := NewMachine(config.DefaultConfig(), rom)
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

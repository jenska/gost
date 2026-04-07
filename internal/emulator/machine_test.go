package emulator

import (
	"encoding/binary"
	"testing"

	"github.com/jenska/gost/internal/assets"
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

func TestSTBusAlignmentAndMapping(t *testing.T) {
	ram := devices.NewRAM(0, 1024*1024)
	rom := devices.NewROM(loopROM(nil), defaultROMHighAlias, secondaryROMAlias)
	overlay := devices.NewOverlayROM(rom, ram)
	memoryConfig := devices.NewMemoryConfig(overlay, ram.Size())
	blitter := devices.NewBlitter(ram)
	monsterProbe := devices.NewBusErrorRegion(devices.AddressRange{Start: 0xFFFE00, End: 0xFFFE10})
	openBus := devices.NewOpenBus(devices.AddressRange{Start: 0xFF8000, End: 0x1000000})
	bus := NewSTBus(overlay, ram, memoryConfig, devices.NewGLUE(), blitter, monsterProbe, openBus, rom)

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
	bus := NewSTBus(overlay, ram, rom)

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

func TestBundledEmuTOSCreatesMachine(t *testing.T) {
	machine := mustBundledMachine(t, DefaultConfig())
	if machine.Registers().PC == 0 {
		t.Fatalf("expected non-zero reset PC from bundled EmuTOS")
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

func TestBundledEmuTOSReachesShifterSetup(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine with bundled EmuTOS: %v", err)
	}

	for frame := 0; frame < 200; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if machine.shifter.ScreenBase() != 0 {
			return
		}
	}

	t.Fatalf("expected EmuTOS boot to program a non-zero shifter screen base within 200 frames")
}

func TestBundledEmuTOSReachesDesktop(t *testing.T) {
	machine := mustBootBundledMachine(t, DefaultConfig(), 400)

	if panicRecordSet(t, machine) {
		t.Fatalf("expected desktop boot to avoid the old panic path")
	}

	width, height := machine.shifter.Dimensions()
	if width != 640 || height != 400 {
		t.Fatalf("expected high-resolution desktop mode, got %dx%d", width, height)
	}

	frame := machine.FrameBuffer()
	if len(frame) != width*height*4 {
		t.Fatalf("unexpected framebuffer size: got %d want %d", len(frame), width*height*4)
	}

	var menuBlack, trashBlack, whitePixels int
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			r, g, b, a := frame[offset], frame[offset+1], frame[offset+2], frame[offset+3]
			if a == 0 {
				continue
			}
			if r == 0xFF && g == 0xFF && b == 0xFF {
				whitePixels++
			}
			if r == 0x00 && g == 0x00 && b == 0x00 {
				if y < 16 && x < 220 {
					menuBlack++
				}
				if x < 80 && y > 340 {
					trashBlack++
				}
			}
		}
	}

	if menuBlack < 40 {
		t.Fatalf("expected menu-bar text pixels, got only %d black pixels in top-left menu region", menuBlack)
	}
	if trashBlack < 40 {
		t.Fatalf("expected desktop icon pixels, got only %d black pixels in trash region", trashBlack)
	}
	if whitePixels < 5000 {
		t.Fatalf("expected visible desktop framebuffer, got only %d white pixels", whitePixels)
	}
}

func TestBundledEmuTOSMountsDefaultVirtualHardDiskAsDriveC(t *testing.T) {
	machine := mustBootBundledMachine(t, DefaultConfig(), 500)

	// EmuTOS low-memory variable _drvbits is at 0x04C2; bit 2 indicates C:.
	var drvbits uint32
	for i := 0; i < 4; i++ {
		value, err := machine.ram.Read(cpu.Byte, 0x04C2+uint32(i))
		if err != nil {
			t.Fatalf("read _drvbits byte %d: %v", i, err)
		}
		drvbits = (drvbits << 8) | uint32(byte(value))
	}
	if drvbits&(1<<2) == 0 {
		t.Fatalf("expected C: to be present in _drvbits, got %08x", drvbits)
	}
}

func TestBundledEmuTOSReachesColorDesktop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ColorMonitor = true

	machine := mustBootBundledMachine(t, cfg, 400)

	if panicRecordSet(t, machine) {
		t.Fatalf("expected color desktop boot to avoid the panic path")
	}

	width, height := machine.shifter.Dimensions()
	if width != 320 || height != 200 {
		t.Fatalf("expected low-resolution color desktop mode, got %dx%d", width, height)
	}
}

func TestBundledEmuTOSKeypressProducesAudioSamples(t *testing.T) {
	machine := mustBootBundledMachine(t, DefaultConfig(), 400)

	audio := machine.AudioSource()
	silence := make([]float32, audio.OutputSampleRate())
	_ = audio.DrainMonoF32(silence)

	if conterm, err := machine.ram.Read(cpu.Byte, 0x000484); err != nil {
		t.Fatalf("read conterm: %v", err)
	} else if conterm&0x01 == 0 {
		t.Fatalf("expected EmuTOS keyclick to be enabled in conterm, got %02x", conterm)
	}
	keyclickHook, err := machine.ram.Read(cpu.Long, 0x0005B0)
	if err != nil {
		t.Fatalf("read kcl_hook: %v", err)
	}
	if keyclickHook == 0 {
		t.Fatalf("expected non-zero keyclick hook")
	}

	var psgWrites []uint32
	var timerCExceptions int
	var aciaExceptions int
	var keyclickHookHits int
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		if info.PC == keyclickHook {
			keyclickHookHits++
		}
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		switch info.Vector {
		case 0x45:
			timerCExceptions++
		case 0x46:
			aciaExceptions++
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		if !info.Write {
			return
		}
		if info.Address >= 0xFF8800 && info.Address < 0xFF8804 {
			psgWrites = append(psgWrites, info.Address)
		}
	})
	machine.PushKey(0x1E, true)
	machine.PushKey(0x1E, false)

	samples := make([]float32, 2048)
	var heardAudio bool
	for frame := 0; frame < 10; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step audio frame %d: %v", frame, err)
		}
		n := audio.DrainMonoF32(samples)
		for i := 0; i < n; i++ {
			if samples[i] != 0 {
				heardAudio = true
				break
			}
		}
		if heardAudio {
			break
		}
	}
	machine.cpu.SetBusTracer(nil)
	machine.cpu.SetTracer(nil)

	if len(psgWrites) == 0 {
		status, err := machine.bus.Read(cpu.Byte, 0xFFFC00)
		if err != nil {
			t.Fatalf("read ACIA status after keypress: %v", err)
		}
		disasm := []string{"<decode failed>"}
		pc := keyclickHook
		for i := 0; i < 4; i++ {
			inst, err := cpu.DisassembleInstruction(machine.bus.CPUAddressBus(), pc)
			if err != nil {
				break
			}
			if i == 0 {
				disasm = disasm[:0]
			}
			disasm = append(disasm, inst.Assembly)
			if len(inst.Bytes) == 0 {
				break
			}
			pc += uint32(len(inst.Bytes))
		}
		t.Fatalf("expected keypress to trigger PSG register writes; ACIA status=%02x timerC=%d acia=%d keyclick=%d hook=%06x ins=%q", status, timerCExceptions, aciaExceptions, keyclickHookHits, keyclickHook, disasm)
	}

	if !heardAudio {
		t.Fatalf("expected keypress audio samples to contain audible data")
	}
}

func TestBundledEmuTOSMouseMoveChangesDesktopFrame(t *testing.T) {
	base := mustBootBundledMachine(t, DefaultConfig(), 400)
	withMouse := mustBootBundledMachine(t, DefaultConfig(), 400)

	withMouse.PushMouse(12, 8, 0)

	for frame := 0; frame < 20; frame++ {
		if _, err := base.StepFrame(); err != nil {
			t.Fatalf("baseline post-mouse frame %d: %v", frame, err)
		}
		if _, err := withMouse.StepFrame(); err != nil {
			t.Fatalf("mouse post-mouse frame %d: %v", frame, err)
		}
	}

	baseFrame := base.FrameBuffer()
	mouseFrame := withMouse.FrameBuffer()
	if len(baseFrame) != len(mouseFrame) {
		t.Fatalf("framebuffer size mismatch: baseline=%d mouse=%d", len(baseFrame), len(mouseFrame))
	}

	changed := 0
	for i := range baseFrame {
		if baseFrame[i] != mouseFrame[i] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatalf("expected mouse input to change the desktop framebuffer")
	}
}

func TestBundledEmuTOSUsesBlitterDuringDesktopBoot(t *testing.T) {
	machine := mustBundledMachine(t, DefaultConfig())

	var statusWrites int
	var busyStarts int
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if info.InstructionFetch || !info.Write || addr != 0xFF8A3C {
			return
		}
		statusWrites++
		if info.Value&0x80 != 0 {
			busyStarts++
		}
	})
	defer machine.cpu.SetBusTracer(nil)

	stepFrames(t, machine, 400)

	if statusWrites == 0 {
		t.Fatalf("expected desktop boot to write the blitter status register")
	}
	if busyStarts == 0 {
		t.Fatalf("expected desktop boot to start at least one blitter transfer")
	}
}

func TestBundledEmuTOSReportsMousePosition(t *testing.T) {
	machine := mustBootBundledMachine(t, DefaultConfig(), 400)

	beforeX, beforeY, ok := machine.MousePosition()
	if !ok {
		t.Fatalf("expected GEM desktop to expose a mouse position")
	}

	machine.PushMouse(12, 8, 0)
	for frame := 0; frame < 20; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", frame, err)
		}
	}

	afterX, afterY, ok := machine.MousePosition()
	if !ok {
		t.Fatalf("expected GEM desktop to expose a mouse position after moving")
	}
	if afterX == beforeX && afterY == beforeY {
		t.Fatalf("expected mouse position to change after movement, stayed at (%d,%d)", afterX, afterY)
	}
}

func TestBundledEmuTOSDoesNotPanicWithoutMFPDelivery(t *testing.T) {
	machine := mustBundledMachine(t, DefaultConfig())

	filteredClocked := make([]devices.Clocked, 0, len(machine.clocked))
	for _, device := range machine.clocked {
		if device == machine.mfp {
			continue
		}
		filteredClocked = append(filteredClocked, device)
	}
	machine.clocked = filteredClocked

	filteredIRQs := make([]devices.InterruptSource, 0, len(machine.irqSources))
	for _, source := range machine.irqSources {
		if source == machine.mfp {
			continue
		}
		filteredIRQs = append(filteredIRQs, source)
	}
	machine.irqSources = filteredIRQs

	stepFrames(t, machine, 120)

	if panicRecordSet(t, machine) {
		t.Fatalf("expected late panic to disappear when MFP delivery is removed")
	}
}

func TestMachineResetRestoresColdBootState(t *testing.T) {
	machine := mustMachine(t, loopROM([]byte{0x4E, 0x71, 0x60, 0xFE}))

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
	if got := byte(configValue); got != 0x05 {
		t.Fatalf("expected cold reset MMU config 05, got %02x", got)
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
	machine, err := NewMachine(DefaultConfig(), rom)
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	return machine
}

func mustBundledMachine(t *testing.T, cfg Config) *Machine {
	t.Helper()
	machine, err := NewMachine(cfg, assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine with bundled EmuTOS: %v", err)
	}
	return machine
}

func mustBootBundledMachine(t *testing.T, cfg Config, frames int) *Machine {
	t.Helper()
	machine := mustBundledMachine(t, cfg)
	stepFrames(t, machine, frames)
	return machine
}

func stepFrames(t *testing.T, machine *Machine, frames int) {
	t.Helper()
	for frame := 0; frame < frames; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}
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

func panicRecordSet(t *testing.T, machine *Machine) bool {
	t.Helper()
	value, err := machine.ram.Read(cpu.Long, 0x380)
	if err != nil {
		t.Fatalf("read panic record marker: %v", err)
	}
	return value == 0x12345678
}

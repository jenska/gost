//go:build debugtests
// +build debugtests

package emulator

import (
	"testing"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/devices"
	cpu "github.com/jenska/m68kemu"
)

func TestBundledEmuTOSCreatesMachine(t *testing.T) {
	machine := mustBundledMachine(t, DefaultConfig())
	if machine.Registers().PC == 0 {
		t.Fatalf("expected non-zero reset PC from bundled EmuTOS")
	}
}

func TestBundledEmuTOSReachesShifterSetup(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine with bundled EmuTOS: %v", err)
	}

	for frame := range 200 {
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
	cfg := DefaultConfig()
	cfg.ColorMonitor = false
	machine := mustBootBundledMachine(t, cfg, 400)

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
	for y := range height {
		for x := range width {
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
	for i := range 4 {
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
	for frame := range 10 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step audio frame %d: %v", frame, err)
		}
		n := audio.DrainMonoF32(samples)
		for i := range n {
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
		for i := range 4 {
			inst, err := cpu.DisassembleInstruction(machine.bus, pc)
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

	for frame := range 20 {
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
	for frame := range 20 {
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
	for frame := range frames {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}
}

func panicRecordSet(t *testing.T, machine *Machine) bool {
	t.Helper()
	value, err := machine.ram.Read(cpu.Long, 0x380)
	if err != nil {
		t.Fatalf("read panic record marker: %v", err)
	}
	return value == 0x12345678
}

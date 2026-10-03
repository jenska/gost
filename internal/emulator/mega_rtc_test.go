package emulator

import (
	"testing"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/config"
	cpu "github.com/jenska/m68kemu"
)

// TestEmuTOSDetectsMegaRTC boots EmuTOS on a Mega ST 2 and checks that it
// probed the RP5C15 the way its detect_megartc does: bank 1 alarm minutes set
// to 10/5.
func TestEmuTOSDetectsMegaRTC(t *testing.T) {
	cfg := config.DefaultConfig()
	preset, ok := config.PresetByID("megast2")
	if !ok {
		t.Fatal("megast2 preset missing")
	}
	preset.Apply(cfg)

	machine, err := NewMachine(cfg, assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if machine.megaRTC == nil {
		t.Fatal("Mega ST preset built a machine without the RP5C15")
	}
	for frame := range 200 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if machine.shifter.ScreenBase() != 0 {
			break
		}
	}

	if err := machine.bus.Write(cpu.Byte, 0xFFFC3B, 0x09); err != nil { // bank 1, timer on
		t.Fatalf("select bank 1: %v", err)
	}
	lo, err := machine.bus.Read(cpu.Byte, 0xFFFC25)
	if err != nil {
		t.Fatalf("read alarm minutes: %v", err)
	}
	hi, err := machine.bus.Read(cpu.Byte, 0xFFFC27)
	if err != nil {
		t.Fatalf("read alarm tens of minutes: %v", err)
	}
	if lo&0x0F != 10 || hi&0x0F != 5 {
		t.Fatalf("bank 1 alarm minutes = %d/%d, want 10/5 left by EmuTOS's clock probe", lo&0x0F, hi&0x0F)
	}
}

func TestNonMegaMachineHasNoMegaRTC(t *testing.T) {
	machine, err := NewMachine(config.DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if machine.megaRTC != nil {
		t.Fatal("default 1040 STF built a machine with the Mega RP5C15")
	}
	v, err := machine.bus.Read(cpu.Byte, 0xFFFC21)
	if err != nil {
		t.Fatalf("read $FFFC21: %v", err)
	}
	if v != 0 {
		t.Fatalf("$FFFC21 = %02x without a Mega RTC, want open bus 00", v)
	}
}

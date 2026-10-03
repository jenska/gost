package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFlagsAndConfigFileAgree checks that every config key yields the same
// Config whether it arrives as a CLI flag or as a JSON config file entry.
func TestFlagsAndConfigFileAgree(t *testing.T) {
	args := []string{
		"--rom=tos.img",
		"--cartridge=cart.bin",
		"--floppy-a=a.st",
		"--floppy-b=b.msa",
		"--hd-size-mb=60",
		"--hd-image=hd.img",
		"--scale=2.5",
		"--fullscreen",
		"--headless",
		"--frames=42",
		"--dump-frame=out.png",
		"--trace=boot",
		"--trace-start=0xE00100",
		"--trace-end=0xE00200",
		"--ram-size=2097152",
		"--clock-hz=8000000",
		"--cpu-clock-hz=16000000",
		"--frame-hz=60",
		"--color-monitor",
		"--rtc",
		"--midres-y-scale=1",
		"--model=ste",
		"--launcher",
	}
	fromFlags, err := Load(args)
	if err != nil {
		t.Fatalf("load from flags: %v", err)
	}

	path := filepath.Join(t.TempDir(), "gost.json")
	data := []byte(`{
		"rom": "tos.img",
		"cartridge": "cart.bin",
		"floppy-a": "a.st",
		"floppy-b": "b.msa",
		"hd-size-mb": 60,
		"hd-image": "hd.img",
		"scale": 2.5,
		"fullscreen": true,
		"headless": true,
		"frames": 42,
		"dump-frame": "out.png",
		"trace": "boot",
		"trace-start": "0xE00100",
		"trace-end": 14680576,
		"ram-size": 2097152,
		"clock-hz": 8000000,
		"cpu-clock-hz": 16000000,
		"frame-hz": 60,
		"color-monitor": true,
		"rtc": true,
		"midres-y-scale": 1,
		"model": "ste",
		"launcher": true
	}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	fromFile, err := Load([]string{"--config", path})
	if err != nil {
		t.Fatalf("load from config file: %v", err)
	}

	if !reflect.DeepEqual(fromFlags, fromFile) {
		t.Fatalf("flags and config file disagree:\nflags: %+v\nfile:  %+v", *fromFlags, *fromFile)
	}
}

func TestCPUMHzAliasMatchesCPUClockHz(t *testing.T) {
	fromMHz, err := Load([]string{"--cpu-mhz=16"})
	if err != nil {
		t.Fatalf("load --cpu-mhz: %v", err)
	}
	fromHz, err := Load([]string{"--cpu-clock-hz=16000000"})
	if err != nil {
		t.Fatalf("load --cpu-clock-hz: %v", err)
	}
	if !reflect.DeepEqual(fromMHz, fromHz) {
		t.Fatalf("cpu-mhz and cpu-clock-hz disagree:\nmhz: %+v\nhz:  %+v", *fromMHz, *fromHz)
	}
}

// TestSavedConfigRoundTripsAllPersistedFields saves a config with every
// persisted field moved off its default and expects the reload to be identical.
func TestSavedConfigRoundTripsAllPersistedFields(t *testing.T) {
	t.Setenv(ConfigDirEnv, t.TempDir())

	cfg := DefaultConfig()
	cfg.Model = MachineModelSTE
	cfg.RAMSize = 4 * 1024 * 1024
	cfg.CPUClockHz = 16_000_000
	cfg.ColorMonitor = true
	cfg.HardDiskSizeMB = 60
	cfg.RTC = true
	cfg.Scale = 2
	cfg.Fullscreen = true
	cfg.ROMPath = "/roms/tos206.img"
	cfg.CartridgePath = "/roms/cart.bin"
	cfg.FloppyA = "/disks/a.st"
	cfg.FloppyB = "/disks/b.msa"
	cfg.HardDiskImagePath = "/disks/hd.img"

	if err := SaveLastConfig(cfg); err != nil {
		t.Fatalf("save last config: %v", err)
	}
	got, ok := LoadLastConfig()
	if !ok {
		t.Fatalf("LoadLastConfig() ok = false after save")
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Fatalf("round trip mismatch:\ngot:  %+v\nwant: %+v", *got, *cfg)
	}
}

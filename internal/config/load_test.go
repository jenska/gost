package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultConfigIsMonochrome1040STF(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Fatalf("Load(nil) = %+v, want DefaultConfig()", *cfg)
	}
	if got := MatchPreset(cfg); got != "1040stf" {
		t.Fatalf("MatchPreset(default) = %q, want 1040stf", got)
	}
	if cfg.ColorMonitor {
		t.Fatalf("expected the default machine to use a monochrome monitor")
	}
	if cfg.MegaRTC || cfg.ICDRTC {
		t.Fatalf("expected the default machine to have no RTC")
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("unexpected hard disk size: got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
	}
	if cfg.Frames != DefaultHeadlessFrames {
		t.Fatalf("unexpected headless frame default: got %d want %d", cfg.Frames, DefaultHeadlessFrames)
	}
}

func TestLoadAppliesPresetBeforeOverrides(t *testing.T) {
	cfg, err := Load([]string{
		"--preset=stf",
		"--ram-size=2097152",
		"--color-monitor=false",
		"--trace-start=0xE12345",
		"--cpu-mhz=12",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.RAMSize != 2*1024*1024 {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, 2*1024*1024)
	}
	if cfg.ColorMonitor {
		t.Fatalf("expected explicit override to disable color monitor")
	}
	if cfg.TraceStart != 0xE12345 {
		t.Fatalf("unexpected trace start: got %06x want %06x", cfg.TraceStart, 0xE12345)
	}
	if cfg.CPUClockHz != 12_000_000 {
		t.Fatalf("unexpected CPU clock: got %d want %d", cfg.CPUClockHz, 12_000_000)
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("expected preset hard disk default to remain enabled, got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
	}
}

func TestLoadCanReadPresetFromConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"preset":"stf"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := MatchPreset(cfg); got != "1040stf" {
		t.Fatalf("unexpected preset: got %q want 1040stf", got)
	}
	if cfg.RAMSize != 1024*1024 {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, 1024*1024)
	}
	if !cfg.ColorMonitor {
		t.Fatalf("expected STF preset to default to color mode")
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("expected ST preset to enable default hard disk, got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
	}
}

func TestLoadCanReadCPUClockHzFromConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"preset":"stf","cpu-clock-hz":32000000}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.CPUClockHz != 32_000_000 {
		t.Fatalf("unexpected CPU clock: got %d want %d", cfg.CPUClockHz, 32_000_000)
	}
}

func TestFlagsOverrideConfigFileSettings(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"preset":"stf","ram-size":524288,"color-monitor":true}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load([]string{
		"--config", configPath,
		"--ram-size=2097152",
		"--color-monitor=false",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.RAMSize != 2*1024*1024 {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, 2*1024*1024)
	}
	if cfg.ColorMonitor {
		t.Fatalf("expected flags to override config file color monitor setting")
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("expected ST preset hard disk default to remain enabled, got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
	}
}

func TestLoadCanEnableRTCFlag(t *testing.T) {
	cfg, err := Load([]string{"--rtc"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.ICDRTC {
		t.Fatalf("expected --rtc to enable ICD RTC support")
	}
}

func TestLoadCanEnableRTCFromConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"rtc":true}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.ICDRTC {
		t.Fatalf("expected JSON rtc option to enable ICD RTC support")
	}
}

func TestLoadCanSetCartridgePath(t *testing.T) {
	cfg, err := Load([]string{"--cartridge", "diag-cart.bin"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CartridgePath != "diag-cart.bin" {
		t.Fatalf("unexpected cartridge path: got %q want diag-cart.bin", cfg.CartridgePath)
	}
}

func TestLoadCanSetCartridgePathFromConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"cartridge":"diag-cart.bin"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CartridgePath != "diag-cart.bin" {
		t.Fatalf("unexpected cartridge path: got %q want diag-cart.bin", cfg.CartridgePath)
	}
}

func TestLoadCanSetFloppyBPath(t *testing.T) {
	cfg, err := Load([]string{"--floppy-b", "disk-b.msa"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.FloppyB != "disk-b.msa" {
		t.Fatalf("unexpected drive B disk path: got %q want disk-b.msa", cfg.FloppyB)
	}
}

func TestLoadCanSetFloppyBPathFromConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"floppy-b":"disk-b.msa"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.FloppyB != "disk-b.msa" {
		t.Fatalf("unexpected drive B disk path: got %q want disk-b.msa", cfg.FloppyB)
	}
}

func TestLoadParsesLauncherFlag(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--launcher"}, true},
		{[]string{"--launcher=false"}, false},
		{[]string{"--preset", "stf", "--launcher"}, true},
	} {
		cfg, err := Load(tc.args)
		if err != nil {
			t.Fatalf("Load(%v): %v", tc.args, err)
		}
		if cfg.Launcher != tc.want {
			t.Errorf("Load(%v).Launcher = %v, want %v", tc.args, cfg.Launcher, tc.want)
		}
	}
}

func TestLoadRejectsUnknownFlagAndPreset(t *testing.T) {
	for _, args := range [][]string{
		{"--no-such-flag"},
		{"--preset", "amiga"},
	} {
		if _, err := Load(args); err == nil {
			t.Errorf("Load(%v) = nil error, want error", args)
		}
	}
}

func TestCLIPresetOverridesConfigFilePreset(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gost.json")
	if err := os.WriteFile(configPath, []byte(`{"preset":"st"}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	cfg, err := Load([]string{"--config", configPath, "--preset", "mega-st"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.RAMSize != 2*1024*1024 {
		t.Fatalf("RAMSize = %d, want the Mega ST preset's 2 MB", cfg.RAMSize)
	}
}

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
		"--mega-rtc",
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
		"mega-rtc": true,
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

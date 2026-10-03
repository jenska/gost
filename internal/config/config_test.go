package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigForPresetDefault(t *testing.T) {
	cfg, err := ConfigForPreset(PresetDefault)
	if err != nil {
		t.Fatalf("config for preset: %v", err)
	}

	if cfg.Preset != PresetDefault {
		t.Fatalf("unexpected preset: got %q want %q", cfg.Preset, PresetDefault)
	}
	if cfg.RAMSize != DefaultRAMSize {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, DefaultRAMSize)
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("unexpected hard disk size: got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
	}
	if cfg.Frames != DefaultHeadlessFrames {
		t.Fatalf("unexpected headless frame default: got %d want %d", cfg.Frames, DefaultHeadlessFrames)
	}
	if cfg.Model != MachineModelST {
		t.Fatalf("unexpected model: got %q want %q", cfg.Model, MachineModelST)
	}
}

func TestConfigForPresetSTF(t *testing.T) {
	cfg, err := ConfigForPreset(PresetSTF)
	if err != nil {
		t.Fatalf("config for preset: %v", err)
	}

	if cfg.Preset != PresetSTF {
		t.Fatalf("unexpected preset: got %q want %q", cfg.Preset, PresetSTF)
	}
	if cfg.RAMSize != STFDefaultRAMSize {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, STFDefaultRAMSize)
	}
	if !cfg.ColorMonitor {
		t.Fatalf("expected STF preset to enable color monitor")
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("unexpected STF hard disk size: got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
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

	if cfg.Preset != PresetSTF {
		t.Fatalf("unexpected preset: got %q want %q", cfg.Preset, PresetSTF)
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

	if cfg.Preset != PresetSTF {
		t.Fatalf("unexpected preset: got %q want %q", cfg.Preset, PresetSTF)
	}
	if cfg.RAMSize != STFDefaultRAMSize {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, STFDefaultRAMSize)
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

	if cfg.Preset != PresetSTF {
		t.Fatalf("unexpected preset: got %q want %q", cfg.Preset, PresetSTF)
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

func TestConfigForPresetMegaST(t *testing.T) {
	cfg, err := ConfigForPreset(PresetMegaST)
	if err != nil {
		t.Fatalf("config for preset: %v", err)
	}

	if cfg.Preset != PresetMegaST {
		t.Fatalf("unexpected preset: got %q want %q", cfg.Preset, PresetMegaST)
	}
	if cfg.RAMSize != MegaSTDefaultRAMSize {
		t.Fatalf("unexpected RAM size: got %d want %d", cfg.RAMSize, MegaSTDefaultRAMSize)
	}
	if cfg.ColorMonitor {
		t.Fatalf("expected Mega ST preset to default to monochrome mode")
	}
	if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
		t.Fatalf("expected Mega ST preset to enable default hard disk, got %d want %d", cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
	}
	if cfg.Model != MachineModelST {
		t.Fatalf("unexpected model: got %q want %q", cfg.Model, MachineModelST)
	}
	if cfg.RTC {
		t.Fatalf("expected Mega ST preset to keep optional RTC disabled by default")
	}
}

func TestLoadCanEnableRTCFlag(t *testing.T) {
	cfg, err := Load([]string{"--rtc"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.RTC {
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
	if !cfg.RTC {
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

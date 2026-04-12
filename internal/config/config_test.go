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
	if cfg.HardDiskSizeMB != 0 {
		t.Fatalf("expected STF preset to disable hard disk, got %d", cfg.HardDiskSizeMB)
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
	if cfg.HardDiskSizeMB != 0 {
		t.Fatalf("expected preset hard disk default to remain disabled, got %d", cfg.HardDiskSizeMB)
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
	if cfg.HardDiskSizeMB != 0 {
		t.Fatalf("expected ST preset to disable hard disk, got %d", cfg.HardDiskSizeMB)
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
	if cfg.HardDiskSizeMB != 0 {
		t.Fatalf("expected ST preset hard disk default to remain disabled, got %d", cfg.HardDiskSizeMB)
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
	if cfg.HardDiskSizeMB != 0 {
		t.Fatalf("expected Mega ST preset to disable hard disk, got %d", cfg.HardDiskSizeMB)
	}
	if cfg.Model != MachineModelST {
		t.Fatalf("unexpected model: got %q want %q", cfg.Model, MachineModelST)
	}
}

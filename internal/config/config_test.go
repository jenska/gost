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

func TestLoadROMPadsOddLengthImages(t *testing.T) {
	path := writeTempROM(t, []byte{0x12, 0x34, 0x56})

	image, err := LoadROM(path)
	if err != nil {
		t.Fatalf("load ROM: %v", err)
	}
	if got, want := len(image), 4; got != want {
		t.Fatalf("unexpected ROM length: got %d want %d", got, want)
	}
	if image[3] != 0xFF {
		t.Fatalf("unexpected padded byte: got %02x want ff", image[3])
	}
}

func TestLoadROMKeepsEvenLengthImages(t *testing.T) {
	path := writeTempROM(t, []byte{0x12, 0x34})

	image, err := LoadROM(path)
	if err != nil {
		t.Fatalf("load ROM: %v", err)
	}
	if got, want := len(image), 2; got != want {
		t.Fatalf("unexpected ROM length: got %d want %d", got, want)
	}
}

func writeTempROM(t *testing.T, data []byte) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "rom-*.img")
	if err != nil {
		t.Fatalf("create temp ROM: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatalf("write temp ROM: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp ROM: %v", err)
	}
	return file.Name()
}

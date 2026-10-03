package config

import (
	"reflect"
	"testing"
)

func TestProfileSaveListLoadRoundTrip(t *testing.T) {
	t.Setenv(ConfigDirEnv, t.TempDir())

	cfg := DefaultConfig()
	cfg.RAMSize = 2 * 1024 * 1024
	cfg.CPUClockHz = 16_000_000
	cfg.ColorMonitor = false
	cfg.FloppyA = "/disks/a.stx"
	cfg.ICDRTC = true

	if err := SaveProfile("my-ste", cfg); err != nil {
		t.Fatalf("save profile: %v", err)
	}

	names, err := ListProfiles()
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"my-ste"}) {
		t.Fatalf("ListProfiles() = %v, want [my-ste]", names)
	}

	loaded, err := LoadProfile("my-ste")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	for _, tc := range []struct {
		name string
		got  any
		want any
	}{
		{"RAMSize", loaded.RAMSize, cfg.RAMSize},
		{"CPUClockHz", loaded.CPUClockHz, cfg.CPUClockHz},
		{"ColorMonitor", loaded.ColorMonitor, cfg.ColorMonitor},
		{"FloppyA", loaded.FloppyA, cfg.FloppyA},
		{"RTC", loaded.ICDRTC, cfg.ICDRTC},
		{"Model", loaded.Model, cfg.Model},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	if err := DeleteProfile("my-ste"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	if names, _ := ListProfiles(); len(names) != 0 {
		t.Fatalf("profile still listed after delete: %v", names)
	}
}

func TestListProfilesMissingDir(t *testing.T) {
	t.Setenv(ConfigDirEnv, t.TempDir())
	names, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v, want nil", err)
	}
	if len(names) != 0 {
		t.Fatalf("ListProfiles() = %v, want empty", names)
	}
}

func TestSaveProfileRejectsBadNames(t *testing.T) {
	t.Setenv(ConfigDirEnv, t.TempDir())
	for _, name := range []string{"", "../evil", "a/b", ".hidden"} {
		if err := SaveProfile(name, DefaultConfig()); err == nil {
			t.Errorf("SaveProfile(%q) = nil, want error", name)
		}
	}
}

func TestLastConfigRoundTrip(t *testing.T) {
	t.Setenv(ConfigDirEnv, t.TempDir())

	if _, ok := LoadLastConfig(); ok {
		t.Fatalf("LoadLastConfig() ok = true with no stored config")
	}

	cfg := DefaultConfig()
	cfg.RAMSize = 512 * 1024
	if err := SaveLastConfig(cfg); err != nil {
		t.Fatalf("save last config: %v", err)
	}
	got, ok := LoadLastConfig()
	if !ok {
		t.Fatalf("LoadLastConfig() ok = false after save")
	}
	if got.RAMSize != cfg.RAMSize {
		t.Fatalf("last config RAM = %d, want %d", got.RAMSize, cfg.RAMSize)
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
	cfg.ICDRTC = true
	cfg.MegaRTC = true
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

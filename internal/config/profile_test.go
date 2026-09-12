package config

import (
	"reflect"
	"testing"
)

func TestProfileSaveListLoadRoundTrip(t *testing.T) {
	t.Setenv(ConfigDirEnv, t.TempDir())

	cfg, err := ConfigForPreset(PresetSTF)
	if err != nil {
		t.Fatalf("config for preset: %v", err)
	}
	cfg.RAMSize = 2 * 1024 * 1024
	cfg.CPUClockHz = 16_000_000
	cfg.ColorMonitor = false
	cfg.FloppyA = "/disks/a.stx"
	cfg.RTC = true

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
		{"RTC", loaded.RTC, cfg.RTC},
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

func TestShouldShowLauncher(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{"--launcher"}, true},
		{[]string{"--config", "x.json"}, false},
		{[]string{"--preset", "stf"}, false},
		{[]string{"--launcher=false"}, false},
		{[]string{"--preset", "stf", "--launcher"}, true},
	} {
		if got := ShouldShowLauncher(tc.args); got != tc.want {
			t.Errorf("ShouldShowLauncher(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestPresetCatalogueApplyAndMatch(t *testing.T) {
	for _, preset := range MachinePresets {
		cfg, err := ConfigForPreset(PresetDefault)
		if err != nil {
			t.Fatalf("config for preset: %v", err)
		}
		preset.Apply(cfg)
		if got := MatchPreset(cfg); got != preset.ID {
			t.Errorf("MatchPreset after Apply(%s) = %q, want %q", preset.ID, got, preset.ID)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("preset %s produced invalid config: %v", preset.ID, err)
		}
	}
}

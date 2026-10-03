package config

import (
	"strings"
	"testing"
)

func TestPresetFlagAppliesCatalogueEntry(t *testing.T) {
	for _, tc := range []struct {
		preset  string
		want    string
		ram     uint32
		color   bool
		megaRTC bool
	}{
		{"default", "1040stf", 1024 * 1024, false, false},
		{"1040stf", "1040stf", 1024 * 1024, true, false},
		{"stf", "1040stf", 1024 * 1024, true, false},
		{"st", "520st", 512 * 1024, true, false},
		{"mega-st", "megast2", 2 * 1024 * 1024, false, true},
		{"MegaSTE", "megaste", 4 * 1024 * 1024, false, true},
	} {
		cfg, err := Load([]string{"--preset", tc.preset})
		if err != nil {
			t.Fatalf("--preset %s: %v", tc.preset, err)
		}
		if got := MatchPreset(cfg); got != tc.want {
			t.Errorf("--preset %s: MatchPreset = %q, want %q", tc.preset, got, tc.want)
		}
		if cfg.RAMSize != tc.ram || cfg.ColorMonitor != tc.color || cfg.MegaRTC != tc.megaRTC {
			t.Errorf("--preset %s: RAM %d color %v mega-rtc %v, want %d %v %v",
				tc.preset, cfg.RAMSize, cfg.ColorMonitor, cfg.MegaRTC, tc.ram, tc.color, tc.megaRTC)
		}
		if cfg.HardDiskSizeMB != DefaultHardDiskSizeMB {
			t.Errorf("--preset %s: hard disk size %d, want %d", tc.preset, cfg.HardDiskSizeMB, DefaultHardDiskSizeMB)
		}
	}
}

func TestMegaPresetsHaveBuiltInRTC(t *testing.T) {
	for _, p := range MachinePresets {
		mega := strings.HasPrefix(p.ID, "mega")
		if p.MegaRTC != mega {
			t.Errorf("preset %s: MegaRTC = %v, want %v", p.ID, p.MegaRTC, mega)
		}
	}
}

func TestPresetCatalogueApplyAndMatch(t *testing.T) {
	for _, preset := range MachinePresets {
		cfg := DefaultConfig()
		preset.Apply(cfg)
		if got := MatchPreset(cfg); got != preset.ID {
			t.Errorf("MatchPreset after Apply(%s) = %q, want %q", preset.ID, got, preset.ID)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("preset %s produced invalid config: %v", preset.ID, err)
		}
	}
}

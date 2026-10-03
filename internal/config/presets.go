package config

import (
	"fmt"
	"strings"
)

// MachinePreset is an Atari ST machine selectable with --preset and in the
// desktop launcher. Applying one sets the machine fields of a Config; the
// individual RAM, CPU, monitor, and image fields can still be overridden
// afterwards.
type MachinePreset struct {
	// ID is the --preset value and the stable identifier of the machine.
	ID string
	// Label is the human-readable name shown in the launcher.
	Label string
	// Model is the machine type (st or ste).
	Model MachineModel
	// RAMSize is the emulated RAM size in bytes.
	RAMSize uint32
	// CPUClockHz is the emulated CPU clock frequency in Hz.
	CPUClockHz uint64
	// MegaRTC marks the Mega models' built-in RP5C15 real-time clock.
	MegaRTC bool
	// ColorMonitor is the monitor the machine is set up with. Every model
	// drives both monitors, so it is not part of what identifies the machine.
	ColorMonitor bool
	// Note is a short hint (RAM, typical TOS versions) shown beside the label.
	Note string
}

// MachinePresets is the preset catalogue, roughly ordered from the smallest
// classic ST to the Mega STE.
var MachinePresets = []MachinePreset{
	{
		ID: "520st", Label: "Atari 520 ST", Model: MachineModelST,
		RAMSize: 512 * 1024, CPUClockHz: DefaultClockHz, ColorMonitor: true,
		Note: "512 KB, TOS 1.00-1.04",
	},
	{
		ID: "1040stf", Label: "Atari 1040 STF", Model: MachineModelST,
		RAMSize: 1024 * 1024, CPUClockHz: DefaultClockHz, ColorMonitor: true,
		Note: "1 MB, TOS 1.02-1.04",
	},
	{
		ID: "1040ste", Label: "Atari 1040 STE", Model: MachineModelSTE,
		RAMSize: 1024 * 1024, CPUClockHz: DefaultClockHz, ColorMonitor: true,
		Note: "1 MB, TOS 1.06-1.62",
	},
	{
		ID: "megast1", Label: "Atari Mega ST 1", Model: MachineModelST,
		RAMSize: 1024 * 1024, CPUClockHz: DefaultClockHz, MegaRTC: true,
		Note: "1 MB, TOS 1.02-1.04",
	},
	{
		ID: "megast2", Label: "Atari Mega ST 2", Model: MachineModelST,
		RAMSize: 2 * 1024 * 1024, CPUClockHz: DefaultClockHz, MegaRTC: true,
		Note: "2 MB, TOS 1.02-1.04",
	},
	{
		ID: "megast4", Label: "Atari Mega ST 4", Model: MachineModelST,
		RAMSize: 4 * 1024 * 1024, CPUClockHz: DefaultClockHz, MegaRTC: true,
		Note: "4 MB, TOS 1.02-1.04",
	},
	{
		ID: "megaste", Label: "Atari Mega STE", Model: MachineModelSTE,
		RAMSize: 4 * 1024 * 1024, CPUClockHz: 16_000_000, MegaRTC: true,
		Note: "4 MB, 16 MHz, TOS 2.05-2.06",
	},
}

// presetDefault selects the plain defaults: a 1040 STF on a monochrome monitor.
const presetDefault = "default"

// presetAliases keeps the preset names used before the catalogue existed
// working on the command line and in old config files.
var presetAliases = map[string]string{
	"stf":     "1040stf",
	"st":      "520st",
	"mega-st": "megast2",
}

// PresetByID returns the catalogue entry with the given ID or legacy alias,
// ignoring case and surrounding space.
func PresetByID(id string) (MachinePreset, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	if alias, ok := presetAliases[id]; ok {
		id = alias
	}
	for _, p := range MachinePresets {
		if p.ID == id {
			return p, true
		}
	}
	return MachinePreset{}, false
}

// MatchPreset returns the ID of the catalogue entry describing cfg's machine,
// or "" when the configuration is custom. The monitor is not compared.
func MatchPreset(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	for _, p := range MachinePresets {
		if p.Model == cfg.Model &&
			p.RAMSize == cfg.RAMSize &&
			p.CPUClockHz == cfg.CPUClockHz &&
			p.MegaRTC == cfg.MegaRTC {
			return p.ID
		}
	}
	return ""
}

// Apply writes the preset's machine settings into cfg, leaving image paths and
// other unrelated fields untouched.
func (p MachinePreset) Apply(cfg *Config) {
	cfg.Model = p.Model
	cfg.RAMSize = p.RAMSize
	cfg.CPUClockHz = p.CPUClockHz
	cfg.MegaRTC = p.MegaRTC
	cfg.ColorMonitor = p.ColorMonitor
}

// configForPreset returns the defaults with the named preset applied. An empty
// name or "default" yields DefaultConfig.
func configForPreset(name string) (*Config, error) {
	cfg := DefaultConfig()
	if id := strings.ToLower(strings.TrimSpace(name)); id == "" || id == presetDefault {
		return cfg, nil
	}
	p, ok := PresetByID(name)
	if !ok {
		return nil, fmt.Errorf("unsupported preset %q", name)
	}
	p.Apply(cfg)
	return cfg, nil
}

// presetUsage lists the accepted --preset values for the -help text.
func presetUsage() string {
	ids := []string{presetDefault}
	for _, p := range MachinePresets {
		ids = append(ids, p.ID)
	}
	return "machine preset: " + strings.Join(ids, "|") + " (legacy: st, stf, mega-st)"
}

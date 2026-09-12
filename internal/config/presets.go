package config

// MachinePreset is a user-facing Atari ST configuration offered by the desktop
// launcher. Selecting one seeds a Config; the individual RAM, CPU, monitor, and
// image fields can then still be overridden before boot.
type MachinePreset struct {
	// ID is the stable identifier stored with a saved profile selection.
	ID string
	// Label is the human-readable name shown in the launcher.
	Label string
	// Base is the underlying config preset the machine derives from.
	Base Preset
	// Model is the machine type (st or ste).
	Model MachineModel
	// RAMSize is the emulated RAM size in bytes.
	RAMSize uint32
	// ColorMonitor selects a colour monitor rather than the monochrome monitor.
	ColorMonitor bool
	// CPUClockHz is the emulated CPU clock frequency in Hz.
	CPUClockHz uint64
	// Note is a short hint (RAM, typical TOS versions) shown beside the label.
	Note string
}

// MachinePresets is the catalogue shown in the launcher, roughly ordered from
// the smallest classic ST to the Mega STE.
var MachinePresets = []MachinePreset{
	{
		ID: "520st", Label: "Atari 520 ST", Base: PresetST, Model: MachineModelST,
		RAMSize: 512 * 1024, ColorMonitor: true, CPUClockHz: DefaultClockHz,
		Note: "512 KB, TOS 1.00-1.04",
	},
	{
		ID: "1040stf", Label: "Atari 1040 STF", Base: PresetSTF, Model: MachineModelST,
		RAMSize: 1024 * 1024, ColorMonitor: true, CPUClockHz: DefaultClockHz,
		Note: "1 MB, TOS 1.02-1.04",
	},
	{
		ID: "1040ste", Label: "Atari 1040 STE", Base: PresetSTF, Model: MachineModelSTE,
		RAMSize: 1024 * 1024, ColorMonitor: true, CPUClockHz: DefaultClockHz,
		Note: "1 MB, TOS 1.06-1.62",
	},
	{
		ID: "megast2", Label: "Atari Mega ST 2", Base: PresetMegaST, Model: MachineModelST,
		RAMSize: 2 * 1024 * 1024, ColorMonitor: false, CPUClockHz: DefaultClockHz,
		Note: "2 MB, TOS 1.02-1.04",
	},
	{
		ID: "megast4", Label: "Atari Mega ST 4", Base: PresetMegaST, Model: MachineModelST,
		RAMSize: 4 * 1024 * 1024, ColorMonitor: false, CPUClockHz: DefaultClockHz,
		Note: "4 MB, TOS 1.02-1.04",
	},
	{
		ID: "megaste", Label: "Atari Mega STE", Base: PresetMegaST, Model: MachineModelSTE,
		RAMSize: 4 * 1024 * 1024, ColorMonitor: false, CPUClockHz: 16_000_000,
		Note: "4 MB, 16 MHz, TOS 2.05-2.06",
	},
}

// PresetByID returns the catalogue entry with the given ID.
func PresetByID(id string) (MachinePreset, bool) {
	for _, p := range MachinePresets {
		if p.ID == id {
			return p, true
		}
	}
	return MachinePreset{}, false
}

// MatchPreset returns the ID of the catalogue entry whose machine settings match
// cfg, or "" when the configuration is custom.
func MatchPreset(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	for _, p := range MachinePresets {
		if p.Model == cfg.Model &&
			p.RAMSize == cfg.RAMSize &&
			p.ColorMonitor == cfg.ColorMonitor &&
			p.CPUClockHz == cfg.CPUClockHz {
			return p.ID
		}
	}
	return ""
}

// Apply writes the preset's machine settings into cfg, leaving image paths and
// other unrelated fields untouched.
func (p MachinePreset) Apply(cfg *Config) {
	cfg.Preset = p.Base
	cfg.Model = p.Model
	cfg.RAMSize = p.RAMSize
	cfg.ColorMonitor = p.ColorMonitor
	cfg.CPUClockHz = p.CPUClockHz
}

// RAMSizeChoice is one selectable RAM size for the launcher dropdown.
type RAMSizeChoice struct {
	Label string
	Bytes uint32
}

// RAMSizeChoices are the RAM sizes offered by the launcher.
var RAMSizeChoices = []RAMSizeChoice{
	{Label: "256 KB", Bytes: 256 * 1024},
	{Label: "512 KB", Bytes: 512 * 1024},
	{Label: "1 MB", Bytes: 1024 * 1024},
	{Label: "2 MB", Bytes: 2 * 1024 * 1024},
	{Label: "2.5 MB", Bytes: 2560 * 1024},
	{Label: "4 MB", Bytes: 4 * 1024 * 1024},
}

// CPUClockChoice is one selectable CPU speed for the launcher dropdown.
type CPUClockChoice struct {
	Label string
	Hz    uint64
}

// CPUClockChoices are the CPU speeds offered by the launcher.
var CPUClockChoices = []CPUClockChoice{
	{Label: "8 MHz (stock)", Hz: 8_000_000},
	{Label: "16 MHz", Hz: 16_000_000},
	{Label: "32 MHz", Hz: 32_000_000},
}

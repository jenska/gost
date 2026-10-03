package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultRAMSize        = 1024 * 1024
	DefaultClockHz        = 8_000_000
	DefaultFrameHz        = 50
	DefaultHardDiskSizeMB = 30
	DefaultHeadlessFrames = 1000

	STDefaultRAMSize     = 512 * 1024
	STFDefaultRAMSize    = 1024 * 1024
	MegaSTDefaultRAMSize = 2 * 1024 * 1024
)

type MachineModel string

const (
	MachineModelST  MachineModel = "st"
	MachineModelSTE MachineModel = "ste"

	bootTraceStart = 0xE00000
	bootTraceEnd   = 0xE01000
)

type Preset string

const (
	PresetDefault Preset = "default"
	PresetSTF     Preset = "stf"
	PresetST      Preset = "st"
	PresetMegaST  Preset = "mega-st"
)

const (
	KeyConfig         = "config"
	KeyPreset         = "preset"
	KeyROM            = "rom"
	KeyCartridge      = "cartridge"
	KeyFloppyA        = "floppy-a"
	KeyFloppyB        = "floppy-b"
	KeyHardDiskSizeMB = "hd-size-mb"
	KeyHardDiskImage  = "hd-image"
	KeyScale          = "scale"
	KeyFullscreen     = "fullscreen"
	KeyHeadless       = "headless"
	KeyFrames         = "frames"
	KeyDumpFrame      = "dump-frame"
	KeyTrace          = "trace"
	KeyTraceStart     = "trace-start"
	KeyTraceEnd       = "trace-end"
	KeyRAMSize        = "ram-size"
	KeyClockHz        = "clock-hz"
	KeyCPUMHz         = "cpu-mhz"
	KeyCPUClockHz     = "cpu-clock-hz"
	KeyFrameHz        = "frame-hz"
	KeyColorMonitor   = "color-monitor"
	KeyRTC            = "rtc"
	KeyMidResYScale   = "midres-y-scale"
	KeyModel          = "model"
	KeyLauncher       = "launcher"
)

// Config holds all configuration parameters for the Atari ST emulation.
type Config struct {
	// Preset is the initial configuration preset to load (default, stf, st, mega-st).
	Preset Preset
	// ROMPath is the path to a custom TOS ROM file; if empty, uses bundled EmuTOS.
	ROMPath string
	// CartridgePath is the path to an optional cartridge ROM image.
	CartridgePath string
	// FloppyA is the path to a disk image to insert into floppy drive A.
	FloppyA string
	// FloppyB is the path to a disk image to insert into floppy drive B.
	FloppyB string
	// HardDiskSizeMB is the virtual hard disk size in megabytes (0 disables hard disk).
	HardDiskSizeMB uint32
	// HardDiskImagePath is the file path for persistent virtual hard disk storage.
	HardDiskImagePath string
	// Scale is the UI scaling factor for the display window.
	Scale float64
	// Fullscreen enables fullscreen display mode.
	Fullscreen bool
	// Headless enables headless execution mode (no UI window).
	Headless bool
	// Frames specifies how many frames to run in headless mode before exit.
	Frames int
	// DumpFramePath is the output path for frame dumps (PNG format).
	DumpFramePath string
	// Trace specifies the trace mode: "cpu", "cpu-verbose", "boot", "boot-verbose", "shifter", "shifter-verbose".
	Trace string
	// TraceStart is the CPU address where detailed tracing begins.
	TraceStart uint32
	// TraceEnd is the CPU address where detailed tracing ends.
	TraceEnd uint32
	// RAMSize is the emulated machine RAM size in bytes.
	RAMSize uint32
	// ClockHz is the emulated system clock frequency in Hz.
	ClockHz uint64
	// CPUClockHz is the emulated CPU clock frequency in Hz.
	CPUClockHz uint64
	// FrameHz is the emulated display refresh rate in Hz (typically 50 or 60).
	FrameHz uint64
	// ColorMonitor enables color monitor mode; false uses monochrome mode.
	ColorMonitor bool
	// RTC enables the ICD-compatible ACSI real-time clock.
	RTC bool
	// MidResYScale applies Y-axis pixel doubling for medium resolution mode.
	MidResYScale int
	// Model specifies the machine type: st or ste.
	Model MachineModel
	// Launcher forces the desktop configuration launcher to open before boot.
	Launcher bool
}

type configPatch map[string]json.RawMessage

func DefaultConfig() *Config {
	cfg, err := ConfigForPreset(PresetDefault)
	if err != nil {
		panic(err)
	}
	return cfg
}

func (cfg *Config) FrameCycles() uint64 {
	if cfg == nil || cfg.ClockHz == 0 || cfg.FrameHz == 0 {
		return 0
	}
	return cfg.ClockHz / cfg.FrameHz
}

func ConfigForPreset(preset Preset) (*Config, error) {
	normalized, err := normalizePreset(preset)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	cfg.Scale = 1.0
	cfg.Frames = DefaultHeadlessFrames
	cfg.TraceStart = bootTraceStart
	cfg.TraceEnd = bootTraceEnd
	cfg.RAMSize = DefaultRAMSize
	cfg.ClockHz = DefaultClockHz
	cfg.CPUClockHz = DefaultClockHz
	cfg.FrameHz = DefaultFrameHz
	cfg.HardDiskSizeMB = DefaultHardDiskSizeMB
	cfg.ColorMonitor = false
	cfg.RTC = false
	cfg.MidResYScale = 2
	cfg.Model = MachineModelST

	cfg.Preset = normalized
	switch normalized {
	case PresetSTF:
		cfg.Model = MachineModelST
		cfg.RAMSize = STFDefaultRAMSize
		cfg.ColorMonitor = true
	case PresetST:
		cfg.Model = MachineModelST
		cfg.RAMSize = STDefaultRAMSize
		cfg.ColorMonitor = false
	case PresetMegaST:
		cfg.Model = MachineModelST
		cfg.RAMSize = MegaSTDefaultRAMSize
		cfg.ColorMonitor = false
	default:
	}
	return cfg, nil
}

func NewConfig() (*Config, error) {
	return Load(os.Args[1:])
}

func Load(args []string) (*Config, error) {
	if args == nil {
		args = []string{}
	}
	if containsHelpArg(args) {
		return nil, parseFlags(DefaultConfig(), args)
	}

	configPath, _ := lookupFlagValue(args, KeyConfig)
	patch, err := loadConfigPatch(configPath)
	if err != nil {
		return nil, err
	}

	preset, err := selectPreset(args, patch)
	if err != nil {
		return nil, err
	}

	cfg, err := ConfigForPreset(preset)
	if err != nil {
		return nil, err
	}
	if err := patch.Apply(cfg); err != nil {
		return nil, err
	}
	if err := parseFlags(cfg, args); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadConfigFile builds a validated Config from a single JSON file, applying the
// preset it names (if any) before its remaining keys. Unlike Load it consults no
// CLI arguments, so it is the path used for saved profiles and the last-used
// config.
func LoadConfigFile(path string) (*Config, error) {
	patch, err := loadConfigPatch(path)
	if err != nil {
		return nil, err
	}

	preset := PresetDefault
	if named, ok, err := patch.Preset(); err != nil {
		return nil, err
	} else if ok {
		if preset, err = normalizePreset(named); err != nil {
			return nil, err
		}
	}

	cfg, err := ConfigForPreset(preset)
	if err != nil {
		return nil, err
	}
	if err := patch.Apply(cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	preset, err := normalizePreset(cfg.Preset)
	if err != nil {
		return err
	}
	model, err := normalizeModel(cfg.Model)
	if err != nil {
		return err
	}
	cfg.Preset = preset
	cfg.Model = model

	if cfg.Scale <= 0 {
		return fmt.Errorf("invalid scale %.3f: must be > 0", cfg.Scale)
	}
	if cfg.RAMSize == 0 {
		return fmt.Errorf("invalid ram-size %d: must be > 0", cfg.RAMSize)
	}
	if cfg.ClockHz == 0 {
		return fmt.Errorf("invalid clock-hz %d: must be > 0", cfg.ClockHz)
	}
	if cfg.FrameHz == 0 {
		return fmt.Errorf("invalid frame-hz %d: must be > 0", cfg.FrameHz)
	}
	if cfg.CPUClockHz == 0 {
		return fmt.Errorf("invalid %s %d: must be > 0", KeyCPUClockHz, cfg.CPUClockHz)
	}
	if cfg.MidResYScale < 1 {
		return fmt.Errorf("invalid midres-y-scale %d: must be >= 1", cfg.MidResYScale)
	}
	if cfg.Frames < 0 {
		return fmt.Errorf("invalid frames %d: must be >= 0", cfg.Frames)
	}
	if cfg.FrameCycles() == 0 {
		return fmt.Errorf("invalid frame timing: clock-hz %d / frame-hz %d yields 0 frame cycles", cfg.ClockHz, cfg.FrameHz)
	}
	return nil
}

func loadConfigPatch(path string) (configPatch, error) {
	if path == "" {
		return configPatch{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	patch := configPatch{}
	if err := decoder.Decode(&patch); err != nil {
		return nil, err
	}
	return patch, nil
}

// Apply sets every key in the patch on cfg through the same flag.Value that
// parses the key on the command line.
func (p configPatch) Apply(cfg *Config) error {
	for key, raw := range p {
		if key == KeyPreset {
			continue // the preset seeds the defaults before the patch applies
		}
		f, ok := lookupField(key)
		if !ok {
			return fmt.Errorf("unsupported config key %q", key)
		}
		text, err := jsonScalarText(raw)
		if err != nil {
			return fmt.Errorf("decode %q: %w", key, err)
		}
		if err := f.value(cfg).Set(text); err != nil {
			return fmt.Errorf("decode %q: %w", key, err)
		}
	}
	return nil
}

// jsonScalarText returns a JSON string, number, or boolean as the text a flag
// would carry: strings unquoted, numbers and booleans verbatim.
func jsonScalarText(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", fmt.Errorf("empty value")
	}
	switch raw[0] {
	case '"':
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", err
		}
		return text, nil
	case '{', '[', 'n':
		return "", fmt.Errorf("expected a string, number, or boolean, got %s", raw)
	default:
		return string(raw), nil
	}
}

func (p configPatch) Preset() (Preset, bool, error) {
	raw, ok := p[KeyPreset]
	if !ok {
		return "", false, nil
	}

	var preset Preset
	if err := json.Unmarshal(raw, &preset); err != nil {
		return "", false, fmt.Errorf("decode %q: %w", KeyPreset, err)
	}
	return preset, true, nil
}

func selectPreset(args []string, patch configPatch) (Preset, error) {
	if raw, ok := lookupFlagValue(args, KeyPreset); ok {
		return normalizePreset(Preset(raw))
	}
	if preset, ok, err := patch.Preset(); err != nil {
		return "", err
	} else if ok {
		return normalizePreset(preset)
	}
	return PresetDefault, nil
}

func parseFlags(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("gost", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	preset := string(cfg.Preset)
	fs.String(KeyConfig, "", "optional JSON config file loaded before CLI overrides")
	fs.StringVar(&preset, KeyPreset, preset, "machine preset: default|stf|st|mega-st")
	for _, f := range fields {
		fs.Var(f.value(cfg), f.key, f.usage)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.Preset = Preset(preset)
	return nil
}

func normalizePreset(preset Preset) (Preset, error) {
	switch normalized := Preset(strings.ToLower(strings.TrimSpace(string(preset)))); normalized {
	case "", PresetDefault:
		return PresetDefault, nil
	case PresetSTF:
		return PresetSTF, nil
	case PresetST:
		return PresetST, nil
	case PresetMegaST:
		return PresetMegaST, nil
	default:
		return "", fmt.Errorf("unsupported preset %q", preset)
	}
}

func normalizeModel(model MachineModel) (MachineModel, error) {
	switch normalized := MachineModel(strings.ToLower(strings.TrimSpace(string(model)))); normalized {
	case "", MachineModelST:
		return MachineModelST, nil
	case MachineModelSTE:
		return MachineModelSTE, nil
	default:
		return "", fmt.Errorf("unsupported machine model %q", model)
	}
}

func lookupFlagValue(args []string, name string) (string, bool) {
	longName := "--" + name
	shortName := "-" + name

	for i := range args {
		arg := args[i]
		switch {
		case arg == longName || arg == shortName:
			if i+1 >= len(args) {
				return "", false
			}
			return args[i+1], true
		case strings.HasPrefix(arg, longName+"="):
			return strings.TrimPrefix(arg, longName+"="), true
		case strings.HasPrefix(arg, shortName+"="):
			return strings.TrimPrefix(arg, shortName+"="), true
		}
	}

	return "", false
}

func containsHelpArg(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}

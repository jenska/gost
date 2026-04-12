package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultRAMSize        = 1024 * 1024
	DefaultClockHz        = 8_000_000
	DefaultFrameHz        = 50
	DefaultHardDiskSizeMB = 30
	DefaultHeadlessFrames = 500

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

type PresetDefinition struct {
	Name        Preset
	Description string
}

const (
	KeyConfig         = "config"
	KeyPreset         = "preset"
	KeyROM            = "rom"
	KeyFloppyA        = "floppy-a"
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
	KeyMidResYScale   = "midres-y-scale"
	KeyModel          = "model"
)

var presetDefinitions = []PresetDefinition{
	{
		Name:        PresetDefault,
		Description: "development-friendly defaults with 1 MiB RAM and a 30 MiB virtual hard disk",
	},
	{
		Name:        PresetSTF,
		Description: "Atari STF baseline with ST timing, 1 MiB RAM, color monitor, and no hard disk",
	},
	{
		Name:        PresetST,
		Description: "Atari ST baseline with ST timing, 512 KiB RAM, monochrome monitor, and no hard disk",
	},
	{
		Name:        PresetMegaST,
		Description: "Atari Mega ST baseline with ST timing, 2 MiB RAM, monochrome monitor, and no hard disk",
	},
}

type Config struct {
	Preset            Preset
	ROMPath           string
	FloppyA           string
	HardDiskSizeMB    uint32
	HardDiskImagePath string
	Scale             float64
	Fullscreen        bool
	Headless          bool
	Frames            int
	DumpFramePath     string
	Trace             string
	TraceStart        uint32
	TraceEnd          uint32
	RAMSize           uint32
	ClockHz           uint64
	CPUClockHz        uint64
	FrameHz           uint64
	ColorMonitor      bool
	MidResYScale      int
	Model             MachineModel
}

type configPatch map[string]json.RawMessage

type addressFlag struct {
	target *uint32
}

type uint32Flag struct {
	target *uint32
}

type mhzFlag struct {
	target *uint64
}

func (f addressFlag) String() string {
	if f.target == nil {
		return ""
	}
	return formatAddress(*f.target)
}

func (f addressFlag) Set(raw string) error {
	value, err := parseAddress(raw)
	if err != nil {
		return err
	}
	*f.target = value
	return nil
}

func (f uint32Flag) String() string {
	if f.target == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*f.target), 10)
}

func (f uint32Flag) Set(raw string) error {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 0, 32)
	if err != nil {
		return err
	}
	*f.target = uint32(value)
	return nil
}

func (f mhzFlag) String() string {
	if f.target == nil {
		return ""
	}
	return strconv.FormatFloat(float64(*f.target)/1_000_000.0, 'f', -1, 64)
}

func (f mhzFlag) Set(raw string) error {
	hz, err := parseMHz(raw)
	if err != nil {
		return err
	}
	*f.target = hz
	return nil
}

func Presets() []PresetDefinition {
	definitions := make([]PresetDefinition, len(presetDefinitions))
	copy(definitions, presetDefinitions)
	return definitions
}

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
	cfg.MidResYScale = 2
	cfg.Model = MachineModelST

	cfg.Preset = normalized
	switch normalized {
	case PresetSTF:
		cfg.Model = MachineModelST
		cfg.RAMSize = STFDefaultRAMSize
		cfg.ColorMonitor = true
		cfg.HardDiskSizeMB = 0
	case PresetST:
		cfg.Model = MachineModelST
		cfg.RAMSize = STDefaultRAMSize
		cfg.ColorMonitor = false
		cfg.HardDiskSizeMB = 0
	case PresetMegaST:
		cfg.Model = MachineModelST
		cfg.RAMSize = MegaSTDefaultRAMSize
		cfg.ColorMonitor = false
		cfg.HardDiskSizeMB = 0
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

func (p configPatch) Apply(cfg *Config) error {
	for key, raw := range p {
		switch key {
		case KeyPreset:
			// Preset is selected before defaults are built, so ignore it here.
		case KeyROM:
			if err := decodeJSON(raw, &cfg.ROMPath); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyFloppyA:
			if err := decodeJSON(raw, &cfg.FloppyA); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyHardDiskSizeMB:
			if err := decodeJSON(raw, &cfg.HardDiskSizeMB); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyHardDiskImage:
			if err := decodeJSON(raw, &cfg.HardDiskImagePath); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyScale:
			if err := decodeJSON(raw, &cfg.Scale); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyFullscreen:
			if err := decodeJSON(raw, &cfg.Fullscreen); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyHeadless:
			if err := decodeJSON(raw, &cfg.Headless); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyFrames:
			if err := decodeJSON(raw, &cfg.Frames); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyDumpFrame:
			if err := decodeJSON(raw, &cfg.DumpFramePath); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyTrace:
			if err := decodeJSON(raw, &cfg.Trace); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyTraceStart:
			value, err := decodeAddressJSON(raw)
			if err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.TraceStart = value
		case KeyTraceEnd:
			value, err := decodeAddressJSON(raw)
			if err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.TraceEnd = value
		case KeyRAMSize:
			if err := decodeJSON(raw, &cfg.RAMSize); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyClockHz:
			if err := decodeJSON(raw, &cfg.ClockHz); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyCPUMHz:
			value, err := decodeMHzJSON(raw)
			if err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.CPUClockHz = value
		case KeyFrameHz:
			if err := decodeJSON(raw, &cfg.FrameHz); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyColorMonitor:
			if err := decodeJSON(raw, &cfg.ColorMonitor); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyMidResYScale:
			if err := decodeJSON(raw, &cfg.MidResYScale); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case KeyModel:
			if err := decodeJSON(raw, &cfg.Model); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		default:
			return fmt.Errorf("unsupported config key %q", key)
		}
	}
	return nil
}

func (p configPatch) Preset() (Preset, bool, error) {
	raw, ok := p[KeyPreset]
	if !ok {
		return "", false, nil
	}

	var preset Preset
	if err := decodeJSON(raw, &preset); err != nil {
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
	model := string(cfg.Model)

	fs.String(KeyConfig, "", "optional JSON config file loaded before CLI overrides")
	fs.StringVar(&preset, KeyPreset, preset, "machine preset: default|stf|st|mega-st")
	fs.StringVar(&cfg.ROMPath, KeyROM, cfg.ROMPath, "path to Atari ST TOS ROM")
	fs.StringVar(&cfg.FloppyA, KeyFloppyA, cfg.FloppyA, "path to drive A disk image (.st or .msa)")
	fs.Var(uint32Flag{target: &cfg.HardDiskSizeMB}, KeyHardDiskSizeMB, "virtual ACSI hard disk size in MiB (0 disables)")
	fs.StringVar(&cfg.HardDiskImagePath, KeyHardDiskImage, cfg.HardDiskImagePath, "path to persistent virtual hard disk image file")
	fs.Float64Var(&cfg.Scale, KeyScale, cfg.Scale, "display scale factor")
	fs.BoolVar(&cfg.Fullscreen, KeyFullscreen, cfg.Fullscreen, "run in fullscreen mode")
	fs.BoolVar(&cfg.Headless, KeyHeadless, cfg.Headless, "disable video output and window creation")
	fs.IntVar(&cfg.Frames, KeyFrames, cfg.Frames, "frames to run in headless mode")
	fs.StringVar(&cfg.DumpFramePath, KeyDumpFrame, cfg.DumpFramePath, "write the last rendered framebuffer to a PNG file")
	fs.StringVar(&cfg.Trace, KeyTrace, cfg.Trace, "enable tracing: cpu|cpu-verbose|boot|boot-verbose|shifter|shifter-verbose")
	fs.Var(addressFlag{target: &cfg.TraceStart}, KeyTraceStart, "first PC included in boot traces")
	fs.Var(addressFlag{target: &cfg.TraceEnd}, KeyTraceEnd, "last PC included in boot traces")
	fs.Var(uint32Flag{target: &cfg.RAMSize}, KeyRAMSize, "amount of emulated RAM in bytes")
	fs.Uint64Var(&cfg.ClockHz, KeyClockHz, cfg.ClockHz, "base machine clock frequency in Hz")
	fs.Var(mhzFlag{target: &cfg.CPUClockHz}, KeyCPUMHz, "CPU frequency in MHz (hardware timing remains unchanged)")
	fs.Uint64Var(&cfg.FrameHz, KeyFrameHz, cfg.FrameHz, "frames per second for display and VBL timing")
	fs.BoolVar(&cfg.ColorMonitor, KeyColorMonitor, cfg.ColorMonitor, "emulate an Atari color monitor instead of monochrome")
	fs.IntVar(&cfg.MidResYScale, KeyMidResYScale, cfg.MidResYScale, "vertical host scaling for medium resolution (>=1)")
	fs.StringVar(&model, KeyModel, model, "machine model: st|ste")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg.Preset = Preset(preset)
	cfg.Model = MachineModel(model)
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

func decodeJSON(raw json.RawMessage, dst any) error {
	return json.Unmarshal(raw, dst)
}

func decodeAddressJSON(raw json.RawMessage) (uint32, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("empty address")
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		return parseAddress(text)
	}

	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	if value > 0xFFFFFFFF {
		return 0, fmt.Errorf("address %d overflows uint32", value)
	}
	return uint32(value), nil
}

func decodeMHzJSON(raw json.RawMessage) (uint64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("empty MHz value")
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		return parseMHz(text)
	}

	var mhz float64
	if err := json.Unmarshal(raw, &mhz); err != nil {
		return 0, err
	}
	return mhzToHz(mhz)
}

func lookupFlagValue(args []string, name string) (string, bool) {
	longName := "--" + name
	shortName := "-" + name

	for i := 0; i < len(args); i++ {
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

func parseAddress(raw string) (uint32, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 0, 32)
	if err != nil {
		return 0, err
	}
	return uint32(value), nil
}

func formatAddress(value uint32) string {
	return fmt.Sprintf("0x%06x", value)
}

func parseMHz(raw string) (uint64, error) {
	mhz, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}
	return mhzToHz(mhz)
}

func mhzToHz(mhz float64) (uint64, error) {
	if mhz <= 0 {
		return 0, fmt.Errorf("invalid %s %.3f: must be > 0", KeyCPUMHz, mhz)
	}

	hz := uint64(mhz * 1_000_000.0)
	if hz == 0 {
		return 0, fmt.Errorf("invalid %s %.6f: effective CPU clock rounded to 0 Hz", KeyCPUMHz, mhz)
	}
	return hz, nil
}

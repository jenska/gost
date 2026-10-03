package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// configPatch is a JSON config file: flag keys mapped to their raw values.
type configPatch map[string]json.RawMessage

func NewConfig() (*Config, error) {
	return Load(os.Args[1:])
}

// Load builds a validated Config from process arguments: preset defaults, then
// the JSON file named by --config, then the remaining flags.
func Load(args []string) (*Config, error) {
	// The first pass only learns which config file and preset the flags name.
	// It runs against a throwaway config, so flag errors and -help surface here,
	// once, before any file is read.
	var configPath, preset string
	if err := parseFlags(DefaultConfig(), args, &configPath, &preset, os.Stderr); err != nil {
		return nil, err
	}
	patch, err := loadConfigPatch(configPath)
	if err != nil {
		return nil, err
	}
	cfg, err := patch.build(preset)
	if err != nil {
		return nil, err
	}
	if err := parseFlags(cfg, args, nil, nil, io.Discard); err != nil {
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
	cfg, err := patch.build("")
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
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

// build returns the defaults of preset, or of the preset the patch names when
// preset is empty, with the patch's remaining keys applied on top.
func (p configPatch) build(preset string) (*Config, error) {
	if raw, ok := p[KeyPreset]; ok && preset == "" {
		if err := json.Unmarshal(raw, &preset); err != nil {
			return nil, fmt.Errorf("decode %q: %w", KeyPreset, err)
		}
	}
	cfg, err := configForPreset(preset)
	if err != nil {
		return nil, err
	}
	if err := p.Apply(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseFlags applies args to cfg. The --config and --preset values are stored
// in configPath and preset when those are non-nil.
func parseFlags(cfg *Config, args []string, configPath, preset *string, output io.Writer) error {
	fs := flag.NewFlagSet("gost", flag.ContinueOnError)
	fs.SetOutput(output)

	if configPath == nil {
		configPath = new(string)
	}
	if preset == nil {
		preset = new(string)
	}
	fs.StringVar(configPath, KeyConfig, "", "optional JSON config file loaded before CLI overrides")
	fs.StringVar(preset, KeyPreset, "", presetUsage())
	for _, f := range fields {
		fs.Var(f.value(cfg), f.key, f.usage)
	}
	return fs.Parse(args)
}

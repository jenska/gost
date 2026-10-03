package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigDirEnv overrides the base directory used for saved profiles and the
// last-used config. It exists mainly so tests can redirect config storage into a
// temporary directory.
const ConfigDirEnv = "GOST_CONFIG_DIR"

const lastConfigFile = "last.json"

// configHome resolves the base directory for GoST's local configuration.
func configHome() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(ConfigDirEnv)); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(base, "gost"), nil
}

// ProfileDir is the directory that holds saved configuration profiles.
func ProfileDir() (string, error) {
	home, err := configHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "profiles"), nil
}

// ToPatch renders the user-facing subset of the configuration as a flag-keyed
// map suitable for JSON serialisation and reloading through LoadConfigFile.
func (cfg *Config) ToPatch() map[string]any {
	patch := map[string]any{}
	for _, f := range fields {
		if !f.persist {
			continue
		}
		value := f.value(cfg).Get()
		if s, ok := value.(string); ok && s == "" {
			continue // unset image paths stay out of the file
		}
		patch[f.key] = value
	}
	return patch
}

func marshalPatch(cfg *Config) ([]byte, error) {
	data, err := json.MarshalIndent(cfg.ToPatch(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// SaveProfile writes cfg to <ProfileDir>/<name>.json.
func SaveProfile(name string, cfg *Config) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	dir, err := ProfileDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := marshalPatch(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644)
}

// LoadProfile reads the profile saved under name.
func LoadProfile(name string) (*Config, error) {
	if err := validateProfileName(name); err != nil {
		return nil, err
	}
	dir, err := ProfileDir()
	if err != nil {
		return nil, err
	}
	return LoadConfigFile(filepath.Join(dir, name+".json"))
}

// DeleteProfile removes the profile saved under name.
func DeleteProfile(name string) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	dir, err := ProfileDir()
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, name+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ListProfiles returns the sorted names of all saved profiles.
func ListProfiles() ([]string, error) {
	dir, err := ProfileDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			names = append(names, strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
		}
	}
	sort.Strings(names)
	return names, nil
}

// SaveLastConfig records cfg as the configuration to pre-select next time the
// launcher opens.
func SaveLastConfig(cfg *Config) error {
	home, err := configHome()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	data, err := marshalPatch(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, lastConfigFile), data, 0o644)
}

// LoadLastConfig returns the last configuration recorded by SaveLastConfig. The
// boolean is false when no usable record exists.
func LoadLastConfig() (*Config, bool) {
	home, err := configHome()
	if err != nil {
		return nil, false
	}
	cfg, err := LoadConfigFile(filepath.Join(home, lastConfigFile))
	if err != nil {
		return nil, false
	}
	return cfg, true
}

func validateProfileName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("profile name is required")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid profile name %q", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid profile name %q", name)
	}
	return nil
}

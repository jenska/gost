package emulator

import coreconfig "github.com/jenska/gost/internal/config"

type Config = coreconfig.Config

func DefaultConfig() *Config {
	return coreconfig.DefaultConfig()
}

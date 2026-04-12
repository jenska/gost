package config

import "os"

func LoadROM(path string) ([]byte, error) {
	return os.ReadFile(path)
}

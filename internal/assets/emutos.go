package assets

import _ "embed"

const (
	DefaultOSName    = "EmuTOS 1.4 (US, 256K)"
	DefaultOSVersion = "1.4"
)

//go:embed emutos-1.4-256k-us.img
var defaultROM []byte

func DefaultROM() []byte {
	return append([]byte(nil), defaultROM...)
}

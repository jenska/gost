package assets

import _ "embed"

//go:embed gost-icon.png
var appIconPNG []byte

// AppIconPNG returns the GoST logo used as the application icon.
func AppIconPNG() []byte {
	return appIconPNG
}

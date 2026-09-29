package ebiten

import (
	"bytes"
	"image"
	_ "image/png"

	ebitenlib "github.com/hajimehoshi/ebiten/v2"

	"github.com/jenska/gost/internal/assets"
)

// applyAppIcon shows the GoST logo in the window title bar and taskbar, and
// in the Dock on macOS, where Ebiten's window icon has no effect.
func applyAppIcon() {
	png := assets.AppIconPNG()
	if icon, _, err := image.Decode(bytes.NewReader(png)); err == nil {
		ebitenlib.SetWindowIcon([]image.Image{icon})
	}
	setDockIcon(png)
}

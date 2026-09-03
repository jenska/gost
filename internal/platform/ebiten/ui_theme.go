package ebiten

import (
	"bytes"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	panelFaceSource *text.GoTextFaceSource

	panelTextColor          = color.NRGBA{R: 0xF2, G: 0xF5, B: 0xF7, A: 0xFF}
	panelTitleColor         = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	panelMutedTextColor     = color.NRGBA{R: 0xB9, G: 0xC1, B: 0xC9, A: 0xFF}
	panelErrorTextColor     = color.NRGBA{R: 0xFF, G: 0xB0, B: 0x86, A: 0xFF}
	panelAccentColor        = color.NRGBA{R: 0x7D, G: 0xD3, B: 0xFC, A: 0xFF}
	panelBackgroundColor    = color.NRGBA{R: 0x15, G: 0x19, B: 0x1D, A: 0xEE}
	panelRowBackgroundColor = color.NRGBA{R: 0x22, G: 0x28, B: 0x2E, A: 0xF6}
	inputBackgroundColor    = color.NRGBA{R: 0x0E, G: 0x11, B: 0x14, A: 0xFF}
	buttonIdleColor         = color.NRGBA{R: 0x35, G: 0x45, B: 0x52, A: 0xFF}
	buttonHoverColor        = color.NRGBA{R: 0x46, G: 0x5C, B: 0x6D, A: 0xFF}
	buttonPressedColor      = color.NRGBA{R: 0x22, G: 0x73, B: 0x91, A: 0xFF}
)

func panelFontFace(size float64) *text.Face {
	if panelFaceSource == nil {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		if err != nil {
			panic(err)
		}
		panelFaceSource = source
	}
	face := text.Face(&text.GoTextFace{
		Source: panelFaceSource,
		Size:   size,
	})
	return &face
}

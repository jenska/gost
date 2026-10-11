package ebiten

import (
	"bytes"
	"image"
	"image/color"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	panelFaceSource *text.GoTextFaceSource

	panelTextColor      = color.NRGBA{R: 0xF2, G: 0xF5, B: 0xF7, A: 0xFF}
	panelTitleColor     = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	panelMutedTextColor = color.NRGBA{R: 0xB9, G: 0xC1, B: 0xC9, A: 0xFF}
	panelAccentColor    = color.NRGBA{R: 0x7D, G: 0xD3, B: 0xFC, A: 0xFF}
	// The panel and its inner areas are translucent so the panel floats on
	// the emulated screen; stacked, the inner areas are about 90% opaque.
	panelBackgroundColor    = color.NRGBA{R: 0x15, G: 0x19, B: 0x1D, A: 0xD0}
	panelRowBackgroundColor = color.NRGBA{R: 0x22, G: 0x28, B: 0x2E, A: 0x80}
	panelBorderColor        = color.NRGBA{R: 0xD8, G: 0xE2, B: 0xEA, A: 0xB0}
	panelShadowColor        = color.NRGBA{A: 0x60}
	inputBackgroundColor    = color.NRGBA{R: 0x0E, G: 0x11, B: 0x14, A: 0xFF}
	buttonIdleColor         = color.NRGBA{R: 0x35, G: 0x45, B: 0x52, A: 0xFF}
	buttonHoverColor        = color.NRGBA{R: 0x46, G: 0x5C, B: 0x6D, A: 0xFF}
	buttonPressedColor      = color.NRGBA{R: 0x22, G: 0x73, B: 0x91, A: 0xFF}
	choiceSelectedColor     = color.NRGBA{R: 0x1E, G: 0x6A, B: 0x86, A: 0xFF}
	choiceSelectedHover     = color.NRGBA{R: 0x28, G: 0x7E, B: 0x9E, A: 0xFF}
	separatorColor          = color.NRGBA{R: 0x3A, G: 0x45, B: 0x4F, A: 0xFF}
)

// listEntryColor styles list entries: the topic list and the preset drop-down.
var listEntryColor = &widget.ListEntryColor{
	Unselected:                 panelMutedTextColor,
	Selected:                   panelTitleColor,
	DisabledUnselected:         panelMutedTextColor,
	DisabledSelected:           panelMutedTextColor,
	SelectingBackground:        buttonHoverColor,
	SelectedBackground:         choiceSelectedColor,
	FocusedBackground:          buttonIdleColor,
	SelectingFocusedBackground: buttonPressedColor,
	SelectedFocusedBackground:  choiceSelectedHover,
	DisabledSelectedBackground: buttonIdleColor,
}

// choiceIdleImage and choiceSelectedImage style the segmented option buttons
// of the configuration pages: the selected option is drawn highlighted.
var (
	choiceIdleImage = &widget.ButtonImage{
		Idle:    euiimage.NewNineSliceColor(buttonIdleColor),
		Hover:   euiimage.NewNineSliceColor(buttonHoverColor),
		Pressed: euiimage.NewNineSliceColor(buttonPressedColor),
	}
	choiceSelectedImage = &widget.ButtonImage{
		Idle:    euiimage.NewNineSliceColor(choiceSelectedColor),
		Hover:   euiimage.NewNineSliceColor(choiceSelectedHover),
		Pressed: euiimage.NewNineSliceColor(buttonPressedColor),
	}
)

// panelShadowSize is how far the panel's shadow reaches beyond its border.
const panelShadowSize = 6

// newPanelFrameImage is the panel background: a translucent fill inside a
// one-pixel light border.
func newPanelFrameImage() *euiimage.NineSlice {
	img := ebitenlib.NewImage(3, 3)
	img.Fill(panelBorderColor)
	img.Set(1, 1, panelBackgroundColor)
	return euiimage.NewNineSliceSimple(img, 1, 1)
}

// drawPanelShadow draws a soft shadow around rect, fading out over
// panelShadowSize pixels. It only paints outside rect, so the translucent
// panel drawn on top does not darken what shows through it.
func drawPanelShadow(screen *ebitenlib.Image, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	for i := 1; i <= panelShadowSize; i++ {
		fade := float64(panelShadowSize-i+1) / panelShadowSize
		c := color.NRGBA{A: uint8(float64(panelShadowColor.A) * fade * fade)}
		d := float32(i) - 0.5
		vector.StrokeRect(screen,
			float32(rect.Min.X)-d, float32(rect.Min.Y)-d,
			float32(rect.Dx())+2*d, float32(rect.Dy())+2*d,
			1, c, false)
	}
}

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

func newPanelText(label string, size float64, c color.Color) *widget.Text {
	face := panelFontFace(size)
	return widget.NewText(
		widget.TextOpts.Text(label, face, c),
		widget.TextOpts.Padding(&widget.Insets{Top: 1, Bottom: 1}),
	)
}

func newPanelTextInput(placeholder string, width int) *widget.TextInput {
	if width <= 0 {
		width = 260
	}
	return widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(widget.WidgetOpts.MinSize(width, 24)),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle: euiimage.NewNineSliceColor(inputBackgroundColor),
		}),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:  panelTextColor,
			Caret: panelAccentColor,
		}),
		widget.TextInputOpts.Face(panelFontFace(12)),
		widget.TextInputOpts.Padding(&widget.Insets{Top: 4, Bottom: 4, Left: 6, Right: 6}),
		widget.TextInputOpts.Placeholder(placeholder),
	)
}

func newPanelButton(label string, onClick func()) *widget.Button {
	return newPanelButtonW(label, onClick)
}

func newPanelButtonW(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    euiimage.NewNineSliceColor(buttonIdleColor),
			Hover:   euiimage.NewNineSliceColor(buttonHoverColor),
			Pressed: euiimage.NewNineSliceColor(buttonPressedColor),
		}),
		widget.ButtonOpts.Text(label, panelFontFace(12), &widget.ButtonTextColor{
			Idle:     panelTextColor,
			Hover:    panelTextColor,
			Pressed:  panelTextColor,
			Disabled: panelMutedTextColor,
		}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 4, Bottom: 4, Left: 8, Right: 8}),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
			onClick()
		}),
	)
}

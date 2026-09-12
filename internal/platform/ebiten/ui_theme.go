package ebiten

import (
	"bytes"
	"image/color"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	panelFaceSource *text.GoTextFaceSource

	panelTextColor          = color.NRGBA{R: 0xF2, G: 0xF5, B: 0xF7, A: 0xFF}
	panelTitleColor         = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	panelMutedTextColor     = color.NRGBA{R: 0xB9, G: 0xC1, B: 0xC9, A: 0xFF}
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
	return newPanelButtonW(label, 0, onClick)
}

func newPanelButtonW(label string, width int, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(width, 24)),
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

func newPanelCheckbox(label string, checked bool, onChange func(bool)) *widget.Checkbox {
	box := func(c color.Color) *euiimage.NineSlice { return euiimage.NewNineSliceColor(c) }
	initial := widget.WidgetUnchecked
	if checked {
		initial = widget.WidgetChecked
	}
	return widget.NewCheckbox(
		widget.CheckboxOpts.WidgetOpts(widget.WidgetOpts.MinSize(16, 16)),
		widget.CheckboxOpts.Image(&widget.CheckboxImage{
			Unchecked:         box(inputBackgroundColor),
			UncheckedHovered:  box(buttonHoverColor),
			UncheckedDisabled: box(inputBackgroundColor),
			Checked:           box(panelAccentColor),
			CheckedHovered:    box(panelAccentColor),
			CheckedDisabled:   box(buttonIdleColor),
			Greyed:            box(buttonIdleColor),
			GreyedHovered:     box(buttonIdleColor),
			GreyedDisabled:    box(buttonIdleColor),
		}),
		widget.CheckboxOpts.Text(label, panelFontFace(12), &widget.LabelColor{
			Idle:     panelTextColor,
			Disabled: panelMutedTextColor,
		}),
		widget.CheckboxOpts.Spacing(8),
		widget.CheckboxOpts.InitialState(initial),
		widget.CheckboxOpts.StateChangedHandler(func(args *widget.CheckboxChangedEventArgs) {
			onChange(args.State == widget.WidgetChecked)
		}),
	)
}

func newPanelSlider(minValue, maxValue, current int, onChange func(int)) *widget.Slider {
	return widget.NewSlider(
		widget.SliderOpts.WidgetOpts(widget.WidgetOpts.MinSize(180, 18)),
		widget.SliderOpts.MinMax(minValue, maxValue),
		widget.SliderOpts.InitialCurrent(current),
		widget.SliderOpts.Images(
			&widget.SliderTrackImage{
				Idle:  euiimage.NewNineSliceColor(inputBackgroundColor),
				Hover: euiimage.NewNineSliceColor(inputBackgroundColor),
			},
			&widget.ButtonImage{
				Idle:    euiimage.NewNineSliceColor(panelAccentColor),
				Hover:   euiimage.NewNineSliceColor(panelAccentColor),
				Pressed: euiimage.NewNineSliceColor(buttonPressedColor),
			},
		),
		widget.SliderOpts.FixedHandleSize(14),
		widget.SliderOpts.TrackPadding(&widget.Insets{Top: 5, Bottom: 5}),
		widget.SliderOpts.ChangedHandler(func(args *widget.SliderChangedEventArgs) {
			onChange(args.Current)
		}),
	)
}

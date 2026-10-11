package ebiten

import (
	"image"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/jenska/gost/internal/config"
)

const (
	presetComboWidth = 220
	// presetNoteWidth reserves room for the longest preset note so the
	// header does not have to grow when the selection changes.
	presetNoteWidth = 170
)

// customPreset is shown in the preset combo box while the working config
// matches none of the presets. It is not offered in the drop-down list.
var customPreset = &config.MachinePreset{Label: "Custom", Note: "settings match no preset"}

// newHeader is the panel's top row: the title on the left and the machine
// preset combo box on the right.
func (p *configPanel) newHeader(title string) *widget.Container {
	header := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(panelContentWidth-2*panelPadding, 0)),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(3),
			widget.GridLayoutOpts.Stretch([]bool{true, false, false}, nil),
			widget.GridLayoutOpts.Spacing(8, 0),
		)),
	)
	header.AddChild(newPanelText(title, 15, panelTitleColor))

	p.presetNote = newPanelText("", 11, panelMutedTextColor)
	p.presetNote.GetWidget().LayoutData = widget.GridLayoutData{VerticalPosition: widget.GridLayoutPositionCenter}
	p.presetNote.GetWidget().MinWidth = presetNoteWidth
	header.AddChild(p.presetNote)

	p.presetCombo = p.newPresetCombo()
	header.AddChild(p.presetCombo)
	p.header = header
	p.refreshers = append(p.refreshers, p.syncPreset)
	return header
}

func (p *configPanel) newPresetCombo() *widget.ListComboButton {
	entries := make([]any, len(config.MachinePresets))
	for i := range config.MachinePresets {
		entries[i] = &config.MachinePresets[i]
	}
	label := func(e any) string { return e.(*config.MachinePreset).Label }
	trackImage := &widget.SliderTrackImage{
		Idle:  euiimage.NewNineSliceColor(inputBackgroundColor),
		Hover: euiimage.NewNineSliceColor(inputBackgroundColor),
	}
	handleImage := &widget.ButtonImage{
		Idle:    euiimage.NewNineSliceColor(buttonIdleColor),
		Hover:   euiimage.NewNineSliceColor(buttonHoverColor),
		Pressed: euiimage.NewNineSliceColor(buttonPressedColor),
	}
	handleSize := 6

	return widget.NewListComboButton(
		widget.ListComboButtonOpts.Entries(entries),
		widget.ListComboButtonOpts.InitialEntry(p.matchingPreset()),
		widget.ListComboButtonOpts.EntryLabelFunc(func(e any) string {
			return "Preset:  " + label(e) + "  ▼"
		}, label),
		widget.ListComboButtonOpts.ButtonParams(&widget.ButtonParams{
			Image:       choiceIdleImage,
			TextPadding: &widget.Insets{Top: 4, Bottom: 4, Left: 10, Right: 10},
			MinSize:     &image.Point{X: presetComboWidth, Y: 0},
		}),
		widget.ListComboButtonOpts.Text(panelFontFace(12), nil, &widget.ButtonTextColor{
			Idle:     panelTextColor,
			Hover:    panelTextColor,
			Pressed:  panelTextColor,
			Disabled: panelMutedTextColor,
		}),
		widget.ListComboButtonOpts.ListParams(&widget.ListParams{
			EntryFace:        panelFontFace(12),
			EntryColor:       listEntryColor,
			EntryTextPadding: &widget.Insets{Top: 5, Bottom: 5, Left: 10, Right: 10},
			MinSize:          &image.Point{X: presetComboWidth, Y: 0},
			ScrollContainerImage: &widget.ScrollContainerImage{
				Idle: euiimage.NewNineSliceColor(inputBackgroundColor),
				Mask: euiimage.NewNineSliceColor(inputBackgroundColor),
			},
			Slider: &widget.SliderParams{
				TrackImage:    trackImage,
				HandleImage:   handleImage,
				MinHandleSize: &handleSize,
			},
		}),
		widget.ListComboButtonOpts.MaxContentHeight(260),
		widget.ListComboButtonOpts.EntrySelectedHandler(func(args *widget.ListComboButtonEntrySelectedEventArgs) {
			if preset, ok := args.Entry.(*config.MachinePreset); ok {
				p.selectPreset(preset)
			}
		}),
	)
}

// selectPreset applies a preset the user picked from the combo box. The combo
// also reports entries it is set to programmatically (syncPreset); those
// already match the working config and must not re-apply, which would reset
// fields such as the monitor that presets do not identify by.
func (p *configPanel) selectPreset(preset *config.MachinePreset) {
	if preset == customPreset || preset == p.matchingPreset() {
		return
	}
	preset.Apply(&p.work)
	p.changed()
	p.Refresh()
}

// syncPreset points the combo box and its note at the preset the working
// config matches, or at "Custom".
func (p *configPanel) syncPreset() {
	preset := p.matchingPreset()
	if p.presetCombo.SelectedEntry() != preset {
		p.presetCombo.SetSelectedEntry(preset)
	}
	if p.presetNote.Label != preset.Note {
		p.presetNote.Label = preset.Note
		p.header.RequestRelayout()
	}
}

// matchingPreset is the preset the working config matches, or customPreset.
func (p *configPanel) matchingPreset() *config.MachinePreset {
	id := config.MatchPreset(&p.work)
	for i := range config.MachinePresets {
		if config.MachinePresets[i].ID == id {
			return &config.MachinePresets[i]
		}
	}
	return customPreset
}

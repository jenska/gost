package ebiten

import (
	"fmt"
	"image/color"
	"path/filepath"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

type floppyPanel struct {
	app          *App
	container    *widget.Container
	status       *widget.Text
	mounted      [2]*widget.Text
	pathInputs   [2]*widget.TextInput
	lastPathText [2]string
}

func newFloppyPanel(app *App) *floppyPanel {
	p := &floppyPanel{app: app}
	p.container = widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				StretchHorizontal:  false,
				StretchVertical:    false,
			}),
			widget.WidgetOpts.MinSize(580, 0),
		),
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(panelBackgroundColor)),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 16, Bottom: 16, Left: 18, Right: 18}),
			widget.RowLayoutOpts.Spacing(12),
		)),
	)
	p.container.AddChild(newPanelText("Floppy Disks", 18, panelTitleColor))
	p.container.AddChild(newPanelText("Enter or browse to a disk image path, then mount it into drive A or B.", 13, panelMutedTextColor))
	for drive := range len(p.pathInputs) {
		p.container.AddChild(p.newDriveRow(drive))
	}
	p.status = newPanelText("", 13, panelErrorTextColor)
	p.container.AddChild(p.status)
	p.Refresh()
	return p
}

func (p *floppyPanel) newDriveRow(drive int) *widget.Container {
	row := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(6),
		)),
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(panelRowBackgroundColor)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(540, 0)),
	)
	row.AddChild(newPanelText(fmt.Sprintf("Drive %c", 'A'+drive), 14, panelTitleColor))
	p.mounted[drive] = newPanelText("", 12, panelMutedTextColor)
	row.AddChild(p.mounted[drive])

	controls := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	input := newPanelTextInput(fmt.Sprintf("Path to drive %c image", 'A'+drive))
	p.pathInputs[drive] = input
	controls.AddChild(input)
	controls.AddChild(newPanelButton("Browse", func() {
		p.QueueBrowse(drive)
	}))
	controls.AddChild(newPanelButton("Mount", func() {
		p.QueueMount(drive)
	}))
	controls.AddChild(newPanelButton("Eject", func() {
		p.QueueEject(drive)
	}))
	row.AddChild(controls)
	return row
}

func (p *floppyPanel) QueueMount(drive int) {
	if drive < 0 || drive >= len(p.pathInputs) || p.pathInputs[drive] == nil {
		return
	}
	p.app.QueueMountFloppy(drive, p.pathInputs[drive].GetText())
}

func (p *floppyPanel) QueueBrowse(drive int) {
	if drive < 0 || drive >= len(p.pathInputs) || p.pathInputs[drive] == nil {
		return
	}
	p.app.QueueBrowseFloppy(drive)
	p.Refresh()
}

func (p *floppyPanel) QueueEject(drive int) {
	if drive < 0 || drive >= len(p.pathInputs) || p.pathInputs[drive] == nil {
		return
	}
	p.pathInputs[drive].SetText("")
	p.lastPathText[drive] = ""
	p.app.ClearSelectedFloppyPath(drive)
	p.app.QueueEjectFloppy(drive)
}

func (p *floppyPanel) Refresh() {
	for drive, input := range p.pathInputs {
		if input == nil || p.mounted[drive] == nil {
			continue
		}
		mountedPath := p.app.MountedFloppyPath(drive)
		if mountedPath != "" {
			p.mounted[drive].Label = "Mounted: " + filepath.Base(mountedPath)
		} else {
			p.mounted[drive].Label = "No disk mounted"
		}

		// A freshly browsed path takes precedence over the mounted one; once
		// the field shows a path the user is free to edit it, so only push a
		// value that changed since we last set it.
		desiredPath := p.app.SelectedFloppyPath(drive)
		if desiredPath == "" {
			desiredPath = mountedPath
		}
		if desiredPath != "" && desiredPath != p.lastPathText[drive] {
			input.SetText(desiredPath)
			p.lastPathText[drive] = desiredPath
		}
	}
	if p.status == nil {
		return
	}
	if errText := p.app.LastHostCommandError(); errText != "" {
		p.status.Label = errText
	} else {
		p.status.Label = "F12 closes this panel."
	}
}

func newPanelText(label string, size float64, c color.Color) *widget.Text {
	face := panelFontFace(size)
	return widget.NewText(
		widget.TextOpts.Text(label, face, c),
		widget.TextOpts.Padding(&widget.Insets{Top: 2, Bottom: 2}),
	)
}

func newPanelTextInput(placeholder string) *widget.TextInput {
	return widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(widget.WidgetOpts.MinSize(330, 30)),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle: euiimage.NewNineSliceColor(inputBackgroundColor),
		}),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:  panelTextColor,
			Caret: panelAccentColor,
		}),
		widget.TextInputOpts.Face(panelFontFace(13)),
		widget.TextInputOpts.Padding(&widget.Insets{Top: 6, Bottom: 6, Left: 8, Right: 8}),
		widget.TextInputOpts.Placeholder(placeholder),
	)
}

func newPanelButton(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(74, 30)),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    euiimage.NewNineSliceColor(buttonIdleColor),
			Hover:   euiimage.NewNineSliceColor(buttonHoverColor),
			Pressed: euiimage.NewNineSliceColor(buttonPressedColor),
		}),
		widget.ButtonOpts.Text(label, panelFontFace(13), &widget.ButtonTextColor{
			Idle:     panelTextColor,
			Hover:    panelTextColor,
			Pressed:  panelTextColor,
			Disabled: panelMutedTextColor,
		}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 6, Bottom: 6, Left: 10, Right: 10}),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
			onClick()
		}),
	)
}

package ebiten

import (
	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

// Building blocks shared by the config_page_*.go topic pages.

// newPage starts a topic page; settings are added to the returned page's
// content container.
func newPage(title, description string) *configPage {
	return &configPage{
		title:       title,
		description: description,
		content: widget.NewContainer(
			widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				StretchHorizontal: true,
			})),
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionVertical),
				widget.RowLayoutOpts.Spacing(14),
			)),
		),
	}
}

// addSetting appends a labelled setting to the page: a heading, the control
// and an optional muted hint below it. The hint text is returned so callers
// can update it as the setting changes.
func (pg *configPage) addSetting(label, hint string, control widget.PreferredSizeLocateableWidget) *widget.Text {
	setting := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(4),
		)),
	)
	setting.AddChild(newPanelText(label, 12, panelAccentColor))
	setting.AddChild(control)
	hintText := newPanelText(hint, 11, panelMutedTextColor)
	setting.AddChild(hintText)
	pg.content.AddChild(setting)
	return hintText
}

func newSeparator(width int) *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(width, 1)),
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(separatorColor)),
	)
}

func newHorizontalRow() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)
}

// newChoice builds a segmented control: one button per label, with the
// option selected() reports drawn highlighted (-1 highlights none). Clicking a
// button calls onSelect with its index. columns > 0 wraps the buttons into a
// grid of that many equally wide columns instead of a single row.
func (p *configPanel) newChoice(labels []string, columns int, selected func() int, onSelect func(int)) *widget.Container {
	var group *widget.Container
	if columns > 0 {
		group = widget.NewContainer(
			widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(pageInnerWidth, 0)),
			widget.ContainerOpts.Layout(widget.NewGridLayout(
				widget.GridLayoutOpts.Columns(columns),
				widget.GridLayoutOpts.Spacing(4, 4),
				widget.GridLayoutOpts.DefaultStretch(true, false),
			)),
		)
	} else {
		group = widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Spacing(4),
			)),
		)
	}

	buttons := make([]*widget.Button, len(labels))
	for i, label := range labels {
		buttons[i] = newPanelButtonW(label, func() {
			onSelect(i)
			p.changed()
			p.Refresh()
		})
		group.AddChild(buttons[i])
	}

	refresh := func() {
		current := selected()
		for i, button := range buttons {
			if i == current {
				button.SetImage(choiceSelectedImage)
			} else {
				button.SetImage(choiceIdleImage)
			}
		}
	}
	refresh()
	p.refreshers = append(p.refreshers, refresh)
	return group
}

// newOnOffChoice is a two-option choice bound to a bool.
func (p *configPanel) newOnOffChoice(off, on string, value *bool) *widget.Container {
	return p.newChoice([]string{off, on}, 0, func() int {
		if *value {
			return 1
		}
		return 0
	}, func(i int) {
		*value = i == 1
	})
}

// newPathInput is a file path text field with Browse and clear buttons. The
// field is tracked under target so Refresh can fill in natively browsed paths
// and apply can read it back. extra buttons are placed between Browse and the
// clear button.
func (p *configPanel) newPathInput(placeholder string, target fileTarget, initial string, inputWidth int, clearLabel string, extra ...*widget.Button) *widget.Container {
	row := newHorizontalRow()
	input := newPanelTextInput(placeholder, inputWidth)
	input.SetText(initial)
	p.pathInputs[target] = input
	p.lastPathText[target] = initial
	row.AddChild(input)
	row.AddChild(newPanelButton("Browse…", func() { p.app.QueueBrowse(target) }))
	for _, button := range extra {
		row.AddChild(button)
	}
	row.AddChild(newPanelButton(clearLabel, func() {
		input.SetText("")
		p.lastPathText[target] = ""
		p.app.ClearSelectedPath(target)
		if target == targetHardDiskOpen {
			p.app.ClearSelectedPath(targetHardDiskNew)
		}
	}))
	return row
}

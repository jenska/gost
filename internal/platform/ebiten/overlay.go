package ebiten

import (
	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/widget"
	ebitenlib "github.com/hajimehoshi/ebiten/v2"
)

type overlay struct {
	ui      *ebitenui.UI
	panel   *floppyPanel
	visible bool
}

func newOverlay(app *App) *overlay {
	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	o := &overlay{
		ui:    &ebitenui.UI{Container: root},
		panel: newFloppyPanel(app),
	}
	root.AddChild(o.panel.container)
	return o
}

func (o *overlay) Visible() bool {
	return o != nil && o.visible
}

func (o *overlay) SetVisible(visible bool) {
	o.visible = visible
}

// Update and Draw are only called while the overlay is visible (see App).

func (o *overlay) Update() {
	o.panel.Refresh()
	o.ui.Update()
}

func (o *overlay) Draw(screen *ebitenlib.Image) {
	o.ui.Draw(screen)
}

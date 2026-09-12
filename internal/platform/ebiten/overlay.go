package ebiten

import (
	"image/color"

	"github.com/ebitenui/ebitenui"
	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	ebitenlib "github.com/hajimehoshi/ebiten/v2"
)

// scrollWheelStep is how many pixels one mouse-wheel notch scrolls the overlay.
const scrollWheelStep = 48

// overlay is the F12 in-session configuration HUD. It hosts the same
// configPanel as the launcher, drawn on top of the emulated ST screen without
// ever resizing or rescaling it (see App.Layout): the panel keeps its natural
// size and scrolls within whatever screen space the current ST resolution
// leaves, rather than growing the display to fit.
type overlay struct {
	ui      *ebitenui.UI
	panel   *configPanel
	scroll  *widget.ScrollContainer
	visible bool
}

func newOverlay(app *App) *overlay {
	panel := newConfigPanel(app, configModeOverlay, app.cfg)

	scroll := widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(panel.container),
		widget.ScrollContainerOpts.Padding(&widget.Insets{Top: 10, Bottom: 10, Left: 10, Right: 10}),
		widget.ScrollContainerOpts.Image(&widget.ScrollContainerImage{
			Idle: euiimage.NewNineSliceColor(color.Transparent),
			Mask: euiimage.NewNineSliceColor(color.NRGBA{A: 0xff}),
		}),
		widget.ScrollContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
	)
	scroll.GetWidget().ScrolledEvent.AddHandler(func(args interface{}) {
		a, ok := args.(*widget.WidgetScrolledEventArgs)
		if !ok {
			return
		}
		if scrollRange := scrollableHeight(scroll); scrollRange > 0 {
			scroll.ScrollTop -= a.Y * scrollWheelStep / scrollRange
		}
	})

	root := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewAnchorLayout()))
	root.AddChild(scroll)

	return &overlay{
		ui:     &ebitenui.UI{Container: root},
		panel:  panel,
		scroll: scroll,
	}
}

// scrollableHeight is how many pixels of the panel's content are hidden above
// or below the current viewport; 0 (or less) means everything already fits.
func scrollableHeight(scroll *widget.ScrollContainer) float64 {
	view := scroll.ViewRect().Dy()
	content := scroll.ContentRect().Dy()
	if content <= view {
		return 0
	}
	return float64(content - view)
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

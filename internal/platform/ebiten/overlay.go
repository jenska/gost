package ebiten

import (
	"image"
	"math"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/input"
	"github.com/ebitenui/ebitenui/widget"
	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// overlayMargin is the free space kept around the panel inside the window.
const overlayMargin = 8

// overlay is the F12 in-session configuration HUD. It hosts the same
// configPanel as the launcher, drawn on top of the emulated ST screen without
// ever resizing or rescaling it (see App.Layout).
//
// The ST's logical screen is often much smaller than the panel (a low
// resolution picture is barely 400 pixels wide), so the overlay is not drawn
// there but onto Ebitengine's final screen, at the window's real resolution
// (see App.DrawFinalScreen). When even the window is smaller than the panel,
// the panel is scaled down to fit. The UI renders into a canvas of the
// window's size divided by that scale, and the cursor is mapped from the ST's
// logical screen into the canvas so clicks land where the panel is drawn.
type overlay struct {
	ui      *ebitenui.UI
	panel   *configPanel
	visible bool

	// panelW and panelH are the panel's natural size plus margins.
	panelW, panelH int

	canvas *ebitenlib.Image
	// toCanvas maps a cursor position on the ST's logical screen to the
	// canvas; it is updated on every draw.
	toCanvas ebitenlib.GeoM
	cursor   *overlayCursor
}

func newOverlay(app *App) *overlay {
	panel := newConfigPanel(app, configModeOverlay, app.cfg)

	root := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewAnchorLayout()))
	root.AddChild(panel.container)

	w, h := panel.container.PreferredSize()
	o := &overlay{
		ui: &ebitenui.UI{
			Container:     root,
			PreRenderHook: panel.drawShadow,
		},
		panel:  panel,
		panelW: w + 2*overlayMargin,
		panelH: h + 2*overlayMargin,
	}
	o.cursor = &overlayCursor{overlay: o}
	return o
}

func (o *overlay) Visible() bool {
	return o != nil && o.visible
}

func (o *overlay) SetVisible(visible bool) {
	o.visible = visible
	// The launcher's UI is drawn on the logical screen and needs the default,
	// unmapped cursor.
	if visible {
		input.SetCursorUpdater(o.cursor)
	} else {
		input.SetCursorUpdater(nil)
	}
}

// Update and DrawFinal are only called while the overlay is visible (see App).

func (o *overlay) Update() {
	o.panel.Refresh()
	o.ui.Update()
}

// DrawFinal draws the final screen: the ST's logical screen (offscreen),
// placed by geoM as Ebitengine would, with the overlay on top.
//
// The overlay's UI must be rendered into its canvas before anything is drawn
// onto the screen. Rendering to another image in between ends the screen's
// render pass, and a new pass onto the screen starts by clearing it (Metal's
// LoadActionClear), which would wipe the ST picture to black.
func (o *overlay) DrawFinal(screen ebitenlib.FinalScreen, offscreen *ebitenlib.Image, geoM ebitenlib.GeoM) {
	bounds := screen.Bounds()
	scale := overlayScale(bounds.Dx(), bounds.Dy(), o.panelW, o.panelH, ebitenlib.Monitor().DeviceScaleFactor())
	cw := int(math.Ceil(float64(bounds.Dx()) / scale))
	ch := int(math.Ceil(float64(bounds.Dy()) / scale))
	if o.canvas == nil || o.canvas.Bounds().Dx() != cw || o.canvas.Bounds().Dy() != ch {
		if o.canvas != nil {
			o.canvas.Deallocate()
		}
		// The canvas is drawn onto the final screen, whose shader does not
		// work with images on Ebitengine's texture atlas: keep it unmanaged,
		// as Ebitengine does for its own offscreen.
		o.canvas = ebitenlib.NewImageWithOptions(image.Rect(0, 0, cw, ch), &ebitenlib.NewImageOptions{Unmanaged: true})
	}

	o.canvas.Clear()
	o.ui.Draw(o.canvas)

	ebitenlib.DefaultDrawFinalScreen(screen, offscreen, geoM)
	op := &ebitenlib.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	if scale < 1 {
		op.Filter = ebitenlib.FilterLinear
	}
	screen.DrawImage(o.canvas, op)

	o.toCanvas = overlayCursorTransform(geoM, bounds.Min, scale)
}

// overlayScale is the factor the overlay is drawn at on a screenW x screenH
// final screen (in device pixels). It prefers deviceScale, so the panel looks
// as large as the launcher on high-DPI displays, but never more than fits. A
// scale of 1 or more is rounded down to a whole factor to keep text sharp;
// below 1 the panel is shrunk just enough to fit.
func overlayScale(screenW, screenH, panelW, panelH int, deviceScale float64) float64 {
	scale := math.Max(deviceScale, 1)
	if screenW > 0 && panelW > 0 {
		scale = math.Min(scale, float64(screenW)/float64(panelW))
	}
	if screenH > 0 && panelH > 0 {
		scale = math.Min(scale, float64(screenH)/float64(panelH))
	}
	if scale >= 1 {
		scale = math.Floor(scale)
	}
	return scale
}

// overlayCursorTransform maps the logical screen onto the overlay canvas: to
// the final screen with geoM, then into the canvas drawn at origin and scale.
func overlayCursorTransform(geoM ebitenlib.GeoM, origin image.Point, scale float64) ebitenlib.GeoM {
	m := geoM
	m.Translate(-float64(origin.X), -float64(origin.Y))
	m.Scale(1/scale, 1/scale)
	return m
}

// overlayCursor feeds ebitenui the cursor position in canvas coordinates.
// Keyboard, wheel and text input still come from ebitenui's default handler.
type overlayCursor struct {
	overlay *overlay
}

func (c *overlayCursor) Update()                                {}
func (c *overlayCursor) AfterUpdate()                           {}
func (c *overlayCursor) Draw(*ebitenlib.Image)                  {}
func (c *overlayCursor) AfterDraw(*ebitenlib.Image)             {}
func (c *overlayCursor) GetCursorImage(string) *ebitenlib.Image { return nil }
func (c *overlayCursor) GetCursorOffset(string) image.Point     { return image.Point{} }

func (c *overlayCursor) MouseButtonPressed(b ebitenlib.MouseButton) bool {
	return ebitenlib.IsMouseButtonPressed(b)
}

func (c *overlayCursor) MouseButtonJustPressed(b ebitenlib.MouseButton) bool {
	return inpututil.IsMouseButtonJustPressed(b)
}

func (c *overlayCursor) MouseButtonJustReleased(b ebitenlib.MouseButton) bool {
	return inpututil.IsMouseButtonJustReleased(b)
}

func (c *overlayCursor) CursorPosition() (int, int) {
	x, y := ebitenlib.CursorPosition()
	cx, cy := c.overlay.toCanvas.Apply(float64(x), float64(y))
	return int(math.Floor(cx)), int(math.Floor(cy))
}

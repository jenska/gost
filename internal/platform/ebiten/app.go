package ebiten

import (
	"fmt"

	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jenska/gost/internal/emulator"
)

type App struct {
	machine     *emulator.Machine
	scale       int
	texture     *ebitenlib.Image
	prevKeys    map[ebitenlib.Key]bool
	lastMouseX  int
	lastMouseY  int
	lastButtons byte
}

func Run(machine *emulator.Machine, cfg emulator.Config) error {
	app := &App{
		machine:  machine,
		scale:    cfg.Scale,
		prevKeys: make(map[ebitenlib.Key]bool),
	}

	width, height := machine.Dimensions()
	if app.scale <= 0 {
		app.scale = 2
	}
	app.texture = ebitenlib.NewImage(width, height)

	ebitenlib.SetWindowTitle("GoST Emulator")
	ebitenlib.SetWindowSize(width*app.scale, height*app.scale)
	ebitenlib.SetWindowResizingMode(ebitenlib.WindowResizingModeEnabled)
	ebitenlib.SetTPS(int(cfg.FrameHz))
	ebitenlib.SetFullscreen(cfg.Fullscreen)

	return ebitenlib.RunGame(app)
}

func (a *App) Update() error {
	a.handleKeyboard()
	a.handleMouse()

	changed, err := a.machine.StepFrame()
	if err != nil {
		return err
	}
	if changed {
		width, height := a.machine.Dimensions()
		if a.texture == nil || a.texture.Bounds().Dx() != width || a.texture.Bounds().Dy() != height {
			a.texture = ebitenlib.NewImage(width, height)
			ebitenlib.SetWindowSize(width*a.scale, height*a.scale)
		}
		a.texture.WritePixels(a.machine.FrameBuffer())
	}
	return nil
}

func (a *App) Draw(screen *ebitenlib.Image) {
	if a.texture == nil {
		return
	}
	op := &ebitenlib.DrawImageOptions{}
	op.GeoM.Scale(float64(a.scale), float64(a.scale))
	screen.DrawImage(a.texture, op)
}

func (a *App) Layout(int, int) (int, int) {
	return a.machine.Dimensions()
}

func (a *App) handleKeyboard() {
	pressed := inpututil.AppendPressedKeys(nil)
	current := make(map[ebitenlib.Key]bool, len(pressed))

	for _, key := range pressed {
		current[key] = true
		if !a.prevKeys[key] {
			if scancode, ok := atariScancode(key); ok {
				a.machine.PushKey(scancode, true)
			}
		}
	}

	for key := range a.prevKeys {
		if current[key] {
			continue
		}
		if scancode, ok := atariScancode(key); ok {
			a.machine.PushKey(scancode, false)
		}
	}

	a.prevKeys = current
}

func (a *App) handleMouse() {
	x, y := ebitenlib.CursorPosition()
	dx := x/a.scale - a.lastMouseX
	dy := y/a.scale - a.lastMouseY
	a.lastMouseX = x / a.scale
	a.lastMouseY = y / a.scale

	var buttons byte
	if ebitenlib.IsMouseButtonPressed(ebitenlib.MouseButtonLeft) {
		buttons |= 0x02
	}
	if ebitenlib.IsMouseButtonPressed(ebitenlib.MouseButtonRight) {
		buttons |= 0x01
	}

	if dx != 0 || dy != 0 || buttons != a.lastButtons {
		a.machine.PushMouse(dx, dy, buttons)
		a.lastButtons = buttons
	}
}

func atariScancode(key ebitenlib.Key) (byte, bool) {
	switch key {
	case ebitenlib.KeyA:
		return 0x1E, true
	case ebitenlib.KeyB:
		return 0x30, true
	case ebitenlib.KeyC:
		return 0x2E, true
	case ebitenlib.KeyD:
		return 0x20, true
	case ebitenlib.KeyE:
		return 0x12, true
	case ebitenlib.KeyF:
		return 0x21, true
	case ebitenlib.KeyG:
		return 0x22, true
	case ebitenlib.KeyH:
		return 0x23, true
	case ebitenlib.KeyI:
		return 0x17, true
	case ebitenlib.KeyJ:
		return 0x24, true
	case ebitenlib.KeyK:
		return 0x25, true
	case ebitenlib.KeyL:
		return 0x26, true
	case ebitenlib.KeyM:
		return 0x32, true
	case ebitenlib.KeyN:
		return 0x31, true
	case ebitenlib.KeyO:
		return 0x18, true
	case ebitenlib.KeyP:
		return 0x19, true
	case ebitenlib.KeyQ:
		return 0x10, true
	case ebitenlib.KeyR:
		return 0x13, true
	case ebitenlib.KeyS:
		return 0x1F, true
	case ebitenlib.KeyT:
		return 0x14, true
	case ebitenlib.KeyU:
		return 0x16, true
	case ebitenlib.KeyV:
		return 0x2F, true
	case ebitenlib.KeyW:
		return 0x11, true
	case ebitenlib.KeyX:
		return 0x2D, true
	case ebitenlib.KeyY:
		return 0x15, true
	case ebitenlib.KeyZ:
		return 0x2C, true
	case ebitenlib.Key0:
		return 0x0B, true
	case ebitenlib.Key1:
		return 0x02, true
	case ebitenlib.Key2:
		return 0x03, true
	case ebitenlib.Key3:
		return 0x04, true
	case ebitenlib.Key4:
		return 0x05, true
	case ebitenlib.Key5:
		return 0x06, true
	case ebitenlib.Key6:
		return 0x07, true
	case ebitenlib.Key7:
		return 0x08, true
	case ebitenlib.Key8:
		return 0x09, true
	case ebitenlib.Key9:
		return 0x0A, true
	case ebitenlib.KeySpace:
		return 0x39, true
	case ebitenlib.KeyEnter:
		return 0x1C, true
	case ebitenlib.KeyEscape:
		return 0x01, true
	case ebitenlib.KeyBackspace:
		return 0x0E, true
	case ebitenlib.KeyTab:
		return 0x0F, true
	case ebitenlib.KeyArrowUp:
		return 0x48, true
	case ebitenlib.KeyArrowDown:
		return 0x50, true
	case ebitenlib.KeyArrowLeft:
		return 0x4B, true
	case ebitenlib.KeyArrowRight:
		return 0x4D, true
	default:
		return 0, false
	}
}

func (a *App) String() string {
	width, height := a.machine.Dimensions()
	return fmt.Sprintf("gost %dx%d", width, height)
}

package ebiten

import (
	"fmt"
	"runtime"
	"time"

	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/emulator"
	"github.com/jenska/gost/internal/platform/inputmap"
	"github.com/jenska/ym2149/renderer/atarist"
	"github.com/jenska/ym2149/renderer/audiostream"
)

type App struct {
	machine     *emulator.Machine
	audio       *hostAudioQueue
	scale       float64
	texture     *ebitenlib.Image
	prevKeys    map[ebitenlib.Key]bool
	hostMouseX  int
	hostMouseY  int
	lastButtons byte
	mouseReady  bool
	cursorMode  ebitenlib.CursorModeType
}

const (
	audioBufferSize    = 75 * time.Millisecond
	audioQueueDuration = 250 * time.Millisecond
)

func Run(machine *emulator.Machine, cfg config.Config) error {
	app := &App{
		machine:  machine,
		scale:    cfg.Scale,
		prevKeys: make(map[ebitenlib.Key]bool),
	}

	width, height := machine.DisplayDimensions()
	if app.scale <= 0 {
		app.scale = 2
	}
	app.texture = ebitenlib.NewImage(width, height)
	app.resetMouseTracking()
	source := atarist.New(machine.AudioSource(), atarist.Config{})
	app.audio = newHostAudioQueue(source, audioQueueDuration)

	ebitenlib.SetWindowTitle("GoST Emulator")
	ebitenlib.SetWindowSize(scaledWindowSize(width, height, app.scale))
	ebitenlib.SetWindowResizingMode(ebitenlib.WindowResizingModeEnabled)
	ebitenlib.SetTPS(int(cfg.FrameHz))
	ebitenlib.SetFullscreen(cfg.Fullscreen)

	player, err := newAudioPlayer(app.audio)
	if err != nil {
		return err
	}
	defer player.Close()
	player.Play()

	return ebitenlib.RunGame(app)
}

func (a *App) Update() error {
	a.handleKeyboard()
	a.handleMouse()

	changed, err := a.machine.StepFrame()
	if err != nil {
		return err
	}
	a.audio.Pump()
	if changed {
		width, height := a.machine.DisplayDimensions()
		if a.texture == nil || a.texture.Bounds().Dx() != width || a.texture.Bounds().Dy() != height {
			a.texture = ebitenlib.NewImage(width, height)
			ebitenlib.SetWindowSize(scaledWindowSize(width, height, a.scale))
			a.resetMouseTracking()
		}
		a.texture.WritePixels(a.machine.DisplayFrameBuffer())
	}
	return nil
}

func (a *App) Draw(screen *ebitenlib.Image) {
	if a.texture == nil {
		return
	}
	screen.DrawImage(a.texture, nil)
}

func (a *App) Layout(int, int) (int, int) {
	return a.machine.DisplayDimensions()
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
	width, height := a.machine.DisplayDimensions()
	if width <= 0 || height <= 0 {
		width, height = a.machine.Dimensions()
	}

	var buttons byte
	if ebitenlib.IsMouseButtonPressed(ebitenlib.MouseButtonLeft) {
		buttons |= 0x02
	}
	if ebitenlib.IsMouseButtonPressed(ebitenlib.MouseButtonRight) {
		buttons |= 0x01
	}

	captured := ebitenlib.CursorMode() == ebitenlib.CursorModeCaptured
	focused := ebitenlib.IsFocused()
	if runtime.GOOS == "js" && (captured || buttons != 0 || a.mouseReady) {
		focused = true
	}
	inside := focused &&
		x >= 0 && y >= 0 &&
		x < width && y < height

	if !focused || (!inside && !captured) {
		a.setHostCursorMode(ebitenlib.CursorModeVisible)
		a.mouseReady = false
		a.lastButtons = buttons
		return
	}

	if !captured {
		a.setHostCursorMode(ebitenlib.CursorModeCaptured)
		if runtime.GOOS == "js" {
			if mouseX, mouseY, ok := a.machine.MousePosition(); ok {
				dx := x - mouseX
				dy := y - mouseY
				if dx != 0 || dy != 0 || buttons != a.lastButtons {
					a.machine.PushMouse(dx, dy, buttons)
				}
				a.hostMouseX = x
				a.hostMouseY = y
				a.mouseReady = true
				a.lastButtons = buttons
				return
			}
		}
		if !a.mouseReady {
			a.hostMouseX = x
			a.hostMouseY = y
			a.mouseReady = true
			if runtime.GOOS == "js" && buttons != a.lastButtons {
				a.machine.PushMouse(0, 0, buttons)
			}
			a.lastButtons = buttons
			return
		}
		if runtime.GOOS != "js" {
			a.hostMouseX = x
			a.hostMouseY = y
			a.lastButtons = buttons
			return
		}
	}

	if !a.mouseReady {
		a.hostMouseX = x
		a.hostMouseY = y
		a.mouseReady = true
		a.lastButtons = buttons
		return
	}

	dx := x - a.hostMouseX
	dy := y - a.hostMouseY
	a.hostMouseX = x
	a.hostMouseY = y

	if dx != 0 || dy != 0 || buttons != a.lastButtons {
		a.machine.PushMouse(dx, dy, buttons)
		a.lastButtons = buttons
	}
}

func (a *App) resetMouseTracking() {
	a.hostMouseX = 0
	a.hostMouseY = 0
	a.lastButtons = 0
	a.mouseReady = false
	a.cursorMode = ebitenlib.CursorModeVisible
}

func (a *App) setHostCursorMode(mode ebitenlib.CursorModeType) {
	if a.cursorMode == mode && ebitenlib.CursorMode() == mode {
		return
	}
	ebitenlib.SetCursorMode(mode)
	a.cursorMode = mode
}

func scaledWindowSize(width, height int, scale float64) (int, int) {
	return int(float64(width) * scale), int(float64(height) * scale)
}

func newAudioPlayer(source emulator.AudioSource) (*audio.Player, error) {
	reader := audiostream.NewReader(source, 1024)

	ctx, err := ensureAudioContext(reader.OutputSampleRate())
	if err != nil {
		return nil, fmt.Errorf("create audio context: %w", err)
	}

	player, err := ctx.NewPlayerF32(reader)
	if err != nil {
		return nil, fmt.Errorf("create audio player: %w", err)
	}
	player.SetBufferSize(audioBufferSize)
	return player, nil
}

func ensureAudioContext(sampleRate int) (*audio.Context, error) {
	if ctx := audio.CurrentContext(); ctx != nil {
		if ctx.SampleRate() != sampleRate {
			return nil, fmt.Errorf("existing Ebiten audio context uses sample rate %d, want %d", ctx.SampleRate(), sampleRate)
		}
		return ctx, nil
	}
	return audio.NewContext(sampleRate), nil
}

func atariScancode(key ebitenlib.Key) (byte, bool) {
	return inputmap.AtariScancode(hostKeyFromEbiten(key))
}

func hostKeyFromEbiten(key ebitenlib.Key) inputmap.Key {
	switch key {
	case ebitenlib.KeyA:
		return inputmap.KeyA
	case ebitenlib.KeyB:
		return inputmap.KeyB
	case ebitenlib.KeyC:
		return inputmap.KeyC
	case ebitenlib.KeyD:
		return inputmap.KeyD
	case ebitenlib.KeyE:
		return inputmap.KeyE
	case ebitenlib.KeyF:
		return inputmap.KeyF
	case ebitenlib.KeyG:
		return inputmap.KeyG
	case ebitenlib.KeyH:
		return inputmap.KeyH
	case ebitenlib.KeyI:
		return inputmap.KeyI
	case ebitenlib.KeyJ:
		return inputmap.KeyJ
	case ebitenlib.KeyK:
		return inputmap.KeyK
	case ebitenlib.KeyL:
		return inputmap.KeyL
	case ebitenlib.KeyM:
		return inputmap.KeyM
	case ebitenlib.KeyN:
		return inputmap.KeyN
	case ebitenlib.KeyO:
		return inputmap.KeyO
	case ebitenlib.KeyP:
		return inputmap.KeyP
	case ebitenlib.KeyQ:
		return inputmap.KeyQ
	case ebitenlib.KeyR:
		return inputmap.KeyR
	case ebitenlib.KeyS:
		return inputmap.KeyS
	case ebitenlib.KeyT:
		return inputmap.KeyT
	case ebitenlib.KeyU:
		return inputmap.KeyU
	case ebitenlib.KeyV:
		return inputmap.KeyV
	case ebitenlib.KeyW:
		return inputmap.KeyW
	case ebitenlib.KeyX:
		return inputmap.KeyX
	case ebitenlib.KeyY:
		return inputmap.KeyY
	case ebitenlib.KeyZ:
		return inputmap.KeyZ
	case ebitenlib.Key0, ebitenlib.KeyNumpad0:
		return inputmap.Key0
	case ebitenlib.Key1, ebitenlib.KeyNumpad1:
		return inputmap.Key1
	case ebitenlib.Key2, ebitenlib.KeyNumpad2:
		return inputmap.Key2
	case ebitenlib.Key3, ebitenlib.KeyNumpad3:
		return inputmap.Key3
	case ebitenlib.Key4, ebitenlib.KeyNumpad4:
		return inputmap.Key4
	case ebitenlib.Key5, ebitenlib.KeyNumpad5:
		return inputmap.Key5
	case ebitenlib.Key6, ebitenlib.KeyNumpad6:
		return inputmap.Key6
	case ebitenlib.Key7, ebitenlib.KeyNumpad7:
		return inputmap.Key7
	case ebitenlib.Key8, ebitenlib.KeyNumpad8:
		return inputmap.Key8
	case ebitenlib.Key9, ebitenlib.KeyNumpad9:
		return inputmap.Key9
	case ebitenlib.KeySpace:
		return inputmap.KeySpace
	case ebitenlib.KeyEnter:
		return inputmap.KeyEnter
	case ebitenlib.KeyNumpadEnter:
		return inputmap.KeyNumpadEnter
	case ebitenlib.KeyEscape:
		return inputmap.KeyEscape
	case ebitenlib.KeyBackspace:
		return inputmap.KeyBackspace
	case ebitenlib.KeyTab:
		return inputmap.KeyTab
	case ebitenlib.KeyShiftLeft:
		return inputmap.KeyShiftLeft
	case ebitenlib.KeyShiftRight:
		return inputmap.KeyShiftRight
	case ebitenlib.KeyControlLeft, ebitenlib.KeyControlRight:
		return inputmap.KeyControlLeft
	case ebitenlib.KeyAltLeft, ebitenlib.KeyAltRight, ebitenlib.KeyMetaLeft, ebitenlib.KeyMetaRight:
		return inputmap.KeyAltLeft
	case ebitenlib.KeyCapsLock:
		return inputmap.KeyCapsLock
	case ebitenlib.KeyMinus, ebitenlib.KeyNumpadSubtract:
		return inputmap.KeyMinus
	case ebitenlib.KeyEqual:
		return inputmap.KeyEqual
	case ebitenlib.KeyBracketLeft:
		return inputmap.KeyBracketLeft
	case ebitenlib.KeyBracketRight:
		return inputmap.KeyBracketRight
	case ebitenlib.KeySemicolon:
		return inputmap.KeySemicolon
	case ebitenlib.KeyQuote:
		return inputmap.KeyQuote
	case ebitenlib.KeyBackquote:
		return inputmap.KeyBackquote
	case ebitenlib.KeyBackslash:
		return inputmap.KeyBackslash
	case ebitenlib.KeyComma:
		return inputmap.KeyComma
	case ebitenlib.KeyPeriod, ebitenlib.KeyNumpadDecimal:
		return inputmap.KeyPeriod
	case ebitenlib.KeySlash, ebitenlib.KeyNumpadDivide:
		return inputmap.KeySlash
	case ebitenlib.KeyArrowUp:
		return inputmap.KeyArrowUp
	case ebitenlib.KeyArrowDown:
		return inputmap.KeyArrowDown
	case ebitenlib.KeyArrowLeft:
		return inputmap.KeyArrowLeft
	case ebitenlib.KeyArrowRight:
		return inputmap.KeyArrowRight
	case ebitenlib.KeyHome:
		return inputmap.KeyHome
	case ebitenlib.KeyInsert:
		return inputmap.KeyInsert
	case ebitenlib.KeyDelete:
		return inputmap.KeyDelete
	case ebitenlib.KeyF1:
		return inputmap.KeyF1
	case ebitenlib.KeyF2:
		return inputmap.KeyF2
	case ebitenlib.KeyF3:
		return inputmap.KeyF3
	case ebitenlib.KeyF4:
		return inputmap.KeyF4
	case ebitenlib.KeyF5:
		return inputmap.KeyF5
	case ebitenlib.KeyF6:
		return inputmap.KeyF6
	case ebitenlib.KeyF7:
		return inputmap.KeyF7
	case ebitenlib.KeyF8:
		return inputmap.KeyF8
	case ebitenlib.KeyF9:
		return inputmap.KeyF9
	case ebitenlib.KeyF10:
		return inputmap.KeyF10
	default:
		return inputmap.KeyUnknown
	}
}

func (a *App) String() string {
	width, height := a.machine.DisplayDimensions()
	return fmt.Sprintf("gost %dx%d", width, height)
}

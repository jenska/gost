package ebiten

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"time"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/widget"
	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/emulator"
	"github.com/jenska/gost/internal/platform/host"
	"github.com/jenska/gost/internal/platform/inputmap"
	"github.com/jenska/ym2149/renderer/atarist"
	"github.com/jenska/ym2149/renderer/audiostream"
)

type appMode int

const (
	modeRunning appMode = iota
	modeLauncher
)

// fileTarget identifies which configuration path a native file dialog fills in.
type fileTarget int

const (
	targetFloppyA fileTarget = iota
	targetFloppyB
	targetROM
	targetHardDiskOpen
	targetHardDiskNew
)

type App struct {
	machine *emulator.Machine
	session *emulator.Session
	cfg     config.Config

	audio  *hostAudioQueue
	player *audio.Player

	scale       float64
	texture     *ebitenlib.Image
	prevKeys    map[ebitenlib.Key]bool
	hostMouseX  int
	hostMouseY  int
	lastButtons byte
	mouseReady  bool
	cursorMode  ebitenlib.CursorModeType

	mode          appMode
	launcher      *ebitenui.UI
	launcherPanel *configPanel
	overlay       *overlay

	// buildMachine assembles a fresh Session from a config; overridable in tests.
	buildMachine func(*config.Config) (*emulator.Session, error)

	mountedFloppies      [2]string
	hostCommands         []hostCommand
	lastHostCommandError string

	fileSelector      func() (string, error)
	fileDialogResults chan fileDialogResult
	fileDialogPending map[fileTarget]bool
	selectedPaths     map[fileTarget]string
}

const (
	audioBufferSize    = 75 * time.Millisecond
	audioQueueDuration = 250 * time.Millisecond

	// configWindowWidth/Height size the window while the configuration panel is
	// on screen (launcher, or the F12 overlay during a session).
	configWindowWidth  = 700
	configWindowHeight = 540
)

var launcherBackgroundColor = panelRowBackgroundColor

type hostCommandKind int

const (
	hostCommandMountFloppy hostCommandKind = iota
	hostCommandEjectFloppy
	hostCommandReboot
)

type hostCommand struct {
	kind  hostCommandKind
	drive int
	path  string
	cfg   *config.Config
}

type fileDialogResult struct {
	target fileTarget
	path   string
	err    error
}

// Run drives the desktop frontend for the machine already assembled in session.
// It owns the machine lifecycle from here: an "Apply & Reboot" rebuilds it, and
// the hard-disk image and last-used config are persisted on exit.
func Run(session *emulator.Session, cfg config.Config, startInLauncher bool) error {
	app := &App{
		machine:           session.Machine,
		session:           session,
		cfg:               cfg,
		scale:             cfg.Scale,
		mountedFloppies:   [2]string{cfg.FloppyA, cfg.FloppyB},
		prevKeys:          make(map[ebitenlib.Key]bool),
		buildMachine:      emulator.BuildMachine,
		fileDialogPending: map[fileTarget]bool{},
		selectedPaths:     map[fileTarget]string{},
	}

	width, height := app.machine.DisplayDimensions()
	app.scale = clampScale(app.scale)
	app.texture = ebitenlib.NewImage(width, height)
	app.resetMouseTracking()
	source := atarist.New(app.machine.AudioSource(), atarist.Config{})
	app.audio = newHostAudioQueue(source, audioQueueDuration)

	ebitenlib.SetWindowTitle("GoST Emulator")
	ebitenlib.SetWindowResizingMode(ebitenlib.WindowResizingModeEnabled)
	ebitenlib.SetTPS(int(cfg.FrameHz))
	ebitenlib.SetFullscreen(cfg.Fullscreen)

	player, err := newAudioPlayer(app.audio)
	if err != nil {
		return err
	}
	defer player.Close()
	app.player = player
	player.Play()

	if startInLauncher {
		app.enterLauncher()
	} else {
		app.applyRunningWindow()
	}

	runErr := ebitenlib.RunGame(app)

	if saveErr := app.session.PersistHardDisk(); saveErr != nil && runErr == nil {
		runErr = saveErr
	}
	if runErr == nil && app.cfg.DumpFramePath != "" {
		if dumpErr := app.machine.DumpFramePNG(app.cfg.DumpFramePath); dumpErr != nil {
			runErr = dumpErr
		}
	}
	if runErr == nil {
		if err := config.SaveLastConfig(&app.cfg); err != nil {
			fmt.Fprintf(os.Stderr, "save last config: %v\n", err)
		}
	}
	return runErr
}

func (a *App) enterLauncher() {
	a.mode = modeLauncher
	root := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewAnchorLayout()))
	panel := newConfigPanel(a, configModeLauncher, a.cfg)
	root.AddChild(panel.container)
	a.launcher = &ebitenui.UI{Container: root}
	a.launcherPanel = panel
	ebitenlib.SetWindowSize(configWindowWidth, configWindowHeight)
}

func (a *App) applyRunningWindow() {
	a.mode = modeRunning
	a.launcher = nil
	a.launcherPanel = nil
	a.setRunningWindowSize()
}

func (a *App) setRunningWindowSize() {
	if width, height := a.machine.DisplayDimensions(); width > 0 && height > 0 {
		ebitenlib.SetWindowSize(scaledWindowSize(width, height, a.scale))
	}
}

// applyDisplayConfig applies the scale and fullscreen settings from a.cfg to the
// host window. These are host-only settings and never require a reboot.
func (a *App) applyDisplayConfig() {
	a.scale = clampScale(a.cfg.Scale)
	ebitenlib.SetFullscreen(a.cfg.Fullscreen)
	a.setRunningWindowSize()
}

// applyConfig is the callback the configPanel invokes on Start / Apply & Reboot.
func (a *App) applyConfig(newCfg config.Config) {
	if !a.machineConfigChanged(newCfg) {
		a.cfg = newCfg
		if a.mode == modeLauncher {
			a.applyRunningWindow()
		} else {
			a.setOverlayVisible(false)
		}
		a.applyDisplayConfig()
		return
	}
	stored := newCfg
	a.hostCommands = append(a.hostCommands, hostCommand{kind: hostCommandReboot, cfg: &stored})
	a.setOverlayVisible(false)
}

// machineConfigChanged reports whether newCfg differs from the active config in a
// way that requires rebuilding the machine. Host-only display settings (scale,
// fullscreen) are ignored.
func (a *App) machineConfigChanged(newCfg config.Config) bool {
	old, cur := a.cfg, newCfg
	old.Scale, cur.Scale = 0, 0
	old.Fullscreen, cur.Fullscreen = false, false
	return !reflect.DeepEqual(old.ToPatch(), cur.ToPatch())
}

// clampScale is a runtime sanity bound on the window scale, independent of the
// configuration panel's slider range (1-4): CLI/JSON configs may still request
// a larger window.
func clampScale(scale float64) float64 {
	if scale < 1 {
		return 1
	}
	if scale > 8 {
		return 8
	}
	return scale
}

func (a *App) Update() error {
	a.applyFileDialogResults()
	a.applyHostCommands()

	if a.mode == modeLauncher {
		a.setHostCursorMode(ebitenlib.CursorModeVisible)
		if a.launcherPanel != nil {
			a.launcherPanel.Refresh()
		}
		if a.launcher != nil {
			a.launcher.Update()
		}
		return nil
	}

	a.handleOverlayToggle()
	if a.overlayVisible() {
		a.setHostCursorMode(ebitenlib.CursorModeVisible)
		a.overlay.Update()
	} else {
		a.handleKeyboard()
		a.handleMouse()
	}

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

// reboot rebuilds the machine from newCfg, cold-booting the emulated ST while
// keeping the same window, audio player, and Ebiten game loop.
func (a *App) reboot(newCfg config.Config) error {
	if err := a.session.PersistHardDisk(); err != nil {
		return fmt.Errorf("save hard disk image: %w", err)
	}
	session, err := a.buildMachine(&newCfg)
	if err != nil {
		return err
	}

	a.session = session
	a.machine = session.Machine
	a.cfg = newCfg
	a.mountedFloppies = [2]string{newCfg.FloppyA, newCfg.FloppyB}
	a.selectedPaths = map[fileTarget]string{}
	a.audio.setSource(atarist.New(a.machine.AudioSource(), atarist.Config{}))
	if newCfg.Trace != "" {
		a.machine.EnableTrace(newCfg.Trace, os.Stdout)
	}

	if width, height := a.machine.DisplayDimensions(); width > 0 && height > 0 {
		a.texture = ebitenlib.NewImage(width, height)
	}
	a.resetMouseTracking()
	a.scale = clampScale(newCfg.Scale)
	ebitenlib.SetFullscreen(newCfg.Fullscreen)
	a.applyRunningWindow()
	if newCfg.FrameHz > 0 {
		ebitenlib.SetTPS(int(newCfg.FrameHz))
	}
	return nil
}

func (a *App) QueueMountFloppy(drive int, path string) {
	a.hostCommands = append(a.hostCommands, hostCommand{
		kind:  hostCommandMountFloppy,
		drive: drive,
		path:  path,
	})
}

func (a *App) QueueEjectFloppy(drive int) {
	a.hostCommands = append(a.hostCommands, hostCommand{
		kind:  hostCommandEjectFloppy,
		drive: drive,
	})
}

func (a *App) MountedFloppyPath(drive int) string {
	if drive < 0 || drive >= len(a.mountedFloppies) {
		return ""
	}
	return a.mountedFloppies[drive]
}

func (a *App) LastHostCommandError() string {
	return a.lastHostCommandError
}

// QueueBrowse opens a native file dialog for the given configuration target on a
// background goroutine; the result is picked up in applyFileDialogResults.
func (a *App) QueueBrowse(target fileTarget) {
	if a.fileDialogPending == nil {
		a.fileDialogPending = map[fileTarget]bool{}
	}
	if a.selectedPaths == nil {
		a.selectedPaths = map[fileTarget]string{}
	}
	if a.fileDialogPending[target] {
		a.lastHostCommandError = "file selector already open"
		return
	}
	if a.fileDialogResults == nil {
		a.fileDialogResults = make(chan fileDialogResult, 8)
	}
	a.fileDialogPending[target] = true
	a.lastHostCommandError = "Opening file selector..."
	results := a.fileDialogResults
	selector := a.selectorFor(target)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "file dialog panic: %v\n", r)
				results <- fileDialogResult{target: target, err: fmt.Errorf("file dialog failed: %v", r)}
			}
		}()
		path, err := selector()
		results <- fileDialogResult{target: target, path: path, err: err}
	}()
}

func (a *App) selectorFor(target fileTarget) func() (string, error) {
	if a.fileSelector != nil {
		return a.fileSelector
	}
	switch target {
	case targetFloppyA, targetFloppyB:
		return host.SelectFloppyDiskImage
	case targetROM:
		return func() (string, error) {
			return host.OpenFile(host.FileDialogSpec{
				Title:      "Select a TOS ROM image",
				Extensions: []string{"img", "rom", "bin"},
			})
		}
	case targetHardDiskOpen:
		return func() (string, error) {
			return host.OpenFile(host.FileDialogSpec{
				Title:      "Select a hard disk image",
				Extensions: []string{"img", "hdi", "hd", "raw"},
			})
		}
	case targetHardDiskNew:
		return func() (string, error) {
			return host.SaveFile(host.FileDialogSpec{
				Title:      "Create a hard disk image",
				Extensions: []string{"img", "hdi"},
			})
		}
	default:
		return func() (string, error) { return "", host.ErrFileDialogUnsupported }
	}
}

func (a *App) SelectedPath(target fileTarget) string {
	if a.selectedPaths == nil {
		return ""
	}
	return a.selectedPaths[target]
}

func (a *App) ClearSelectedPath(target fileTarget) {
	if a.selectedPaths != nil {
		delete(a.selectedPaths, target)
	}
}

func (a *App) applyFileDialogResults() {
	for a.fileDialogResults != nil {
		select {
		case result := <-a.fileDialogResults:
			a.applyFileDialogResult(result)
		default:
			return
		}
	}
}

func (a *App) applyFileDialogResult(result fileDialogResult) {
	if a.fileDialogPending != nil {
		a.fileDialogPending[result.target] = false
	}
	if result.err != nil {
		if errors.Is(result.err, host.ErrFileDialogCanceled) {
			a.lastHostCommandError = ""
			return
		}
		a.lastHostCommandError = result.err.Error()
		return
	}
	if a.selectedPaths == nil {
		a.selectedPaths = map[fileTarget]string{}
	}
	a.selectedPaths[result.target] = result.path
	a.lastHostCommandError = ""
}

func (a *App) applyHostCommands() {
	commands := a.hostCommands
	a.hostCommands = nil
	for _, command := range commands {
		if err := a.applyHostCommand(command); err != nil {
			a.lastHostCommandError = err.Error()
			return
		}
		a.lastHostCommandError = ""
	}
}

func (a *App) applyHostCommand(command hostCommand) error {
	switch command.kind {
	case hostCommandMountFloppy:
		return a.mountFloppy(command.drive, command.path)
	case hostCommandEjectFloppy:
		return a.ejectFloppy(command.drive)
	case hostCommandReboot:
		if command.cfg == nil {
			return fmt.Errorf("reboot command missing config")
		}
		return a.reboot(*command.cfg)
	default:
		return fmt.Errorf("unsupported host command %d", command.kind)
	}
}

func (a *App) mountFloppy(drive int, path string) error {
	if drive < 0 || drive >= len(a.mountedFloppies) {
		return fmt.Errorf("unsupported floppy drive %d", drive)
	}
	disk, err := emulator.LoadDiskImage(path)
	if err != nil {
		return fmt.Errorf("load drive %c disk: %w", 'A'+drive, err)
	}
	if err := a.machine.InsertFloppy(drive, disk); err != nil {
		return fmt.Errorf("insert drive %c disk: %w", 'A'+drive, err)
	}
	a.mountedFloppies[drive] = path
	return nil
}

func (a *App) ejectFloppy(drive int) error {
	if drive < 0 || drive >= len(a.mountedFloppies) {
		return fmt.Errorf("unsupported floppy drive %d", drive)
	}
	if err := a.machine.EjectFloppy(drive); err != nil {
		return fmt.Errorf("eject drive %c disk: %w", 'A'+drive, err)
	}
	a.mountedFloppies[drive] = ""
	return nil
}

func (a *App) Draw(screen *ebitenlib.Image) {
	if a.mode == modeLauncher {
		screen.Fill(launcherBackgroundColor)
		if a.launcher != nil {
			a.launcher.Draw(screen)
		}
		return
	}
	if a.texture == nil {
		return
	}
	screen.DrawImage(a.texture, nil)
	if a.overlayVisible() {
		a.overlay.Draw(screen)
	}
}

func (a *App) Layout(int, int) (int, int) {
	if a.mode == modeLauncher {
		return configWindowWidth, configWindowHeight
	}
	// The F12 overlay is a HUD drawn on top of the emulated screen: it never
	// changes the display's logical size, so the ST picture never rescales or
	// repositions when it opens. If the panel is taller or wider than the
	// current ST resolution, it scrolls instead (see overlay.go).
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

func (a *App) handleOverlayToggle() {
	if inpututil.IsKeyJustPressed(ebitenlib.KeyF12) {
		a.setOverlayVisible(!a.overlayVisible())
	}
}

func (a *App) overlayVisible() bool {
	return a.overlay.Visible()
}

func (a *App) setOverlayVisible(visible bool) {
	if visible {
		// Rebuild each time so the panel reflects the current configuration.
		// The OS window is left as-is; Layout grows the logical canvas instead.
		a.overlay = newOverlay(a)
		a.overlay.SetVisible(true)
		a.resetMouseTracking()
		return
	}
	if a.overlay != nil {
		a.overlay.SetVisible(false)
	}
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

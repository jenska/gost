package ebiten

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/emulator"
	"github.com/jenska/ym2149/renderer/atarist"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	session, err := emulator.BuildMachine(config.DefaultConfig())
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}
	return &App{
		machine:           session.Machine,
		session:           session,
		cfg:               *config.DefaultConfig(),
		scale:             2,
		buildMachine:      emulator.BuildMachine,
		fileDialogPending: map[fileTarget]bool{},
		selectedPaths:     map[fileTarget]string{},
	}
}

func TestAppAppliesQueuedFloppyMountAndEject(t *testing.T) {
	diskPath := filepath.Join(t.TempDir(), "disk.st")
	if err := os.WriteFile(diskPath, make([]byte, 512), 0o644); err != nil {
		t.Fatalf("write disk image: %v", err)
	}
	app := newTestApp(t)

	app.QueueMountFloppy(1, diskPath)
	app.applyHostCommands()
	if got := app.MountedFloppyPath(1); got != diskPath {
		t.Fatalf("mounted drive B path = %q, want %q", got, diskPath)
	}
	if got := app.LastHostCommandError(); got != "" {
		t.Fatalf("unexpected host command error: %s", got)
	}

	app.QueueEjectFloppy(1)
	app.applyHostCommands()
	if got := app.MountedFloppyPath(1); got != "" {
		t.Fatalf("mounted drive B path after eject = %q, want empty", got)
	}
}

func TestAppRecordsQueuedFloppyMountErrors(t *testing.T) {
	app := newTestApp(t)

	app.QueueMountFloppy(0, filepath.Join(t.TempDir(), "missing.st"))
	app.applyHostCommands()
	if got := app.MountedFloppyPath(0); got != "" {
		t.Fatalf("drive A path after failed mount = %q, want empty", got)
	}
	if got := app.LastHostCommandError(); got == "" {
		t.Fatalf("expected host command error for missing disk")
	}
}

func TestAppOverlayVisibility(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)

	if app.overlayVisible() {
		t.Fatalf("overlay should be hidden initially")
	}
	app.setOverlayVisible(true)
	if !app.overlayVisible() {
		t.Fatalf("overlay should be visible after enabling")
	}
	if app.overlay == nil || app.overlay.ui == nil {
		t.Fatalf("overlay should initialize its Ebiten UI root")
	}
	app.setOverlayVisible(false)
	if app.overlayVisible() {
		t.Fatalf("overlay should be hidden after disabling")
	}
}

func TestAppLayoutStaysFixedWhileOverlayIsOpen(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	if _, err := app.machine.StepFrame(); err != nil {
		t.Fatalf("step frame: %v", err)
	}
	beforeW, beforeH := app.Layout(0, 0)
	if beforeW == 0 || beforeH == 0 {
		t.Fatalf("expected a non-zero display size after stepping a frame, got %dx%d", beforeW, beforeH)
	}

	app.setOverlayVisible(true)
	if openW, openH := app.Layout(0, 0); openW != beforeW || openH != beforeH {
		t.Fatalf("F12 overlay changed the display size: got %dx%d, want %dx%d (the ST screen must not rescale)", openW, openH, beforeW, beforeH)
	}

	app.setOverlayVisible(false)
	if closedW, closedH := app.Layout(0, 0); closedW != beforeW || closedH != beforeH {
		t.Fatalf("closing the overlay changed the display size: got %dx%d, want %dx%d", closedW, closedH, beforeW, beforeH)
	}
}

func TestOverlayScaleFitsPanelIntoWindow(t *testing.T) {
	cases := []struct {
		screenW, screenH int
		deviceScale      float64
		want             float64
	}{
		{screenW: 1280, screenH: 800, deviceScale: 1, want: 1},
		{screenW: 700, screenH: 507, deviceScale: 1, want: 1},
		{screenW: 640, screenH: 400, deviceScale: 1, want: 400.0 / 507},
		{screenW: 340, screenH: 507, deviceScale: 1, want: 0.5},
		// High-DPI: as large as the device scale allows, in whole factors.
		{screenW: 1536, screenH: 1024, deviceScale: 2, want: 2},
		{screenW: 1280, screenH: 800, deviceScale: 2, want: 1},
		{screenW: 640, screenH: 400, deviceScale: 2, want: 400.0 / 507},
	}
	for _, c := range cases {
		got := overlayScale(c.screenW, c.screenH, 680, 507, c.deviceScale)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("overlayScale(%d, %d, dpi %v) = %v, want %v", c.screenW, c.screenH, c.deviceScale, got, c.want)
		}
		if w, h := 680*got, 507*got; w > float64(c.screenW)+1e-9 || h > float64(c.screenH)+1e-9 {
			t.Errorf("screen %dx%d: scaled panel %.1fx%.1f does not fit", c.screenW, c.screenH, w, h)
		}
	}
}

func TestOverlayCursorTransformMapsLogicalToCanvas(t *testing.T) {
	// A 320x200 ST screen shown 3x with a 10px letterbox, overlay at 0.5.
	var geoM ebitenlib.GeoM
	geoM.Scale(3, 3)
	geoM.Translate(10, 0)
	m := overlayCursorTransform(geoM, image.Point{}, 0.5)
	x, y := m.Apply(100, 50)
	if x != 620 || y != 300 {
		t.Fatalf("logical (100,50) maps to canvas (%v,%v), want (620,300)", x, y)
	}
}

func TestOverlayPanelFitsTypicalWindows(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	o := newOverlay(app)
	if o.panelW > configWindowWidth || o.panelH > configWindowHeight {
		t.Fatalf("overlay panel %dx%d exceeds the %dx%d launcher window", o.panelW, o.panelH, configWindowWidth, configWindowHeight)
	}
}

func TestAppRebuildSwapsMachineAndAudioSource(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	source := atarist.New(app.machine.AudioSource(), atarist.Config{})
	app.audio = newHostAudioQueue(source, audioQueueDuration)
	before := app.machine

	newCfg := app.cfg
	newCfg.RAMSize = 2 * 1024 * 1024
	newCfg.ColorMonitor = !newCfg.ColorMonitor

	if err := app.reboot(newCfg); err != nil {
		t.Fatalf("reboot: %v", err)
	}
	if app.machine == before {
		t.Fatalf("reboot did not replace the machine")
	}
	if app.cfg.RAMSize != newCfg.RAMSize {
		t.Fatalf("app config RAM = %d, want %d", app.cfg.RAMSize, newCfg.RAMSize)
	}
	if same := app.audio.currentSource(); same == emulator.AudioSource(source) {
		t.Fatalf("reboot did not swap the audio source")
	}
}

func TestAppApplyConfigFromLauncherQueuesReboot(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	app.mode = modeLauncher

	changed := app.cfg
	changed.RAMSize = 2 * 1024 * 1024
	app.applyConfig(changed)

	if len(app.hostCommands) != 1 || app.hostCommands[0].kind != hostCommandReboot {
		t.Fatalf("expected a queued reboot command, got %+v", app.hostCommands)
	}
}

func TestAppApplyConfigFromLauncherWithoutChangesJustStarts(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	app.mode = modeLauncher

	app.applyConfig(app.cfg)

	if len(app.hostCommands) != 0 {
		t.Fatalf("expected no reboot command, got %+v", app.hostCommands)
	}
	if app.mode != modeRunning {
		t.Fatalf("expected app to switch to running mode")
	}
}

func TestConfigPanelBrowseFillsPathInput(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	app.fileSelector = func() (string, error) { return "/tmp/selected.stx", nil }
	panel := newConfigPanel(app, configModeOverlay, app.cfg)

	panel.app.QueueBrowse(targetFloppyA)
	waitForFileDialogResult(t, app)
	panel.Refresh()

	if got := panel.pathInputs[targetFloppyA].GetText(); got != "/tmp/selected.stx" {
		t.Fatalf("floppy A input = %q, want selected path", got)
	}
}

func TestConfigPanelApplyReadsPathInputsAndValidates(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	app.mode = modeLauncher
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	panel.pathInputs[targetFloppyB].SetText("/tmp/game.stx")
	panel.work.ColorMonitor = !panel.work.ColorMonitor
	panel.apply()

	if len(app.hostCommands) != 1 || app.hostCommands[0].kind != hostCommandReboot {
		t.Fatalf("expected queued reboot, got %+v", app.hostCommands)
	}
	if got := app.hostCommands[0].cfg.FloppyB; got != "/tmp/game.stx" {
		t.Fatalf("applied FloppyB = %q, want /tmp/game.stx", got)
	}
}

func TestConfigPanelApplyPresetUpdatesMachineFields(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	for i := range config.MachinePresets {
		preset := &config.MachinePresets[i]
		panel.selectPreset(preset)
		if got := panel.matchingPreset(); got != preset {
			t.Fatalf("after selecting preset %s, matching preset = %s", preset.ID, got.ID)
		}
		if panel.work.RAMSize != preset.RAMSize || panel.work.Model != preset.Model {
			t.Fatalf("preset %s not applied: RAM=%d model=%s", preset.ID, panel.work.RAMSize, panel.work.Model)
		}
		if panel.presetCombo.SelectedEntry() != preset {
			t.Fatalf("preset combo shows %v, want %s", panel.presetCombo.SelectedEntry(), preset.ID)
		}
	}
}

func TestConfigPanelPresetComboShowsCustomAndKeepsMonitor(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	preset := &config.MachinePresets[0]
	panel.selectPreset(preset)
	panel.work.ColorMonitor = !preset.ColorMonitor
	// Re-reporting the already matching preset (as the combo does when it is
	// synced) must not re-apply it and reset the monitor.
	panel.selectPreset(preset)
	if panel.work.ColorMonitor == preset.ColorMonitor {
		t.Fatalf("re-selecting the matching preset reset the monitor")
	}

	panel.work.RAMSize = 256 * 1024
	panel.Refresh()
	if panel.presetCombo.SelectedEntry() != customPreset {
		t.Fatalf("preset combo shows %v, want Custom", panel.presetCombo.SelectedEntry())
	}
}

func TestConfigPanelProfileSaveLoadRoundTrip(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	panel.work.RAMSize = 2 * 1024 * 1024
	wantRAM := panel.work.RAMSize
	panel.profileName.SetText("test-profile")
	panel.saveProfile()

	other := newConfigPanel(app, configModeLauncher, app.cfg)
	other.profileName.SetText("test-profile")
	other.loadProfile()
	if other.work.RAMSize != wantRAM {
		t.Fatalf("loaded RAM = %d, want %d", other.work.RAMSize, wantRAM)
	}
}

func TestConfigPanelDisplayChangesApplyWithoutReboot(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	app.mode = modeLauncher
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	panel.work.Scale = 3
	panel.work.Fullscreen = true
	panel.apply()

	if len(app.hostCommands) != 0 {
		t.Fatalf("display-only change queued a reboot: %+v", app.hostCommands)
	}
	if app.mode != modeRunning {
		t.Fatalf("expected running mode after Start")
	}
	if app.cfg.Scale != 3 || !app.cfg.Fullscreen {
		t.Fatalf("applied cfg scale=%v fullscreen=%v, want 3/true", app.cfg.Scale, app.cfg.Fullscreen)
	}
	if app.scale != 3 {
		t.Fatalf("app.scale = %v, want 3", app.scale)
	}
}

func TestConfigPanelScaleAndFullscreenRoundTripInProfile(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeLauncher, app.cfg)
	panel.work.Scale = 4
	panel.work.Fullscreen = true
	panel.profileName.SetText("big")
	panel.saveProfile()

	other := newConfigPanel(app, configModeLauncher, app.cfg)
	other.profileName.SetText("big")
	other.loadProfile()
	if other.work.Scale != 4 || !other.work.Fullscreen {
		t.Fatalf("loaded scale=%v fullscreen=%v, want 4/true", other.work.Scale, other.work.Fullscreen)
	}
}

func TestConfigPanelFitsConfigWindow(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	for _, mode := range []configMode{configModeLauncher, configModeOverlay} {
		panel := newConfigPanel(app, mode, app.cfg)
		for _, page := range panel.pages {
			panel.selectPage(page)
			w, h := panel.container.PreferredSize()
			if w > configWindowWidth {
				t.Errorf("mode %d, page %q: panel width %d exceeds window width %d", mode, page.title, w, configWindowWidth)
			}
			if h > configWindowHeight {
				t.Errorf("mode %d, page %q: panel height %d exceeds window height %d", mode, page.title, h, configWindowHeight)
			}
			if pw, ph := page.content.PreferredSize(); pw > pageInnerWidth || ph > panelBodyHeight-60 {
				t.Errorf("mode %d, page %q: content %dx%d overflows the page area", mode, page.title, pw, ph)
			}
		}
	}
}

func TestConfigPanelRemembersSelectedTopic(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeOverlay, app.cfg)
	display := panel.pages[len(panel.pages)-2]
	panel.selectPage(display)

	reopened := newConfigPanel(app, configModeOverlay, app.cfg)
	if reopened.current.title != display.title {
		t.Fatalf("reopened panel shows %q, want %q", reopened.current.title, display.title)
	}
}

func waitForFileDialogResult(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		app.applyFileDialogResults()
		pending := false
		for _, p := range app.fileDialogPending {
			pending = pending || p
		}
		if !pending {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for file dialog result")
}

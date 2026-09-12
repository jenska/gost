package ebiten

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestOverlayWrapsPanelInScrollContainer(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)

	o := newOverlay(app)
	if o.scroll == nil {
		t.Fatalf("overlay should host the config panel in a scroll container")
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
	panel.cycleMonitor()
	panel.apply()

	if len(app.hostCommands) != 1 || app.hostCommands[0].kind != hostCommandReboot {
		t.Fatalf("expected queued reboot, got %+v", app.hostCommands)
	}
	if got := app.hostCommands[0].cfg.FloppyB; got != "/tmp/game.stx" {
		t.Fatalf("applied FloppyB = %q, want /tmp/game.stx", got)
	}
}

func TestConfigPanelCyclePresetUpdatesMachineFields(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	seen := map[string]bool{}
	for range len(config.MachinePresets) + 1 {
		panel.cyclePreset()
		seen[config.MatchPreset(&panel.work)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("cycling presets did not change configuration: %v", seen)
	}
}

func TestConfigPanelProfileSaveLoadRoundTrip(t *testing.T) {
	t.Setenv(config.ConfigDirEnv, t.TempDir())
	app := newTestApp(t)
	panel := newConfigPanel(app, configModeLauncher, app.cfg)

	panel.cycleRAM()
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
		w, h := panel.container.PreferredSize()
		if w > configWindowWidth {
			t.Errorf("mode %d: panel width %d exceeds window width %d", mode, w, configWindowWidth)
		}
		if h > configWindowHeight {
			t.Errorf("mode %d: panel height %d exceeds window height %d", mode, h, configWindowHeight)
		}
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

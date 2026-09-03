package ebiten

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/emulator"
	"github.com/jenska/gost/internal/platform/host"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	machine, err := emulator.NewMachine(config.DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	return &App{machine: machine}
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
	app := &App{}

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

func TestFloppyPanelRefreshesMountedPathsAndErrors(t *testing.T) {
	app := &App{
		mountedFloppies: [2]string{
			"/tmp/disk-a.stx",
			"",
		},
		lastHostCommandError: "load drive A disk: missing",
	}
	panel := newFloppyPanel(app)

	panel.Refresh()

	if got := panel.pathInputs[0].GetText(); got != "/tmp/disk-a.stx" {
		t.Fatalf("drive A input text = %q, want mounted path", got)
	}
	if got := panel.mounted[0].Label; got != "Mounted: disk-a.stx" {
		t.Fatalf("drive A mounted label = %q", got)
	}
	if got := panel.mounted[1].Label; got != "No disk mounted" {
		t.Fatalf("drive B mounted label = %q", got)
	}
	if got := panel.status.Label; got != app.lastHostCommandError {
		t.Fatalf("status label = %q, want %q", got, app.lastHostCommandError)
	}
}

func TestFloppyPanelQueuesMountAndEject(t *testing.T) {
	app := &App{}
	panel := newFloppyPanel(app)
	panel.pathInputs[0].SetText("/tmp/disk-a.st")
	panel.pathInputs[1].SetText("/tmp/disk-b.stx")

	panel.QueueMount(0)
	panel.QueueEject(1)

	if len(app.hostCommands) != 2 {
		t.Fatalf("queued command count = %d, want 2", len(app.hostCommands))
	}
	if command := app.hostCommands[0]; command.kind != hostCommandMountFloppy || command.drive != 0 || command.path != "/tmp/disk-a.st" {
		t.Fatalf("unexpected mount command: %+v", command)
	}
	if command := app.hostCommands[1]; command.kind != hostCommandEjectFloppy || command.drive != 1 {
		t.Fatalf("unexpected eject command: %+v", command)
	}
	if got := panel.pathInputs[1].GetText(); got != "" {
		t.Fatalf("drive B input after eject = %q, want empty", got)
	}
}

func TestFloppyPanelBrowseSetsPathInput(t *testing.T) {
	app := &App{
		fileSelector: func() (string, error) {
			return "/tmp/selected.stx", nil
		},
	}
	panel := newFloppyPanel(app)

	panel.QueueBrowse(0)
	waitForFileDialogResult(t, app)
	panel.Refresh()

	if got := panel.pathInputs[0].GetText(); got != "/tmp/selected.stx" {
		t.Fatalf("drive A input text = %q, want selected path", got)
	}
	if got := app.LastHostCommandError(); got != "" {
		t.Fatalf("unexpected browse error: %s", got)
	}
	if got := len(app.hostCommands); got != 0 {
		t.Fatalf("browse queued %d host commands, want none", got)
	}
}

func TestFloppyPanelBrowseRecordsDialogErrors(t *testing.T) {
	app := &App{
		fileSelector: func() (string, error) {
			return "", errors.New("dialog unavailable")
		},
	}
	panel := newFloppyPanel(app)

	panel.QueueBrowse(1)
	waitForFileDialogResult(t, app)
	panel.Refresh()

	if got := app.LastHostCommandError(); got != "dialog unavailable" {
		t.Fatalf("host command error = %q, want dialog error", got)
	}
	if got := panel.status.Label; got != "dialog unavailable" {
		t.Fatalf("status label = %q, want dialog error", got)
	}
}

func TestFloppyPanelBrowseCancelClearsErrors(t *testing.T) {
	app := &App{
		lastHostCommandError: "old error",
		fileSelector: func() (string, error) {
			return "", host.ErrFileDialogCanceled
		},
	}
	panel := newFloppyPanel(app)

	panel.QueueBrowse(0)
	waitForFileDialogResult(t, app)
	panel.Refresh()

	if got := app.LastHostCommandError(); got != "" {
		t.Fatalf("host command error after cancel = %q, want empty", got)
	}
	if got := panel.status.Label; got != "F12 closes this panel." {
		t.Fatalf("status label after cancel = %q", got)
	}
}

func waitForFileDialogResult(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		app.applyFileDialogResults()
		if !app.fileDialogPending[0] && !app.fileDialogPending[1] {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for file dialog result")
}

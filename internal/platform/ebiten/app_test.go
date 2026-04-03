package ebiten

import "testing"

func TestResetMouseTrackingCentersGuestCursor(t *testing.T) {
	app := &App{}
	app.resetMouseTracking(640, 400)

	if app.guestMouseX != 320 || app.guestMouseY != 200 {
		t.Fatalf("unexpected guest mouse center: got (%d,%d) want (320,200)", app.guestMouseX, app.guestMouseY)
	}
	if app.mouseReady {
		t.Fatalf("expected mouseReady to be false after reset")
	}
	if app.mousePrimed {
		t.Fatalf("expected mousePrimed to be false after reset")
	}
	if app.mouseSyncing != 0 {
		t.Fatalf("expected mouseSyncing to be reset")
	}
	if app.mouseStable != 0 {
		t.Fatalf("expected mouseStable to be reset")
	}
	if app.mouseInside {
		t.Fatalf("expected mouseInside to be false after reset")
	}
	if app.cursorHidden {
		t.Fatalf("expected cursorHidden to be false after reset")
	}
}

func TestResetMouseTrackingUsesCurrentResolution(t *testing.T) {
	app := &App{
		guestMouseX:  99,
		guestMouseY:  77,
		lastButtons:  0x03,
		mousePrimed:  true,
		mouseReady:   true,
		mouseInside:  true,
		cursorHidden: true,
	}

	app.resetMouseTracking(320, 200)

	if app.guestMouseX != 160 || app.guestMouseY != 100 {
		t.Fatalf("unexpected guest mouse center after resize: got (%d,%d) want (160,100)", app.guestMouseX, app.guestMouseY)
	}
	if app.lastButtons != 0 {
		t.Fatalf("unexpected lastButtons after reset: got %02x want 00", app.lastButtons)
	}
	if app.mouseReady {
		t.Fatalf("expected mouseReady to be false after resize reset")
	}
	if app.mousePrimed {
		t.Fatalf("expected mousePrimed to be reset after resize")
	}
	if app.mouseSyncing != 0 {
		t.Fatalf("expected mouseSyncing to be reset after resize")
	}
	if app.mouseStable != 0 {
		t.Fatalf("expected mouseStable to be reset after resize")
	}
	if app.mouseInside {
		t.Fatalf("expected mouseInside to be false after resize reset")
	}
	if app.cursorHidden {
		t.Fatalf("expected cursorHidden to be false after resize reset")
	}
}

func TestResetMouseTrackingClearsCursorState(t *testing.T) {
	app := &App{
		mouseInside:  true,
		cursorHidden: true,
	}

	app.resetMouseTracking(640, 400)

	if app.mouseInside {
		t.Fatalf("expected mouseInside to be false after reset")
	}
	if app.cursorHidden {
		t.Fatalf("expected cursorHidden to be false after reset")
	}
}

func TestGuestTargetPositionMatchesLogicalCursorCoordinates(t *testing.T) {
	x, y := guestTargetPosition(100, 100, 640, 400)
	if x != 100 || y != 100 {
		t.Fatalf("unexpected guest target: got (%d,%d) want (100,100)", x, y)
	}
}

func TestScaledWindowSize(t *testing.T) {
	width, height := scaledWindowSize(320, 200, 2.5)
	if width != 800 || height != 500 {
		t.Fatalf("unexpected scaled window size: got (%d,%d) want (800,500)", width, height)
	}
}

func TestClampMouseSyncDelta(t *testing.T) {
	if got := clampMouseSyncDelta(100); got != maxMouseSyncStep {
		t.Fatalf("unexpected positive sync clamp: got %d want %d", got, maxMouseSyncStep)
	}
	if got := clampMouseSyncDelta(-100); got != -maxMouseSyncStep {
		t.Fatalf("unexpected negative sync clamp: got %d want %d", got, -maxMouseSyncStep)
	}
	if got := clampMouseSyncDelta(12); got != 12 {
		t.Fatalf("unexpected unclamped sync delta: got %d want 12", got)
	}
}

func TestGuestTargetPositionClampsToScreen(t *testing.T) {
	x, y := guestTargetPosition(-2, -3, 640, 400)
	if x != 0 || y != 0 {
		t.Fatalf("unexpected clamped target at origin: got (%d,%d) want (0,0)", x, y)
	}

	x, y = guestTargetPosition(999, 999, 640, 400)
	if x != 639 || y != 399 {
		t.Fatalf("unexpected clamped target at edge: got (%d,%d) want (639,399)", x, y)
	}
}

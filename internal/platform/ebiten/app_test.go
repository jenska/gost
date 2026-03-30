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

func TestGuestTargetPositionCompensatesHotspotOffset(t *testing.T) {
	x, y := guestTargetPosition(100, 100, 640, 400)
	if x != 96 || y != 95 {
		t.Fatalf("unexpected compensated target: got (%d,%d) want (96,95)", x, y)
	}
}

func TestGuestTargetPositionClampsToScreen(t *testing.T) {
	x, y := guestTargetPosition(2, 3, 640, 400)
	if x != 0 || y != 0 {
		t.Fatalf("unexpected clamped target at origin: got (%d,%d) want (0,0)", x, y)
	}

	x, y = guestTargetPosition(999, 999, 640, 400)
	if x != 639 || y != 399 {
		t.Fatalf("unexpected clamped target at edge: got (%d,%d) want (639,399)", x, y)
	}
}

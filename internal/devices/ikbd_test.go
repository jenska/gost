package devices

import (
	"testing"
	"time"
)

func TestIKBDResetCommandQueuesVersionByte(t *testing.T) {
	ikbd := NewIKBD()

	ikbd.HandleCommand(0x80)
	if ikbd.HasData() {
		t.Fatalf("reset prefix should wait for the trailing byte")
	}

	ikbd.HandleCommand(0x01)
	if !ikbd.HasData() {
		t.Fatalf("expected reset command to queue a version byte")
	}

	value, err := ikbd.ReadByte()
	if err != nil {
		t.Fatalf("expected queued version byte")
	}
	if value != 0xF1 {
		t.Fatalf("unexpected reset response: got %02x want f1", value)
	}
}

func TestIKBDMouseCanBeDisabledAndReenabled(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	ikbd.PushMouse(4, -2, 0x02)
	if !ikbd.HasData() {
		t.Fatalf("expected default mouse reporting to queue a packet")
	}
	for ikbd.HasData() {
		if _, err := ikbd.ReadByte(); err != nil {
			t.Fatalf("drain default mouse packet: %v", err)
		}
	}

	ikbd.HandleCommand(0x12)
	ikbd.PushMouse(3, 1, 0x01)
	if ikbd.HasData() {
		t.Fatalf("expected disabled mouse reporting to suppress packets")
	}

	ikbd.HandleCommand(0x08)
	ikbd.PushMouse(3, 1, 0x01)
	if !ikbd.HasData() {
		t.Fatalf("expected relative mouse mode to reenable packets")
	}
}

func TestIKBDPauseAndResumeOutput(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	ikbd.HandleCommand(0x13)
	ikbd.PushMouse(2, 0, 0x00)
	if ikbd.HasData() {
		t.Fatalf("expected paused output to suppress mouse packets")
	}

	ikbd.HandleCommand(0x11)
	ikbd.PushMouse(2, 0, 0x00)
	if !ikbd.HasData() {
		t.Fatalf("expected resumed output to queue mouse packets")
	}
}

func TestIKBDSplitsLargeMouseDeltas(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	ikbd.PushMouse(200, -200, 0x00)

	want := []byte{
		0xF8, 0x7F, 0x80,
		0xF8, 0x49, 0xB8,
	}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read mouse byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected mouse byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func TestIKBDAbsoluteMouseInterrogationReportsPosition(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	for _, cmd := range []byte{0x09, 0x02, 0x7F, 0x01, 0x8F} {
		ikbd.HandleCommand(cmd)
	}
	for _, cmd := range []byte{0x0C, 0x01, 0x01} {
		ikbd.HandleCommand(cmd)
	}

	ikbd.PushMouse(12, 7, 0x00)
	ikbd.HandleCommand(0x0D)

	want := []byte{0xF7, 0x00, 0x00, 0x0C, 0x00, 0x07}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read absolute mouse byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected absolute mouse byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func TestIKBDLoadMousePositionSetsAbsoluteCoordinates(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	for _, cmd := range []byte{0x09, 0x01, 0x00, 0x01, 0x00} {
		ikbd.HandleCommand(cmd)
	}
	for _, cmd := range []byte{0x0E, 0x00, 0x00, 0x20, 0x00, 0x40} {
		ikbd.HandleCommand(cmd)
	}
	ikbd.HandleCommand(0x0D)

	want := []byte{0xF7, 0x00, 0x00, 0x20, 0x00, 0x40}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read loaded absolute mouse byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected loaded absolute mouse byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func TestIKBDPushKeyQueuesExtendedMouseScancodes(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	ikbd.PushKey(0x37, true)
	ikbd.PushKey(0x37, false)
	ikbd.PushKey(0x59, true)

	want := []byte{0x37, 0xB7, 0x59}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read extended mouse byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected extended mouse byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func TestIKBDLongCommandDoesNotOverflowParserBuffer(t *testing.T) {
	ikbd := NewIKBD()
	ikbd.Reset()

	for _, cmd := range []byte{0x19, 1, 2, 3, 4, 5, 6} {
		ikbd.HandleCommand(cmd)
	}

	if ikbd.commandAt != 0 || ikbd.remaining != 0 {
		t.Fatalf("expected parser to reset after a long command, got commandAt=%d remaining=%d", ikbd.commandAt, ikbd.remaining)
	}
}

func TestIKBDInterrogateClockReturnsCurrentPackedBCDTime(t *testing.T) {
	now := time.Date(2026, time.April, 3, 21, 45, 18, 0, time.Local)
	ikbd := newTestIKBD(now)
	ikbd.Reset()

	ikbd.HandleCommand(0x1C)

	want := []byte{0xFC, 0x26, 0x04, 0x03, 0x21, 0x45, 0x18}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read clock byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected clock byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func TestIKBDSetClockUpdatesInterrogatedTime(t *testing.T) {
	now := time.Date(2026, time.April, 3, 21, 45, 18, 0, time.Local)
	ikbd := newTestIKBD(now)
	ikbd.Reset()

	for _, cmd := range []byte{0x1B, 0x24, 0x12, 0x31, 0x23, 0x59, 0x58} {
		ikbd.HandleCommand(cmd)
	}
	ikbd.HandleCommand(0x1C)

	want := []byte{0xFC, 0x24, 0x12, 0x31, 0x23, 0x59, 0x58}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read set clock byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected set clock byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func TestIKBDSetClockIgnoresInvalidBCDFields(t *testing.T) {
	now := time.Date(2026, time.April, 3, 21, 45, 18, 0, time.Local)
	ikbd := newTestIKBD(now)
	ikbd.Reset()

	for _, cmd := range []byte{0x1B, 0x99, 0x1A, 0x04, 0x22, 0x77, 0x30} {
		ikbd.HandleCommand(cmd)
	}
	ikbd.HandleCommand(0x1C)

	want := []byte{0xFC, 0x99, 0x04, 0x04, 0x22, 0x45, 0x30}
	for idx, expected := range want {
		got, err := ikbd.ReadByte()
		if err != nil {
			t.Fatalf("read partial clock byte %d: %v", idx, err)
		}
		if got != expected {
			t.Fatalf("unexpected partial clock byte %d: got %02x want %02x", idx, got, expected)
		}
	}
}

func newTestIKBD(now time.Time) *IKBD {
	ikbd := NewIKBD()
	ikbd.now = func() time.Time { return now }
	return ikbd
}

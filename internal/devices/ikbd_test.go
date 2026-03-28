package devices

import "testing"

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

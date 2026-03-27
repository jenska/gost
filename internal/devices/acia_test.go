package devices

import "testing"

func TestACIAReceivesIKBDBytes(t *testing.T) {
	ikbd := NewIKBD()
	acia := NewACIA(ikbd)

	ikbd.PushKey(0x1E, true)
	acia.Advance(0)

	status, err := acia.Read(1, aciaBase)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status&0x01 == 0 {
		t.Fatalf("expected receive-ready status bit, got %02x", status)
	}

	data, err := acia.Read(1, aciaBase+2)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if data != 0x1E {
		t.Fatalf("unexpected data byte: got %02x want 1e", data)
	}
}

package devices

import "testing"

func TestACIAReceivesIKBDBytes(t *testing.T) {
	ikbd := NewIKBD()
	acia := NewACIA(ikbd)

	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

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

func TestACIAMIDIChannelStaysIndependent(t *testing.T) {
	ikbd := NewIKBD()
	acia := NewACIA(ikbd)

	if err := acia.Write(1, aciaBase+4, 0x03); err != nil {
		t.Fatalf("reset MIDI channel: %v", err)
	}

	status, err := acia.Read(1, aciaBase+4)
	if err != nil {
		t.Fatalf("read MIDI status: %v", err)
	}
	if status != 0x02 {
		t.Fatalf("unexpected MIDI status: got %02x want 02", status)
	}

	keyboardStatus, err := acia.Read(1, aciaBase)
	if err != nil {
		t.Fatalf("read keyboard status: %v", err)
	}
	if keyboardStatus != 0x02 {
		t.Fatalf("unexpected keyboard status after MIDI reset: got %02x want 02", keyboardStatus)
	}
}

func TestACIAKeyboardDoesNotQueueDirectCPUInterrupt(t *testing.T) {
	ikbd := NewIKBD()
	acia := NewACIA(ikbd)

	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

	ikbd.PushKey(0x1E, true)
	acia.Advance(0)

	if irqs := acia.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected no direct CPU interrupts from ACIA, got %d", len(irqs))
	}
}

package devices

import "testing"

func TestACIAReceivesIKBDBytes(t *testing.T) {
	acia := NewACIA(nil)

	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

	acia.PushKey(0x1E, true)
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
	acia := NewACIA(nil)

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
	acia := NewACIA(nil)

	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

	acia.PushKey(0x1E, true)
	acia.Advance(0)

	if irqs := acia.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected no direct CPU interrupts from ACIA, got %d", len(irqs))
	}
}

func TestACIAKeyboardSignalsMFPInterruptOnReceive(t *testing.T) {
	mfp := NewMFP(8_000_000)
	acia := NewACIA(mfp.SetACIAInterrupt)

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x40); err != nil {
		t.Fatalf("enable acia interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x40); err != nil {
		t.Fatalf("mask acia interrupt: %v", err)
	}
	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

	acia.PushKey(0x1E, true)
	acia.Advance(0)

	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected one MFP interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x46 {
		t.Fatalf("unexpected ACIA MFP vector: %+v", irqs[0].Vector)
	}

	if _, err := acia.Read(1, aciaBase+2); err != nil {
		t.Fatalf("read keyboard data: %v", err)
	}

	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected interrupt to clear after data read, got %d", len(irqs))
	}
}

func TestACIAStaggersQueuedMouseBytesAcrossAdvances(t *testing.T) {
	acia := NewACIA(nil)

	acia.PushMouse(4, 2, 0)
	acia.Advance(0)

	status, err := acia.Read(1, aciaBase)
	if err != nil {
		t.Fatalf("read initial status: %v", err)
	}
	if status&0x01 == 0 {
		t.Fatalf("expected first mouse byte to be ready, got %02x", status)
	}

	first, err := acia.Read(1, aciaBase+2)
	if err != nil {
		t.Fatalf("read first mouse byte: %v", err)
	}
	if first != 0xF8 {
		t.Fatalf("unexpected first mouse byte: got %02x want f8", first)
	}

	status, err = acia.Read(1, aciaBase)
	if err != nil {
		t.Fatalf("read status after draining first byte: %v", err)
	}
	if status&0x01 != 0 {
		t.Fatalf("expected second mouse byte to wait for the next advance, got %02x", status)
	}

	acia.Advance(0)

	second, err := acia.Read(1, aciaBase+2)
	if err != nil {
		t.Fatalf("read second mouse byte: %v", err)
	}
	if second != 0x04 {
		t.Fatalf("unexpected second mouse byte: got %02x want 04", second)
	}
}

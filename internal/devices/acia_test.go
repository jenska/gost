package devices

import (
	"testing"

	"github.com/jenska/gost/internal/config"
)

func TestACIAReceivesIKBDBytes(t *testing.T) {
	acia := NewACIA(nil)

	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

	acia.PushKey(0x1E, true)
	acia.Advance(aciaIKBDByteCycles)

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

func TestACIAMIDITransmitCapturesOutput(t *testing.T) {
	acia := NewACIA(nil)

	if err := acia.Write(1, aciaBase+6, 0x90); err != nil {
		t.Fatalf("write MIDI data: %v", err)
	}

	output := acia.midi.Output()
	if len(output) != 1 || output[0] != 0x90 {
		t.Fatalf("MIDI output = %x, want 90", output)
	}
}

func TestACIAMIDIReceiveByte(t *testing.T) {
	acia := NewACIA(nil)

	acia.PushMIDIInput([]byte{0x90})
	acia.Advance(aciaMIDIByteCycles)

	status, err := acia.Read(1, aciaBase+4)
	if err != nil {
		t.Fatalf("read MIDI status: %v", err)
	}
	if status&0x01 == 0 {
		t.Fatalf("expected MIDI receive-ready status bit, got %02x", status)
	}

	data, err := acia.Read(1, aciaBase+6)
	if err != nil {
		t.Fatalf("read MIDI data: %v", err)
	}
	if data != 0x90 {
		t.Fatalf("unexpected MIDI data byte: got %02x want 90", data)
	}
}

func TestACIAMIDIStaggersQueuedBytesAcrossAdvances(t *testing.T) {
	acia := NewACIA(nil)

	acia.PushMIDIInput([]byte{0x90, 0x40})
	acia.Advance(aciaMIDIByteCycles)

	first, err := acia.Read(1, aciaBase+6)
	if err != nil {
		t.Fatalf("read first MIDI byte: %v", err)
	}
	if first != 0x90 {
		t.Fatalf("first MIDI byte = %02x, want 90", first)
	}

	status, err := acia.Read(1, aciaBase+4)
	if err != nil {
		t.Fatalf("read MIDI status after first byte: %v", err)
	}
	if status&0x01 != 0 {
		t.Fatalf("expected second MIDI byte to still be on the wire, got %02x", status)
	}

	acia.Advance(aciaMIDIByteCycles)
	second, err := acia.Read(1, aciaBase+6)
	if err != nil {
		t.Fatalf("read second MIDI byte: %v", err)
	}
	if second != 0x40 {
		t.Fatalf("second MIDI byte = %02x, want 40", second)
	}
}

func TestACIAKeyboardDoesNotDriveCPUInterruptLine(t *testing.T) {
	acia := NewACIA(nil)

	if err := acia.Write(1, aciaBase, 0x95); err != nil {
		t.Fatalf("enable keyboard RX interrupts: %v", err)
	}

	acia.PushKey(0x1E, true)
	acia.Advance(aciaIKBDByteCycles)

	// The ACIA has no interrupt line of its own; it signals the CPU only by
	// driving MFP GPIP i4 through the SetACIAInterrupt callback.
	if _, drivesLine := any(acia).(interface{ PendingIRQ() (uint8, uint8) }); drivesLine {
		t.Fatal("ACIA must route interrupts through the MFP, not drive the CPU line directly")
	}
}

func TestACIAKeyboardSignalsMFPInterruptOnReceive(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})
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
	acia.Advance(aciaIKBDByteCycles)

	irqs := drainIRQ(mfp)
	if len(irqs) != 1 {
		t.Fatalf("expected one MFP interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector != 0x46 {
		t.Fatalf("unexpected ACIA MFP vector: %+v", irqs[0].Vector)
	}

	if _, err := acia.Read(1, aciaBase+2); err != nil {
		t.Fatalf("read keyboard data: %v", err)
	}

	if irqs := drainIRQ(mfp); len(irqs) != 0 {
		t.Fatalf("expected interrupt to clear after data read, got %d", len(irqs))
	}
}

func TestACIAMIDISignalsMFPInterruptOnReceive(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})
	acia := NewACIA(mfp.SetACIAInterrupt)

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x40); err != nil {
		t.Fatalf("enable ACIA interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x40); err != nil {
		t.Fatalf("mask ACIA interrupt: %v", err)
	}
	if err := acia.Write(1, aciaBase+4, 0x95); err != nil {
		t.Fatalf("enable MIDI RX interrupts: %v", err)
	}

	acia.PushMIDIInput([]byte{0x90})
	acia.Advance(aciaMIDIByteCycles)

	irqs := drainIRQ(mfp)
	if len(irqs) != 1 {
		t.Fatalf("expected one MFP interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector != 0x46 {
		t.Fatalf("unexpected ACIA MFP vector: %+v", irqs[0].Vector)
	}

	if _, err := acia.Read(1, aciaBase+6); err != nil {
		t.Fatalf("read MIDI data: %v", err)
	}
	if irqs := drainIRQ(mfp); len(irqs) != 0 {
		t.Fatalf("expected interrupt to clear after MIDI data read, got %d", len(irqs))
	}
}

func TestACIAStaggersQueuedMouseBytesAcrossAdvances(t *testing.T) {
	acia := NewACIA(nil)

	acia.PushMouse(4, 2, 0)
	acia.Advance(aciaIKBDByteCycles)

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
		t.Fatalf("expected second mouse byte to still be on the wire, got %02x", status)
	}

	acia.Advance(aciaIKBDByteCycles)

	second, err := acia.Read(1, aciaBase+2)
	if err != nil {
		t.Fatalf("read second mouse byte: %v", err)
	}
	if second != 0x04 {
		t.Fatalf("unexpected second mouse byte: got %02x want 04", second)
	}
}

// The keyboard link runs at 7812.5 baud: each 10-bit byte takes 1.28 ms, so a
// byte is not ready one cycle early and the next follows back to back.
func TestACIAIKBDBytesArriveAtLineRate(t *testing.T) {
	acia := NewACIA(nil)
	acia.PushMouse(4, 2, 0) // three-byte packet

	if got, ok := acia.NextEventCycles(); ok {
		t.Fatalf("no byte should be in flight before the first advance, got %d", got)
	}
	acia.Advance(aciaIKBDByteCycles - 1)
	if status, _ := acia.Read(1, aciaBase); status&0x01 != 0 {
		t.Fatalf("byte ready one cycle early: status %02x", status)
	}
	if got, ok := acia.NextEventCycles(); !ok || got != 1 {
		t.Fatalf("NextEventCycles = %d, %v; want 1, true", got, ok)
	}
	acia.Advance(1)
	if data, _ := acia.Read(1, aciaBase+2); data != 0xF8 {
		t.Fatalf("first byte = %02x, want f8", data)
	}

	// A late reader finds the second byte held and the third arriving behind it.
	acia.Advance(2*aciaIKBDByteCycles + 100)
	if data, _ := acia.Read(1, aciaBase+2); data != 0x04 {
		t.Fatalf("second byte = %02x, want 04", data)
	}
	if status, _ := acia.Read(1, aciaBase); status&0x01 == 0 {
		t.Fatalf("third byte, received while the register was full, should load on read: status %02x", status)
	}
	if data, _ := acia.Read(1, aciaBase+2); data != 0x02 {
		t.Fatalf("third byte = %02x, want 02", data)
	}
}

func TestACIAMIDIBytesArriveAtLineRate(t *testing.T) {
	acia := NewACIA(nil)
	acia.PushMIDIInput([]byte{0x90})

	acia.Advance(aciaMIDIByteCycles - 1)
	if status, _ := acia.Read(1, aciaBase+4); status&0x01 != 0 {
		t.Fatalf("MIDI byte ready one cycle early: status %02x", status)
	}
	acia.Advance(1)
	if status, _ := acia.Read(1, aciaBase+4); status&0x01 == 0 {
		t.Fatalf("MIDI byte not ready after %d cycles: status %02x", aciaMIDIByteCycles, status)
	}
}

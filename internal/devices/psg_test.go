package devices

import "testing"

func TestPSGReadWriteTracksSelectedRegister(t *testing.T) {
	psg := NewPSG(8_000_000)

	if err := psg.Write(0, psgBase, 8); err != nil {
		t.Fatalf("select register: %v", err)
	}
	if err := psg.Write(0, psgBase+2, 0x0F); err != nil {
		t.Fatalf("write data: %v", err)
	}

	value, err := psg.Read(0, psgBase+2)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if value != 0x0F {
		t.Fatalf("unexpected register value: got %02x want 0f", value)
	}
}

func TestPSGAdvanceGeneratesSamples(t *testing.T) {
	psg := NewPSG(8_000_000)
	writePSGRegister(t, psg, 0, 0x10)
	writePSGRegister(t, psg, 1, 0x00)
	writePSGRegister(t, psg, 7, 0x3E)
	writePSGRegister(t, psg, 8, 0x0F)

	psg.Advance(40_000)

	if psg.OutputSampleRate() != psgSampleRate {
		t.Fatalf("unexpected sample rate: got %d want %d", psg.OutputSampleRate(), psgSampleRate)
	}

	samples := make([]float32, 1024)
	n := psg.DrainMonoF32(samples)
	if n == 0 {
		t.Fatalf("expected generated samples after advancing PSG")
	}

	var nonZero bool
	for i := 0; i < n; i++ {
		if samples[i] != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected audible non-zero samples")
	}
}

func TestPSGEmuTOSKeyclickSequenceGeneratesSamples(t *testing.T) {
	psg := NewPSG(8_000_000)
	writePSGRegister(t, psg, 0, 0x3B)
	writePSGRegister(t, psg, 1, 0x00)
	writePSGRegister(t, psg, 2, 0x00)
	writePSGRegister(t, psg, 3, 0x00)
	writePSGRegister(t, psg, 4, 0x00)
	writePSGRegister(t, psg, 5, 0x00)
	writePSGRegister(t, psg, 6, 0x00)
	writePSGRegister(t, psg, 7, 0xFE)
	writePSGRegister(t, psg, 8, 0x10)
	writePSGRegister(t, psg, 13, 0x03)
	writePSGRegister(t, psg, 11, 0x80)
	writePSGRegister(t, psg, 12, 0x01)

	psg.Advance(40_000)

	samples := make([]float32, 2048)
	n := psg.DrainMonoF32(samples)
	if n == 0 {
		t.Fatalf("expected generated samples after EmuTOS keyclick sequence")
	}

	var nonZero bool
	for i := 0; i < n; i++ {
		if samples[i] != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected EmuTOS keyclick sequence to generate audible samples")
	}
}

func TestPSGNotifiesPortAObservers(t *testing.T) {
	psg := NewPSG(8_000_000)
	var values []byte
	psg.SetPortAObserver(func(value byte) {
		values = append(values, value)
	})

	writePSGRegister(t, psg, 14, 0x05)

	if len(values) == 0 {
		t.Fatalf("expected at least one port A notification")
	}
	if values[len(values)-1] != 0x05 {
		t.Fatalf("unexpected final port A value: got %02x want 05", values[len(values)-1])
	}
}

func writePSGRegister(t *testing.T, psg *PSG, reg, value byte) {
	t.Helper()
	if err := psg.Write(0, psgBase, uint32(reg)); err != nil {
		t.Fatalf("select register %d: %v", reg, err)
	}
	if err := psg.Write(0, psgBase+2, uint32(value)); err != nil {
		t.Fatalf("write register %d: %v", reg, err)
	}
}

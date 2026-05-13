package devices

import "testing"

func TestMIDIInputFIFO(t *testing.T) {
	midi := NewMIDI()
	midi.PushInput([]byte{0x90, 0x40})

	first, ok := midi.PopInput()
	if !ok || first != 0x90 {
		t.Fatalf("first input = %02x, %v; want 90, true", first, ok)
	}
	second, ok := midi.PopInput()
	if !ok || second != 0x40 {
		t.Fatalf("second input = %02x, %v; want 40, true", second, ok)
	}
	if _, ok := midi.PopInput(); ok {
		t.Fatalf("expected empty input FIFO")
	}
}

func TestMIDIOutputReturnsCopy(t *testing.T) {
	midi := NewMIDI()
	midi.WriteOutput(0x90)

	output := midi.Output()
	output[0] = 0x00

	if got, want := midi.Output()[0], byte(0x90); got != want {
		t.Fatalf("output alias changed data: got %02x want %02x", got, want)
	}
}

func TestMIDIClearOutput(t *testing.T) {
	midi := NewMIDI()
	midi.WriteOutput(0x90)
	midi.ClearOutput()

	if output := midi.Output(); len(output) != 0 {
		t.Fatalf("expected cleared output, got %x", output)
	}
}

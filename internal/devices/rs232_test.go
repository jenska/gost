package devices

import "testing"

func TestRS232InputFIFO(t *testing.T) {
	rs232 := NewRS232()
	rs232.PushInput([]byte("AB"))

	first, ok := rs232.PopInput()
	if !ok || first != 'A' {
		t.Fatalf("first input = %q, %v; want A, true", first, ok)
	}
	second, ok := rs232.PopInput()
	if !ok || second != 'B' {
		t.Fatalf("second input = %q, %v; want B, true", second, ok)
	}
	if _, ok := rs232.PopInput(); ok {
		t.Fatalf("expected empty input FIFO")
	}
}

func TestRS232OutputReturnsCopy(t *testing.T) {
	rs232 := NewRS232()
	rs232.WriteOutput('C')

	output := rs232.Output()
	output[0] = 'X'

	if got, want := string(rs232.Output()), "C"; got != want {
		t.Fatalf("output alias changed data: got %q want %q", got, want)
	}
}

func TestRS232ClearOutput(t *testing.T) {
	rs232 := NewRS232()
	rs232.WriteOutput('D')
	rs232.ClearOutput()

	if output := rs232.Output(); len(output) != 0 {
		t.Fatalf("expected cleared output, got %q", string(output))
	}
}

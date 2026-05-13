package devices

import "testing"

func TestPrinterPortCapturesDataOnFallingStrobe(t *testing.T) {
	printer := NewPrinterPort()

	printer.SetData('A')
	printer.SetStrobe(true)
	printer.SetStrobe(false)
	printer.SetStrobe(true)

	output := printer.Output()
	if got, want := string(output), "A"; got != want {
		t.Fatalf("printer output = %q, want %q", got, want)
	}
}

func TestPrinterPortIgnoresStrobeWhileBusy(t *testing.T) {
	printer := NewPrinterPort()
	printer.SetBusy(true)

	printer.SetData('B')
	printer.SetStrobe(false)
	printer.SetStrobe(true)

	if output := printer.Output(); len(output) != 0 {
		t.Fatalf("expected busy printer to ignore output, got %q", string(output))
	}
}

func TestPrinterPortOutputReturnsCopy(t *testing.T) {
	printer := NewPrinterPort()
	printer.SetData('C')
	printer.SetStrobe(true)
	printer.SetStrobe(false)

	output := printer.Output()
	output[0] = 'X'

	if got, want := string(printer.Output()), "C"; got != want {
		t.Fatalf("printer output alias changed data: got %q want %q", got, want)
	}
}

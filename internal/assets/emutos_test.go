package assets

import "testing"

func TestDefaultROMIsPresent(t *testing.T) {
	rom := DefaultROM()
	if len(rom) != 256*1024 {
		t.Fatalf("unexpected default ROM size: got %d want %d", len(rom), 256*1024)
	}
}

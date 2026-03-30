package inputmap

import "testing"

func TestAtariScancodeIncludesDesktopKeys(t *testing.T) {
	tests := []struct {
		key  Key
		want byte
	}{
		{KeyShiftLeft, 0x2A},
		{KeyShiftRight, 0x36},
		{KeyControlLeft, 0x1D},
		{KeyAltLeft, 0x38},
		{KeyMinus, 0x0C},
		{KeyEqual, 0x0D},
		{KeyBracketLeft, 0x1A},
		{KeyBracketRight, 0x1B},
		{KeySemicolon, 0x27},
		{KeyQuote, 0x28},
		{KeyComma, 0x33},
		{KeyPeriod, 0x34},
		{KeySlash, 0x35},
		{KeyBackslash, 0x2B},
		{KeyBackquote, 0x29},
		{KeyDelete, 0x53},
		{KeyInsert, 0x52},
		{KeyHome, 0x47},
		{KeyF1, 0x3B},
		{KeyF10, 0x44},
	}

	for _, tt := range tests {
		got, ok := AtariScancode(tt.key)
		if !ok {
			t.Fatalf("expected mapping for key %v", tt.key)
		}
		if got != tt.want {
			t.Fatalf("unexpected mapping for key %v: got %02x want %02x", tt.key, got, tt.want)
		}
	}
}

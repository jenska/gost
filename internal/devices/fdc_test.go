package devices

import "testing"

func TestFDCReadSector(t *testing.T) {
	fdc := NewFDC()
	image := make([]byte, fdcSectorSize*fdcSectorsTrack)
	copy(image[:fdcSectorSize], []byte{0xDE, 0xAD, 0xBE, 0xEF})

	if err := fdc.InsertDisk(image); err != nil {
		t.Fatalf("insert disk: %v", err)
	}
	if err := fdc.Write(1, fdcBase+4, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(1, fdcBase, 0x80); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	for i, want := range []byte{0xDE, 0xAD, 0xBE, 0xEF} {
		got, err := fdc.Read(1, fdcBase+6)
		if err != nil {
			t.Fatalf("read data %d: %v", i, err)
		}
		if byte(got) != want {
			t.Fatalf("byte %d mismatch: got %02x want %02x", i, got, want)
		}
	}
}

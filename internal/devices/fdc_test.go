package devices

import (
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func TestFDCDMAReadSectorIntoRAM(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, fdcSectorSize*fdcSectorsTrack)
	copy(image[:fdcSectorSize], []byte{0xDE, 0xAD, 0xBE, 0xEF})

	if err := fdc.InsertDisk(image); err != nil {
		t.Fatalf("insert disk: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x01); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaSCReg|dmaDRQFloppy); err != nil {
		t.Fatalf("select sector-count register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector count: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1); err != nil {
		t.Fatalf("select sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRead); err != nil {
		t.Fatalf("execute read command: %v", err)
	}

	for i, want := range []byte{0xDE, 0xAD, 0xBE, 0xEF} {
		got, err := ram.Read(cpu.Byte, 0x100+uint32(i))
		if err != nil {
			t.Fatalf("read dma byte %d: %v", i, err)
		}
		if byte(got) != want {
			t.Fatalf("byte %d mismatch: got %02x want %02x", i, got, want)
		}
	}

	status, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetControl)
	if err != nil {
		t.Fatalf("read dma status: %v", err)
	}
	if status&dmaStatusOK == 0 {
		t.Fatalf("expected DMA OK status, got %04x", status)
	}
}

func TestFDCDMAReadAdvancesAddress(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, fdcSectorSize*fdcSectorsTrack*2)
	copy(image[:fdcSectorSize], []byte{0xAA, 0xBB, 0xCC, 0xDD})
	copy(image[fdcSectorSize:fdcSectorSize+4], []byte{0x11, 0x22, 0x33, 0x44})

	if err := fdc.InsertDisk(image); err != nil {
		t.Fatalf("insert disk: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x20); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaSCReg|dmaDRQFloppy); err != nil {
		t.Fatalf("select sector-count register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 2); err != nil {
		t.Fatalf("write sector count: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1); err != nil {
		t.Fatalf("select sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRead|0x10); err != nil {
		t.Fatalf("execute multi-read command: %v", err)
	}

	if got, err := fdc.Read(cpu.Byte, fdcBase+fdcOffsetAddrMed); err != nil {
		t.Fatalf("read dma addr med: %v", err)
	} else if byte(got) != 0x24 {
		t.Fatalf("dma address medium byte = %02x, want 24", got)
	}
	if got, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetControl); err != nil {
		t.Fatalf("read dma status: %v", err)
	} else if got&dmaStatusSCNot0 != 0 {
		t.Fatalf("expected sector count to reach zero, got status %04x", got)
	}
	if fdc.sector != 3 {
		t.Fatalf("expected sector register to advance to 3, got %d", fdc.sector)
	}
}

func TestFDCSeekCommandUpdatesTrackRegister(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1|dmaA0); err != nil {
		t.Fatalf("select data register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 3); err != nil {
		t.Fatalf("write seek target: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdSeek); err != nil {
		t.Fatalf("execute seek: %v", err)
	}

	if fdc.track != 3 {
		t.Fatalf("expected seek to update track register to 3, got %d", fdc.track)
	}

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRestore); err != nil {
		t.Fatalf("execute restore: %v", err)
	}
	if fdc.track != 0 {
		t.Fatalf("expected restore to reset track register, got %d", fdc.track)
	}
}

func TestFDCStatusReadClearsInterruptLine(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	var line []bool
	fdc := NewFDC(ram, func(asserted bool) {
		line = append(line, asserted)
	})

	image := make([]byte, fdcSectorSize*fdcSectorsTrack)
	if err := fdc.InsertDisk(image); err != nil {
		t.Fatalf("insert disk: %v", err)
	}

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1); err != nil {
		t.Fatalf("select sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaSCReg|dmaDRQFloppy); err != nil {
		t.Fatalf("select sector-count register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector count: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRead); err != nil {
		t.Fatalf("execute read command: %v", err)
	}

	if len(line) == 0 || !line[len(line)-1] {
		t.Fatalf("expected FDC completion to assert the interrupt line, got %v", line)
	}

	if _, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetData); err != nil {
		t.Fatalf("read status register: %v", err)
	}

	if len(line) < 2 || line[len(line)-1] {
		t.Fatalf("expected status read to clear the interrupt line, got %v", line)
	}
}

func TestFDCReadStatusDoesNotExposeTrackZeroAsLostData(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, fdcSectorSize*fdcSectorsTrack)
	if err := fdc.InsertDisk(image); err != nil {
		t.Fatalf("insert disk: %v", err)
	}

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1); err != nil {
		t.Fatalf("select sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaSCReg|dmaDRQFloppy); err != nil {
		t.Fatalf("select sector-count register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector count: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRead); err != nil {
		t.Fatalf("execute read command: %v", err)
	}

	status, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetData)
	if err != nil {
		t.Fatalf("read status register: %v", err)
	}
	if byte(status)&fdcStatusTrack0 != 0 {
		t.Fatalf("expected read-sector status to keep bit 2 clear, got %02x", byte(status))
	}
}

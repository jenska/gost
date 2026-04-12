package devices

import (
	"encoding/binary"
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
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, fdcStatusBusy); err != nil {
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

func TestFDCReadsSelectedSideFromDiskGeometry(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, 2*fdcSectorSize)
	copy(image[:4], []byte{0x10, 0x20, 0x30, 0x40})
	copy(image[fdcSectorSize:fdcSectorSize+4], []byte{0xAA, 0xBB, 0xCC, 0xDD})

	if err := fdc.InsertDiskWithGeometry(image, 1, 2, 1); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
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

	fdc.SetDriveControl(0x05) // drive A selected, side 0
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRead); err != nil {
		t.Fatalf("execute side 0 read: %v", err)
	}
	if value, err := ram.Read(cpu.Word, 0x100); err != nil {
		t.Fatalf("read side 0 payload: %v", err)
	} else if uint16(value) != 0x1020 {
		t.Fatalf("unexpected side 0 payload: got %04x want 1020", value)
	}

	fdc.SetDriveControl(0x04) // drive A selected, side 1
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdRead); err != nil {
		t.Fatalf("execute side 1 read: %v", err)
	}
	if value, err := ram.Read(cpu.Word, 0x300); err != nil {
		t.Fatalf("read side 1 payload: %v", err)
	} else if uint16(value) != 0xAABB {
		t.Fatalf("unexpected side 1 payload: got %04x want aabb", value)
	}
}

func TestFDCWriteSectorCopiesRAMIntoDisk(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, fdcSectorSize)
	if err := fdc.InsertDiskWithGeometry(image, 1, 1, 1); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
	}
	if err := ram.LoadAt(0x400, []byte{0xCA, 0xFE, 0xBA, 0xBE}); err != nil {
		t.Fatalf("seed dma buffer: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x04); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaSCReg|dmaDRQFloppy|dmaWriteBit); err != nil {
		t.Fatalf("select sector-count register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector count: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1|dmaWriteBit); err != nil {
		t.Fatalf("select sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaWriteBit); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdWrite); err != nil {
		t.Fatalf("execute write command: %v", err)
	}

	if got := fdc.diskA[:4]; got[0] != 0xCA || got[1] != 0xFE || got[2] != 0xBA || got[3] != 0xBE {
		t.Fatalf("unexpected disk bytes after write: % x", got)
	}
}

func TestFDCStepCommandsUpdateHeadAndTrackRegister(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)
	if err := fdc.InsertDiskWithGeometry(make([]byte, 80*9*512), 9, 1, 80); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
	}

	fdc.track = 10
	fdc.headTrack = 10

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdStepIn|fdcCmdFlagUpdateTrack); err != nil {
		t.Fatalf("execute step-in: %v", err)
	}
	if fdc.track != 11 || fdc.headTrack != 11 {
		t.Fatalf("unexpected step-in position: track=%d head=%d", fdc.track, fdc.headTrack)
	}

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdStepOut); err != nil {
		t.Fatalf("execute step-out: %v", err)
	}
	if fdc.headTrack != 10 {
		t.Fatalf("expected head track 10 after step-out, got %d", fdc.headTrack)
	}
	if fdc.track != 11 {
		t.Fatalf("expected track register unchanged without update bit, got %d", fdc.track)
	}
}

func TestFDCReadAddressTransfersIDField(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	if err := fdc.InsertDiskWithGeometry(make([]byte, 9*2*512), 9, 2, 1); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
	}
	fdc.track = 3
	fdc.sector = 5
	fdc.SetDriveControl(0x04) // drive A, side 1

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x02); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdReadAddr); err != nil {
		t.Fatalf("execute read address: %v", err)
	}

	want := []byte{3, 1, 5, 2, 0, 0}
	for i, w := range want {
		got, err := ram.Read(cpu.Byte, 0x200+uint32(i))
		if err != nil {
			t.Fatalf("read ID byte %d: %v", i, err)
		}
		if byte(got) != w {
			t.Fatalf("ID byte %d = %02x, want %02x", i, got, w)
		}
	}
}

func TestFDCReadTrackTransfersEntireTrack(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, 2*fdcSectorSize)
	for i := range fdcSectorSize {
		image[i] = 0x11
		image[fdcSectorSize+i] = 0x22
	}
	if err := fdc.InsertDiskWithGeometry(image, 2, 1, 1); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x03); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdReadTrack); err != nil {
		t.Fatalf("execute read track: %v", err)
	}

	if value, err := ram.Read(cpu.Byte, 0x300); err != nil {
		t.Fatalf("read first track byte: %v", err)
	} else if byte(value) != 0x11 {
		t.Fatalf("first track byte = %02x, want 11", value)
	}
	if value, err := ram.Read(cpu.Byte, 0x300+fdcSectorSize); err != nil {
		t.Fatalf("read second sector byte: %v", err)
	} else if byte(value) != 0x22 {
		t.Fatalf("second sector byte = %02x, want 22", value)
	}
}

func TestFDCWriteTrackCopiesEntireTrackFromRAM(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, 2*fdcSectorSize)
	if err := fdc.InsertDiskWithGeometry(image, 2, 1, 1); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
	}

	payload := make([]byte, 2*fdcSectorSize)
	for i := range payload {
		if i < fdcSectorSize {
			payload[i] = 0x44
		} else {
			payload[i] = 0x88
		}
	}
	if err := ram.LoadAt(0x500, payload); err != nil {
		t.Fatalf("seed DMA payload: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x05); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaWriteBit); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdWriteTrack); err != nil {
		t.Fatalf("execute write track: %v", err)
	}

	if fdc.diskA[0] != 0x44 || fdc.diskA[fdcSectorSize] != 0x88 {
		t.Fatalf("unexpected track bytes after write-track: %02x %02x", fdc.diskA[0], fdc.diskA[fdcSectorSize])
	}
}

func TestFDCWriteProtectBlocksWriteCommands(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, fdcSectorSize)
	if err := fdc.InsertDiskWithGeometry(image, 1, 1, 1); err != nil {
		t.Fatalf("insert disk with geometry: %v", err)
	}
	fdc.SetDiskWriteProtected(true)
	if err := ram.LoadAt(0x600, []byte{0x12, 0x34}); err != nil {
		t.Fatalf("seed DMA payload: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write dma addr high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x06); err != nil {
		t.Fatalf("write dma addr med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write dma addr low: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaSCReg|dmaDRQFloppy|dmaWriteBit); err != nil {
		t.Fatalf("select sector count register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector count: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaA1|dmaWriteBit); err != nil {
		t.Fatalf("select sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, 1); err != nil {
		t.Fatalf("write sector register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaDRQFloppy|dmaWriteBit); err != nil {
		t.Fatalf("select command register: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, fdcCmdWrite); err != nil {
		t.Fatalf("execute write-sector: %v", err)
	}

	if fdc.status&fdcStatusWriteProtect == 0 {
		t.Fatalf("expected write-protect status bit after blocked write, status=%02x", fdc.status)
	}
}

func TestFDCAcsiReadAndWriteSectors(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, 2*fdcSectorSize)
	copy(image[:4], []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if err := fdc.SetHardDiskImage(image); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}

	// READ(6) LBA 0 count 1 -> RAM @ 0x0700
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write DMA high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x07); err != nil {
		t.Fatalf("write DMA med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write DMA low: %v", err)
	}
	if status := sendACSICommand(t, fdc, []byte{0x08, 0x00, 0x00, 0x00, 0x01, 0x00}); status != acsiStatusGood {
		t.Fatalf("READ(6) status = %02x, want 00", status)
	}
	for i, want := range []byte{0xDE, 0xAD, 0xBE, 0xEF} {
		got, err := ram.Read(cpu.Byte, 0x0700+uint32(i))
		if err != nil {
			t.Fatalf("read DMA byte %d: %v", i, err)
		}
		if byte(got) != want {
			t.Fatalf("DMA byte %d = %02x, want %02x", i, got, want)
		}
	}

	// WRITE(6) LBA 1 count 1 <- RAM @ 0x0800
	if err := ram.LoadAt(0x0800, []byte{0x11, 0x22, 0x33, 0x44}); err != nil {
		t.Fatalf("seed write payload: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write DMA high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, fdcStatusCRC); err != nil {
		t.Fatalf("write DMA med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write DMA low: %v", err)
	}
	if status := sendACSICommand(t, fdc, []byte{0x0A, 0x00, 0x00, fdcStatusBusy, fdcStatusBusy, 0x00}); status != acsiStatusGood {
		t.Fatalf("WRITE(6) status = %02x, want 00", status)
	}
	if got := fdc.hardDisk0[fdcSectorSize : fdcSectorSize+4]; got[0] != 0x11 || got[1] != 0x22 || got[2] != 0x33 || got[3] != 0x44 {
		t.Fatalf("unexpected hard-disk bytes after write: % x", got)
	}
}

func TestFDCAcsiOutOfRangeReadReportsSense(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)
	if err := fdc.SetHardDiskImage(make([]byte, fdcSectorSize)); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}

	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write DMA high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x09); err != nil {
		t.Fatalf("write DMA med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write DMA low: %v", err)
	}
	if status := sendACSICommand(t, fdc, []byte{0x08, 0x00, 0x00, 0x10, 0x01, 0x00}); status != acsiStatusCheckCondition {
		t.Fatalf("out-of-range READ(6) status = %02x, want 02", status)
	}

	if status := sendACSICommand(t, fdc, []byte{0x03, 0x00, 0x00, 0x00, 0x04, 0x00}); status != acsiStatusGood {
		t.Fatalf("REQUEST SENSE status = %02x, want 00", status)
	}
	if got, err := ram.Read(cpu.Byte, 0x0902); err != nil {
		t.Fatalf("read sense key: %v", err)
	} else if byte(got) != acsiSenseIllegalReq {
		t.Fatalf("sense key = %02x, want %02x", got, acsiSenseIllegalReq)
	}
}

func TestFDCAcsiWriteProtectBlocksWrites(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)
	if err := fdc.SetHardDiskImage(make([]byte, fdcSectorSize)); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}
	fdc.SetHardDiskWriteProtected(true)

	if err := ram.LoadAt(0x0A00, []byte{0x55, 0x66, 0x77, 0x88}); err != nil {
		t.Fatalf("seed write payload: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write DMA high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x0A); err != nil {
		t.Fatalf("write DMA med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write DMA low: %v", err)
	}
	if status := sendACSICommand(t, fdc, []byte{0x0A, 0x00, 0x00, 0x00, 0x01, 0x00}); status != acsiStatusCheckCondition {
		t.Fatalf("write-protected WRITE(6) status = %02x, want 02", status)
	}
	if fdc.hardDisk0[0] != 0 {
		t.Fatalf("expected write-protected media to remain unchanged")
	}
}

func TestFDCAcsiDatacontrolStyleWritesSupportMultiByteCommand(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)
	if err := fdc.SetHardDiskImage(make([]byte, fdcSectorSize)); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}

	// Device 1 should fail TUR, which verifies that all command bytes were
	// consumed even when DMA_NOT_NEWCDB is set between bytes.
	if status := sendACSICommandDatacontrolStyle(t, fdc, []byte{0x20, 0x00, 0x00, 0x00, 0x00, 0x00}); status != acsiStatusCheckCondition {
		t.Fatalf("TUR(target=1) status = %02x, want %02x", status, acsiStatusCheckCondition)
	}
}

func TestFDCAcsiICDExtendedReadCapacity10(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)
	if err := fdc.SetHardDiskImage(make([]byte, 2*fdcSectorSize)); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write DMA high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x09); err != nil {
		t.Fatalf("write DMA med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write DMA low: %v", err)
	}

	cmd := []byte{0x1F, 0x25, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if status := sendACSICommandDatacontrolStyle(t, fdc, cmd); status != acsiStatusGood {
		t.Fatalf("ICD READ CAPACITY status = %02x (sense=%02x), want %02x", status, fdc.acsiSense, acsiStatusGood)
	}

	buf := make([]byte, 8)
	if err := ram.CopyOut(0x0900, buf); err != nil {
		t.Fatalf("copy READ CAPACITY payload: %v", err)
	}
	if got := binary.BigEndian.Uint32(buf[0:4]); got != 1 {
		t.Fatalf("READ CAPACITY last LBA = %d, want 1", got)
	}
	if got := binary.BigEndian.Uint32(buf[4:8]); got != fdcSectorSize {
		t.Fatalf("READ CAPACITY block length = %d, want %d", got, fdcSectorSize)
	}
}

func TestFDCAcsiICDExtendedRead10(t *testing.T) {
	ram := NewRAM(0, 1024*1024)
	fdc := NewFDC(ram, nil)

	image := make([]byte, 3*fdcSectorSize)
	copy(image[fdcSectorSize:fdcSectorSize+4], []byte{0xCA, 0xFE, 0xBA, 0xBE})
	if err := fdc.SetHardDiskImage(image); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrHigh, 0x00); err != nil {
		t.Fatalf("write DMA high: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrMed, 0x0B); err != nil {
		t.Fatalf("write DMA med: %v", err)
	}
	if err := fdc.Write(cpu.Byte, fdcBase+fdcOffsetAddrLow, 0x00); err != nil {
		t.Fatalf("write DMA low: %v", err)
	}

	// ICD wrapper (0x1F) + READ(10) CDB for LBA=1, transfer length=1.
	cmd := []byte{0x1F, 0x28, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00}
	if status := sendACSICommandDatacontrolStyle(t, fdc, cmd); status != acsiStatusGood {
		t.Fatalf("ICD READ(10) status = %02x (sense=%02x), want %02x", status, fdc.acsiSense, acsiStatusGood)
	}

	for i, want := range []byte{0xCA, 0xFE, 0xBA, 0xBE} {
		got, err := ram.Read(cpu.Byte, 0x0B00+uint32(i))
		if err != nil {
			t.Fatalf("read DMA byte %d: %v", i, err)
		}
		if byte(got) != want {
			t.Fatalf("DMA byte %d = %02x, want %02x", i, got, want)
		}
	}
}

func TestFDCCreateVirtualHardDiskInitializesAtariPartitionAndFAT16(t *testing.T) {
	fdc := NewFDC(NewRAM(0, 1024*1024), nil)
	if err := fdc.CreateVirtualHardDisk(30 * 1024 * 1024); err != nil {
		t.Fatalf("create virtual hard disk: %v", err)
	}

	image := fdc.hardDisk0
	totalSectors := len(image) / fdcSectorSize
	part0 := image[ahdiRootSectorPrimaryPart0Offset : ahdiRootSectorPrimaryPart0Offset+ahdiPartitionEntrySize]

	if got := int(binary.BigEndian.Uint32(image[ahdiRootSectorDiskSizeOffset : ahdiRootSectorDiskSizeOffset+4])); got != totalSectors {
		t.Fatalf("root sector disk size = %d, want %d", got, totalSectors)
	}
	if part0[0]&0x01 == 0 {
		t.Fatalf("partition 0 is not marked active")
	}
	if got := string(part0[1:4]); got != "BGM" {
		t.Fatalf("partition 0 id = %q, want BGM", got)
	}
	if got := int(binary.BigEndian.Uint32(part0[4:8])); got != 1 {
		t.Fatalf("partition 0 start = %d, want 1", got)
	}
	if got, want := int(binary.BigEndian.Uint32(part0[8:12])), totalSectors-1; got != want {
		t.Fatalf("partition 0 size = %d, want %d", got, want)
	}
	if got := binary.BigEndian.Uint16(image[ahdiRootSectorChecksumOffset : ahdiRootSectorChecksumOffset+2]); got == dosMBRSignature {
		t.Fatalf("root sector checksum must not look like DOS MBR signature")
	}

	boot := image[fdcSectorSize : 2*fdcSectorSize]
	if boot[0] != 0x60 {
		t.Fatalf("boot sector branch opcode = %02x, want 60", boot[0])
	}
	if got := binary.LittleEndian.Uint16(boot[0x0B:0x0D]); got != fdcSectorSize {
		t.Fatalf("boot bytes/sector = %d, want %d", got, fdcSectorSize)
	}
	if got := int(boot[0x0D]); got < 2 || got > 64 || got&(got-1) != 0 {
		t.Fatalf("boot sectors/cluster = %d, want power-of-two 2..64", got)
	}
	if got := boot[0x10]; got != fat16DefaultFATCount {
		t.Fatalf("boot FAT count = %d, want %d", got, fat16DefaultFATCount)
	}
	if got := binary.LittleEndian.Uint16(boot[0x11:0x13]); got != fat16DefaultRootEntries {
		t.Fatalf("boot root entries = %d, want %d", got, fat16DefaultRootEntries)
	}
	if got := binary.LittleEndian.Uint32(boot[0x1C:0x20]); got != 1 {
		t.Fatalf("boot hidden sectors = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint16(boot[0x16:0x18]); got == 0 {
		t.Fatalf("boot sectors/FAT should be non-zero")
	}
	if got := string(boot[0x36 : 0x36+8]); got != "FAT16   " {
		t.Fatalf("boot filesystem type = %q, want %q", got, "FAT16   ")
	}

	reserved := int(binary.LittleEndian.Uint16(boot[0x0E:0x10]))
	sectorsPerFAT := int(binary.LittleEndian.Uint16(boot[0x16:0x18]))
	fatStart := fdcSectorSize + reserved*fdcSectorSize
	fat := image[fatStart : fatStart+4]
	if fat[0] != fat16MediaDescriptorFixedDisk || fat[1] != 0xFF || fat[2] != 0xFF || fat[3] != 0xFF {
		t.Fatalf("unexpected FAT[0] signature bytes: % x", fat)
	}
	if sectorsPerFAT <= 0 {
		t.Fatalf("invalid sectors/FAT")
	}
}

func TestFDCHardDiskImageReturnsCopy(t *testing.T) {
	fdc := NewFDC(NewRAM(0, 1024*1024), nil)
	if err := fdc.SetHardDiskImage(make([]byte, fdcSectorSize)); err != nil {
		t.Fatalf("set hard disk image: %v", err)
	}
	fdc.hardDisk0[0] = 0x12

	image := fdc.HardDiskImage()
	if len(image) != fdcSectorSize {
		t.Fatalf("image size = %d, want %d", len(image), fdcSectorSize)
	}
	image[0] = 0x34

	if fdc.hardDisk0[0] != 0x12 {
		t.Fatalf("hard disk image accessor should return a copy")
	}
}

func sendACSICommand(t *testing.T, fdc *FDC, cmd []byte) byte {
	t.Helper()
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaCSACSI); err != nil {
		t.Fatalf("select ACSI command register: %v", err)
	}
	for i, b := range cmd {
		if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, uint32(b)); err != nil {
			t.Fatalf("write ACSI command byte %d: %v", i, err)
		}
	}
	status, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetData)
	if err != nil {
		t.Fatalf("read ACSI status: %v", err)
	}
	return byte(status)
}

func sendACSICommandDatacontrolStyle(t *testing.T, fdc *FDC, cmd []byte) byte {
	t.Helper()
	control := uint16(dmaCSACSI)
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, uint32(control)); err != nil {
		t.Fatalf("select ACSI command register: %v", err)
	}

	for i, b := range cmd {
		nextControl := control | dmaA0 // DMA_NOT_NEWCDB in ACSI mode
		if i == len(cmd)-1 {
			nextControl = control &^ dmaA0
		}
		value := uint32(b)<<16 | uint32(nextControl)
		if err := fdc.Write(cpu.Long, fdcBase+fdcOffsetData, value); err != nil {
			t.Fatalf("write ACSI datacontrol byte %d: %v", i, err)
		}
		control = nextControl
	}

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, uint32(control&^dmaWriteBit)); err != nil {
		t.Fatalf("switch ACSI to status phase: %v", err)
	}
	status, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetData)
	if err != nil {
		t.Fatalf("read ACSI status: %v", err)
	}
	return byte(status)
}

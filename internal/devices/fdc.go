package devices

import (
	"encoding/binary"
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

const (
	fdcBase = 0xFF8600
	fdcSize = 0x10

	fdcSectorSize   = 512
	fdcSectorsTrack = 9

	dmaA0        = 0x0002
	dmaA1        = 0x0004
	dmaCSACSI    = 0x0008
	dmaSCReg     = 0x0010
	dmaDRQFloppy = 0x0080
	dmaWriteBit  = 0x0100

	dmaStatusOK     = 0x0001
	dmaStatusSCNot0 = 0x0002
	dmaStatusDataRQ = 0x0004

	fdcCmdRestore    = 0x00
	fdcCmdSeek       = 0x10
	fdcCmdStep       = 0x20
	fdcCmdStepIn     = 0x40
	fdcCmdStepOut    = 0x60
	fdcCmdRead       = 0x80
	fdcCmdWrite      = 0xA0
	fdcCmdReadAddr   = 0xC0
	fdcCmdForceI     = 0xD0
	fdcCmdReadTrack  = 0xE0
	fdcCmdWriteTrack = 0xF0

	fdcCmdFlagMultiSector = 0x10
	fdcCmdFlagUpdateTrack = 0x10
	fdcCmdFlagVerify      = 0x04

	fdcStatusBusy         = 0x01
	fdcStatusDataRQ       = 0x02
	fdcStatusTrack0       = 0x04
	fdcStatusLostData     = 0x04
	fdcStatusCRC          = 0x08
	fdcStatusRNF          = 0x10
	fdcStatusWriteProtect = 0x40
	fdcStatusMotorOn      = 0x80

	fdcOffsetData     = 0x04
	fdcOffsetControl  = 0x06
	fdcOffsetAddrHigh = 0x09
	fdcOffsetAddrMed  = 0x0B
	fdcOffsetAddrLow  = 0x0D
	fdcOffsetModeCtl  = 0x0F

	acsiStatusGood           = 0x00
	acsiStatusCheckCondition = 0x02

	acsiSenseNone          = 0x00
	acsiSenseNotReady      = 0x02
	acsiSenseIllegalReq    = 0x05
	acsiSenseWriteProtect  = 0x07
	acsiSenseMediumError   = 0x03
	acsiDefaultCommandSize = 6

	ahdiRootSectorDiskSizeOffset      = 0x1C2
	ahdiRootSectorPrimaryPart0Offset  = 0x1C6
	ahdiRootSectorChecksumOffset      = 0x1FE
	ahdiPartitionEntrySize            = 12
	ahdiPartitionMinSizeForBGM        = 16 * 1024 * 1024 / fdcSectorSize
	fat16MaxClusters                  = 65524
	fat12MaxClusters                  = 4084
	fat16DefaultReservedSectors       = 1
	fat16DefaultFATCount              = 2
	fat16DefaultRootEntries           = 512
	fat16MediaDescriptorFixedDisk     = 0xF8
	fat16DefaultSectorsPerTrack       = 63
	fat16DefaultSides                 = 16
	nonExecutableSectorChecksumTarget = 0x1235
	dosMBRSignature                   = 0x55AA
)

// FDC models the ST's DMA + WD1772 register window using sector-based disk
// images. It supports the full WD1772 command groups (type I/II/III/IV) within
// that sector-image abstraction.
type FDC struct {
	ram         *RAM
	irq         func(bool)
	control     uint16
	sectorCount uint16
	dmaAddr     uint32
	modeCtl     byte
	dmaData     uint16

	status byte
	track  byte
	sector byte
	data   byte
	typeI  bool

	// track/head state
	headTrack         int
	lastStepDirection int // +1=in, -1=out

	// drive A image and geometry
	diskA               []byte
	diskAWriteProtected bool
	sectorsPerTrack     int
	sides               int
	tracks              int

	selectedDrive int
	selectedSide  int

	pending []Interrupt
	vector  uint8
	dmaOK   bool

	hardDisk0               []byte
	hardDisk0WriteProtected bool
	acsiStatus              byte
	acsiSense               byte
	acsiCmdBuf              [12]byte
	acsiCmdLen              int
	acsiExpectedLen         int
	acsiLastNotNewCDB       bool
}

func NewFDC(ram *RAM, irq func(bool)) *FDC {
	f := &FDC{ram: ram, irq: irq, vector: 0x46}
	f.Reset()
	return f
}

func (f *FDC) Contains(address uint32) bool {
	return address >= fdcBase && address < fdcBase+fdcSize
}

func (f *FDC) WaitStates(cpu.Size, uint32) uint32 {
	return 8
}

func (f *FDC) Reset() {
	f.control = 0
	f.sectorCount = 0
	f.dmaAddr = 0
	f.modeCtl = 0
	f.dmaData = 0
	f.track = 0
	f.sector = 1
	f.data = 0
	f.typeI = true
	f.headTrack = 0
	f.lastStepDirection = -1
	f.selectedDrive = 0
	f.selectedSide = 0
	f.pending = f.pending[:0]
	f.dmaOK = true
	f.acsiStatus = acsiStatusGood
	f.acsiSense = acsiSenseNone
	f.acsiCmdLen = 0
	f.acsiExpectedLen = 0
	f.acsiLastNotNewCDB = false
	f.status = f.baseStatus()
	if f.irq != nil {
		f.irq(false)
	}
}

func (f *FDC) InsertDisk(image []byte) error {
	sectorsPerTrack, sides, tracks := inferFloppyGeometry(len(image))
	return f.InsertDiskWithGeometry(image, sectorsPerTrack, sides, tracks)
}

func (f *FDC) InsertDiskWithGeometry(image []byte, sectorsPerTrack, sides, tracks int) error {
	if len(image)%fdcSectorSize != 0 {
		return fmt.Errorf("disk image size %d is not a multiple of %d", len(image), fdcSectorSize)
	}
	if sectorsPerTrack <= 0 || sides <= 0 {
		return fmt.Errorf("invalid disk geometry: sectors/track=%d sides=%d", sectorsPerTrack, sides)
	}
	if tracks <= 0 {
		totalSectors := len(image) / fdcSectorSize
		if totalSectors%(sectorsPerTrack*sides) != 0 {
			return fmt.Errorf("disk image size %d does not match geometry %d/%d/%d",
				len(image), tracks, sides, sectorsPerTrack)
		}
		tracks = totalSectors / (sectorsPerTrack * sides)
	}
	if tracks*sectorsPerTrack*sides*fdcSectorSize != len(image) {
		return fmt.Errorf("disk image size %d does not match geometry %d/%d/%d",
			len(image), tracks, sides, sectorsPerTrack)
	}
	f.diskA = append([]byte(nil), image...)
	f.diskAWriteProtected = false
	f.sectorsPerTrack = sectorsPerTrack
	f.sides = sides
	f.tracks = tracks
	f.status = f.baseStatus()
	return nil
}

func (f *FDC) SetDiskWriteProtected(writeProtected bool) {
	f.diskAWriteProtected = writeProtected
	f.status = f.baseStatus()
}

func (f *FDC) SetHardDiskImage(image []byte) error {
	if len(image)%fdcSectorSize != 0 {
		return fmt.Errorf("hard disk image size %d is not a multiple of %d", len(image), fdcSectorSize)
	}
	f.hardDisk0 = append([]byte(nil), image...)
	return nil
}

func (f *FDC) CreateVirtualHardDisk(sizeBytes int) error {
	if sizeBytes < 0 {
		return fmt.Errorf("hard disk size cannot be negative")
	}
	if sizeBytes == 0 {
		f.hardDisk0 = nil
		return nil
	}
	if sizeBytes%fdcSectorSize != 0 {
		return fmt.Errorf("hard disk size %d is not a multiple of %d", sizeBytes, fdcSectorSize)
	}
	f.hardDisk0 = make([]byte, sizeBytes)
	initializeVirtualHardDisk(f.hardDisk0)
	return nil
}

func (f *FDC) SetHardDiskWriteProtected(writeProtected bool) {
	f.hardDisk0WriteProtected = writeProtected
}

func (f *FDC) HardDiskSizeBytes() int {
	return len(f.hardDisk0)
}

func (f *FDC) HardDiskImage() []byte {
	if len(f.hardDisk0) == 0 {
		return nil
	}
	return append([]byte(nil), f.hardDisk0...)
}

func initializeVirtualHardDisk(image []byte) {
	if len(image) < 2*fdcSectorSize || len(image)%fdcSectorSize != 0 {
		return
	}

	totalSectors := len(image) / fdcSectorSize
	partStart := 1
	partSectors := totalSectors - partStart
	if partSectors <= 0 {
		return
	}

	writeAHDIRootSector(image[:fdcSectorSize], totalSectors, partStart, partSectors)
	formatFAT16Partition(image[partStart*fdcSectorSize:], partSectors, partStart)
}

func writeAHDIRootSector(sector []byte, totalSectors, partStart, partSectors int) {
	if len(sector) < fdcSectorSize {
		return
	}

	binary.BigEndian.PutUint32(sector[ahdiRootSectorDiskSizeOffset:ahdiRootSectorDiskSizeOffset+4], uint32(totalSectors))

	entry := sector[ahdiRootSectorPrimaryPart0Offset : ahdiRootSectorPrimaryPart0Offset+ahdiPartitionEntrySize]
	entry[0] = 0x01 // active
	if partSectors >= ahdiPartitionMinSizeForBGM {
		copy(entry[1:4], []byte("BGM"))
	} else {
		copy(entry[1:4], []byte("GEM"))
	}
	binary.BigEndian.PutUint32(entry[4:8], uint32(partStart))
	binary.BigEndian.PutUint32(entry[8:12], uint32(partSectors))

	setNonExecutableSectorChecksum(sector)
	if binary.BigEndian.Uint16(sector[ahdiRootSectorChecksumOffset:ahdiRootSectorChecksumOffset+2]) == dosMBRSignature {
		setSectorChecksum(sector, nonExecutableSectorChecksumTarget+1)
	}
}

func formatFAT16Partition(partition []byte, partitionSectors, hiddenSectors int) {
	if len(partition) < fdcSectorSize || partitionSectors <= 0 {
		return
	}

	layout, ok := pickFAT16Layout(partitionSectors)
	if !ok {
		return
	}

	boot := partition[:fdcSectorSize]
	boot[0] = 0x60 // BRA.S over BPB/metadata
	boot[1] = 0x1C
	copy(boot[2:8], []byte("GOSTHD"))
	copy(boot[8:11], []byte{0x47, 0x4F, 0x53})

	binary.LittleEndian.PutUint16(boot[0x0B:0x0D], fdcSectorSize)
	boot[0x0D] = byte(layout.sectorsPerCluster)
	binary.LittleEndian.PutUint16(boot[0x0E:0x10], fat16DefaultReservedSectors)
	boot[0x10] = fat16DefaultFATCount
	binary.LittleEndian.PutUint16(boot[0x11:0x13], fat16DefaultRootEntries)
	if partitionSectors <= 0xFFFF {
		binary.LittleEndian.PutUint16(boot[0x13:0x15], uint16(partitionSectors))
		binary.LittleEndian.PutUint32(boot[0x20:0x24], 0)
	} else {
		binary.LittleEndian.PutUint16(boot[0x13:0x15], 0)
		binary.LittleEndian.PutUint32(boot[0x20:0x24], uint32(partitionSectors))
	}
	boot[0x15] = fat16MediaDescriptorFixedDisk
	binary.LittleEndian.PutUint16(boot[0x16:0x18], uint16(layout.sectorsPerFAT))
	binary.LittleEndian.PutUint16(boot[0x18:0x1A], fat16DefaultSectorsPerTrack)
	binary.LittleEndian.PutUint16(boot[0x1A:0x1C], fat16DefaultSides)
	binary.LittleEndian.PutUint32(boot[0x1C:0x20], uint32(hiddenSectors))
	boot[0x24] = 0
	boot[0x25] = 0
	boot[0x26] = 0x29
	copy(boot[0x27:0x2B], []byte{0xC0, 0xDE, 0x30, 0x00})
	copyAt(boot, 0x2B, []byte("GOSTHD     "))
	copyAt(boot, 0x36, []byte("FAT16   "))
	setNonExecutableSectorChecksum(boot)

	fatOffset := fat16DefaultReservedSectors * fdcSectorSize
	fatBytes := layout.sectorsPerFAT * fdcSectorSize
	for fatIndex := 0; fatIndex < fat16DefaultFATCount; fatIndex++ {
		start := fatOffset + fatIndex*fatBytes
		end := start + fatBytes
		if end > len(partition) {
			return
		}
		fat := partition[start:end]
		fat[0] = fat16MediaDescriptorFixedDisk
		fat[1] = 0xFF
		fat[2] = 0xFF
		fat[3] = 0xFF
	}

	rootDirSectors := (fat16DefaultRootEntries*32 + fdcSectorSize - 1) / fdcSectorSize
	rootStart := fatOffset + fat16DefaultFATCount*fatBytes
	rootEnd := rootStart + rootDirSectors*fdcSectorSize
	if rootEnd <= len(partition) {
		root := partition[rootStart:rootEnd]
		copy(root[:11], []byte("GOSTHD     "))
		root[11] = 0x08 // volume label
	}
}

type fat16Layout struct {
	sectorsPerCluster int
	sectorsPerFAT     int
}

func pickFAT16Layout(totalSectors int) (fat16Layout, bool) {
	rootDirSectors := (fat16DefaultRootEntries*32 + fdcSectorSize - 1) / fdcSectorSize
	for _, sectorsPerCluster := range []int{2, 4, 8, 16, 32, 64} {
		sectorsPerFAT, clusters, ok := fat16SectorsPerFAT(totalSectors, sectorsPerCluster, rootDirSectors)
		if !ok {
			continue
		}
		if clusters <= fat12MaxClusters || clusters > fat16MaxClusters {
			continue
		}
		return fat16Layout{sectorsPerCluster: sectorsPerCluster, sectorsPerFAT: sectorsPerFAT}, true
	}
	return fat16Layout{}, false
}

func fat16SectorsPerFAT(totalSectors, sectorsPerCluster, rootDirSectors int) (int, int, bool) {
	if totalSectors <= 0 || sectorsPerCluster <= 0 {
		return 0, 0, false
	}

	sectorsPerFAT := 1
	for i := 0; i < 32; i++ {
		dataSectors := totalSectors - fat16DefaultReservedSectors - rootDirSectors - fat16DefaultFATCount*sectorsPerFAT
		if dataSectors <= 0 {
			return 0, 0, false
		}

		clusters := dataSectors / sectorsPerCluster
		if clusters <= 0 {
			return 0, 0, false
		}

		neededFATBytes := (clusters + 2) * 2
		neededSectors := (neededFATBytes + fdcSectorSize - 1) / fdcSectorSize
		if neededSectors == sectorsPerFAT {
			return sectorsPerFAT, clusters, true
		}
		sectorsPerFAT = neededSectors
	}

	return 0, 0, false
}

func setNonExecutableSectorChecksum(sector []byte) {
	setSectorChecksum(sector, nonExecutableSectorChecksumTarget)
}

func setSectorChecksum(sector []byte, target uint16) {
	if len(sector) < fdcSectorSize {
		return
	}
	binary.BigEndian.PutUint16(sector[ahdiRootSectorChecksumOffset:ahdiRootSectorChecksumOffset+2], 0)
	sum := sectorWordChecksum(sector)
	checksum := uint16(uint32(target-sum) & 0xFFFF)
	binary.BigEndian.PutUint16(sector[ahdiRootSectorChecksumOffset:ahdiRootSectorChecksumOffset+2], checksum)
}

func sectorWordChecksum(sector []byte) uint16 {
	sum := uint32(0)
	for i := 0; i+1 < fdcSectorSize; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(sector[i : i+2]))
	}
	return uint16(sum & 0xFFFF)
}

func (f *FDC) Read(size cpu.Size, address uint32) (uint32, error) {
	offset := address - fdcBase
	switch size {
	case cpu.Byte:
		return uint32(f.readByte(offset)), nil
	case cpu.Word:
		return uint32(f.readWord(offset)), nil
	case cpu.Long:
		hi := f.readWord(offset)
		lo := f.readWord(offset + 2)
		return uint32(hi)<<16 | uint32(lo), nil
	default:
		return 0, nil
	}
}

func (f *FDC) Write(size cpu.Size, address uint32, value uint32) error {
	offset := address - fdcBase
	switch size {
	case cpu.Byte:
		return f.writeByte(offset, byte(value))
	case cpu.Word:
		return f.writeWord(offset, uint16(value))
	case cpu.Long:
		if err := f.writeWord(offset, uint16(value>>16)); err != nil {
			return err
		}
		return f.writeWord(offset+2, uint16(value))
	default:
		return nil
	}
}

func (f *FDC) Advance(uint64) {}

func (f *FDC) DrainInterrupts() []Interrupt {
	if len(f.pending) == 0 {
		return nil
	}
	out := append([]Interrupt(nil), f.pending...)
	f.pending = f.pending[:0]
	return out
}

func (f *FDC) readByte(offset uint32) byte {
	switch offset {
	case fdcOffsetData:
		return byte(f.currentDataWord() >> 8)
	case fdcOffsetData + 1:
		return byte(f.currentDataWord())
	case fdcOffsetControl:
		return byte(f.dmaStatusWord() >> 8)
	case fdcOffsetControl + 1:
		return byte(f.dmaStatusWord())
	case fdcOffsetAddrHigh:
		return byte(f.dmaAddr >> 16)
	case fdcOffsetAddrMed:
		return byte(f.dmaAddr >> 8)
	case fdcOffsetAddrLow:
		return byte(f.dmaAddr)
	case fdcOffsetModeCtl:
		return f.modeCtl
	default:
		return 0
	}
}

func (f *FDC) readWord(offset uint32) uint16 {
	switch offset {
	case fdcOffsetData:
		return f.currentDataWord()
	case fdcOffsetControl:
		return f.dmaStatusWord()
	default:
		return uint16(f.readByte(offset))<<8 | uint16(f.readByte(offset+1))
	}
}

func (f *FDC) writeByte(offset uint32, value byte) error {
	switch offset {
	case fdcOffsetAddrHigh:
		f.dmaAddr = (f.dmaAddr & 0x00FFFF) | (uint32(value) << 16)
	case fdcOffsetAddrMed:
		f.dmaAddr = (f.dmaAddr & 0xFF00FF) | (uint32(value) << 8)
	case fdcOffsetAddrLow:
		f.dmaAddr = (f.dmaAddr & 0xFFFF00) | uint32(value)
	case fdcOffsetModeCtl:
		f.modeCtl = value
	default:
		// Byte accesses to the word registers are uncommon in this path; keep
		// them benign.
	}
	return nil
}

func (f *FDC) writeWord(offset uint32, value uint16) error {
	switch offset {
	case fdcOffsetData:
		return f.writeDataWord(value)
	case fdcOffsetControl:
		f.control = value
	default:
		if err := f.writeByte(offset, byte(value>>8)); err != nil {
			return err
		}
		return f.writeByte(offset+1, byte(value))
	}
	return nil
}

func (f *FDC) currentDataWord() uint16 {
	if f.control&dmaSCReg != 0 {
		return f.sectorCount
	}
	if f.acsiSelected() {
		return f.currentACSIDataWord()
	}
	if !f.floppySelected() {
		return f.dmaData
	}

	switch f.control & (dmaA1 | dmaA0) {
	case 0:
		if f.irq != nil {
			f.irq(false)
		}
		return uint16(f.status)
	case dmaA0:
		return uint16(f.track)
	case dmaA1:
		return uint16(f.sector)
	case dmaA1 | dmaA0:
		return uint16(f.data)
	default:
		return 0
	}
}

func (f *FDC) currentACSIDataWord() uint16 {
	if f.irq != nil {
		f.irq(false)
	}
	return uint16(f.acsiStatus)
}

func (f *FDC) writeDataWord(value uint16) error {
	if f.control&dmaSCReg != 0 {
		f.sectorCount = value
		return nil
	}
	if f.acsiSelected() {
		return f.writeACSIDataWord(value)
	}
	if !f.floppySelected() {
		f.dmaData = value
		return nil
	}

	switch f.control & (dmaA1 | dmaA0) {
	case 0:
		return f.execute(byte(value))
	case dmaA0:
		f.track = byte(value)
	case dmaA1:
		f.sector = byte(value)
	case dmaA1 | dmaA0:
		f.data = byte(value)
	}
	return nil
}

func (f *FDC) writeACSIDataWord(value uint16) error {
	notNewCDB := f.control&dmaA0 != 0 // DMA_NOT_NEWCDB in ACSI mode
	if !notNewCDB && f.acsiLastNotNewCDB && f.acsiCmdLen > 0 {
		// The host asserted "new CDB" while we still had buffered bytes.
		// Drop the partial command and resync at this byte boundary.
		f.acsiCmdLen = 0
		f.acsiExpectedLen = 0
	}
	f.acsiLastNotNewCDB = notNewCDB

	b := byte(value)
	if f.acsiCmdLen == 0 {
		f.acsiExpectedLen = acsiCommandLength(b)
		if f.acsiExpectedLen < 0 || f.acsiExpectedLen > len(f.acsiCmdBuf) {
			f.acsiExpectedLen = acsiDefaultCommandSize
		}
	}
	if f.acsiCmdLen >= len(f.acsiCmdBuf) {
		f.acsiCmdLen = 0
		f.acsiExpectedLen = 0
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
	f.acsiCmdBuf[f.acsiCmdLen] = b
	f.acsiCmdLen++
	if f.acsiExpectedLen == 0 && f.acsiCmdLen >= 2 && (f.acsiCmdBuf[0]&0x1F) == 0x1F {
		f.acsiExpectedLen = acsiExtendedCommandLength(f.acsiCmdBuf[1])
		if f.acsiExpectedLen <= 0 || f.acsiExpectedLen > len(f.acsiCmdBuf) {
			f.acsiExpectedLen = len(f.acsiCmdBuf)
		}
	}
	if f.acsiExpectedLen != 0 && f.acsiCmdLen >= f.acsiExpectedLen {
		cmd := append([]byte(nil), f.acsiCmdBuf[:f.acsiCmdLen]...)
		f.acsiCmdLen = 0
		f.acsiExpectedLen = 0
		return f.execACSI(cmd)
	}
	return nil
}

func (f *FDC) dmaStatusWord() uint16 {
	status := uint16(0)
	if f.dmaOK {
		status |= dmaStatusOK
	}
	if f.sectorCount != 0 {
		status |= dmaStatusSCNot0
	}
	if f.status&fdcStatusDataRQ != 0 {
		status |= dmaStatusDataRQ
	}
	return status
}

func (f *FDC) execute(cmd byte) error {
	f.dmaOK = true
	if f.irq != nil {
		f.irq(false)
	}

	switch {
	case cmd&0xF0 == fdcCmdForceI:
		return f.execForceInterrupt()
	case cmd&0xF0 == fdcCmdRestore:
		return f.execRestore(cmd)
	case cmd&0xF0 == fdcCmdSeek:
		return f.execSeek(cmd)
	case cmd&0xE0 == fdcCmdStepOut:
		return f.execStepOut(cmd)
	case cmd&0xE0 == fdcCmdStepIn:
		return f.execStepIn(cmd)
	case cmd&0xE0 == fdcCmdStep:
		return f.execStep(cmd)
	case cmd&0xF0 == fdcCmdWriteTrack:
		return f.execWriteTrack()
	case cmd&0xF0 == fdcCmdReadTrack:
		return f.execReadTrack()
	case cmd&0xF0 == fdcCmdReadAddr:
		return f.execReadAddress()
	case cmd&0xE0 == fdcCmdWrite:
		return f.execWriteSectors(cmd)
	case cmd&0xE0 == fdcCmdRead:
		return f.execReadSectors(cmd)
	default:
		f.typeI = true
		f.status = f.baseStatus()
		f.queueInterrupt()
		return nil
	}
}

func (f *FDC) execForceInterrupt() error {
	f.typeI = true
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) execRestore(cmd byte) error {
	f.typeI = true
	f.headTrack = 0
	f.track = 0

	extra := byte(0)
	if cmd&fdcCmdFlagVerify != 0 && !f.trackInRange(0) {
		extra |= fdcStatusRNF
		f.dmaOK = false
	}
	f.status = f.baseStatus() | extra
	f.queueInterrupt()
	return nil
}

func (f *FDC) execSeek(cmd byte) error {
	f.typeI = true
	target := int(f.data)
	inRange := f.trackInRange(target)
	f.headTrack = f.clampTrack(target)
	f.track = byte(f.headTrack)
	if target > f.headTrack {
		f.lastStepDirection = +1
	} else if target < f.headTrack {
		f.lastStepDirection = -1
	}

	extra := byte(0)
	if cmd&fdcCmdFlagVerify != 0 && !inRange {
		extra |= fdcStatusRNF
		f.dmaOK = false
	}
	f.status = f.baseStatus() | extra
	f.queueInterrupt()
	return nil
}

func (f *FDC) execStep(cmd byte) error {
	return f.execRelativeStep(f.lastStepDirection, cmd)
}

func (f *FDC) execStepIn(cmd byte) error {
	f.lastStepDirection = +1
	return f.execRelativeStep(+1, cmd)
}

func (f *FDC) execStepOut(cmd byte) error {
	f.lastStepDirection = -1
	return f.execRelativeStep(-1, cmd)
}

func (f *FDC) execRelativeStep(direction int, cmd byte) error {
	f.typeI = true
	if direction == 0 {
		direction = -1
	}
	target := f.headTrack + direction
	inRange := f.trackInRange(target)
	f.headTrack = f.clampTrack(target)

	if cmd&fdcCmdFlagUpdateTrack != 0 {
		f.track = byte(f.headTrack)
	}

	extra := byte(0)
	if cmd&fdcCmdFlagVerify != 0 && !inRange {
		extra |= fdcStatusRNF
		f.dmaOK = false
	}
	f.status = f.baseStatus() | extra
	f.queueInterrupt()
	return nil
}

func (f *FDC) execReadSectors(cmd byte) error {
	f.typeI = false
	if !f.hasSelectedDisk() {
		return f.failTypeIIStatus(fdcStatusRNF)
	}

	count, multi := f.commandSectorCount(cmd)
	baseSector := int(f.sector)
	if baseSector <= 0 || baseSector+count-1 > f.sectorsPerTrack {
		return f.failTypeIIStatus(fdcStatusRNF)
	}

	buffer := make([]byte, 0, count*fdcSectorSize)
	for i := 0; i < count; i++ {
		offset, ok := f.diskOffset(int(f.track), baseSector+i)
		if !ok {
			return f.failTypeIIStatus(fdcStatusRNF)
		}
		buffer = append(buffer, f.diskA[offset:offset+fdcSectorSize]...)
	}
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, buffer); err != nil {
			return f.failTypeIIStatus(fdcStatusRNF | fdcStatusLostData)
		}
	}
	f.dmaAddr += uint32(len(buffer))
	if multi {
		f.sector = byte(baseSector + count)
	}
	f.sectorCount = 0
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) execWriteSectors(cmd byte) error {
	f.typeI = false
	if !f.hasSelectedDisk() {
		return f.failTypeIIStatus(fdcStatusRNF)
	}
	if f.diskAWriteProtected {
		return f.failTypeIIStatus(fdcStatusWriteProtect)
	}

	count, multi := f.commandSectorCount(cmd)
	baseSector := int(f.sector)
	if baseSector <= 0 || baseSector+count-1 > f.sectorsPerTrack {
		return f.failTypeIIStatus(fdcStatusRNF)
	}

	buffer := make([]byte, count*fdcSectorSize)
	if f.ram != nil {
		if err := f.ram.CopyOut(f.dmaAddr, buffer); err != nil {
			return f.failTypeIIStatus(fdcStatusRNF | fdcStatusLostData)
		}
	}
	for i := 0; i < count; i++ {
		offset, ok := f.diskOffset(int(f.track), baseSector+i)
		if !ok {
			return f.failTypeIIStatus(fdcStatusRNF)
		}
		start := i * fdcSectorSize
		copy(f.diskA[offset:offset+fdcSectorSize], buffer[start:start+fdcSectorSize])
	}
	f.dmaAddr += uint32(len(buffer))
	if multi {
		f.sector = byte(baseSector + count)
	}
	f.sectorCount = 0
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) execReadAddress() error {
	f.typeI = false
	if !f.hasSelectedDisk() {
		return f.failTypeIIStatus(fdcStatusRNF)
	}

	sector := int(f.sector)
	if sector <= 0 || sector > f.sectorsPerTrack {
		return f.failTypeIIStatus(fdcStatusRNF)
	}

	// ID field: track, side, sector, N (512 => N=2), CRC bytes.
	record := []byte{
		f.track,
		byte(f.selectedSide),
		byte(sector),
		0x02,
		0x00,
		0x00,
	}
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, record); err != nil {
			return f.failTypeIIStatus(fdcStatusLostData)
		}
	}
	f.dmaAddr += uint32(len(record))
	f.sectorCount = 0
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) execReadTrack() error {
	f.typeI = false
	if !f.hasSelectedDisk() {
		return f.failTypeIIStatus(fdcStatusRNF)
	}

	trackData := make([]byte, 0, f.sectorsPerTrack*fdcSectorSize)
	for sector := 1; sector <= f.sectorsPerTrack; sector++ {
		offset, ok := f.diskOffset(int(f.track), sector)
		if !ok {
			return f.failTypeIIStatus(fdcStatusRNF)
		}
		trackData = append(trackData, f.diskA[offset:offset+fdcSectorSize]...)
	}
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, trackData); err != nil {
			return f.failTypeIIStatus(fdcStatusLostData)
		}
	}
	f.dmaAddr += uint32(len(trackData))
	f.sectorCount = 0
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) execWriteTrack() error {
	f.typeI = false
	if !f.hasSelectedDisk() {
		return f.failTypeIIStatus(fdcStatusRNF)
	}
	if f.diskAWriteProtected {
		return f.failTypeIIStatus(fdcStatusWriteProtect)
	}

	trackData := make([]byte, f.sectorsPerTrack*fdcSectorSize)
	if f.ram != nil {
		if err := f.ram.CopyOut(f.dmaAddr, trackData); err != nil {
			return f.failTypeIIStatus(fdcStatusLostData)
		}
	}
	for sector := 1; sector <= f.sectorsPerTrack; sector++ {
		offset, ok := f.diskOffset(int(f.track), sector)
		if !ok {
			return f.failTypeIIStatus(fdcStatusRNF)
		}
		start := (sector - 1) * fdcSectorSize
		copy(f.diskA[offset:offset+fdcSectorSize], trackData[start:start+fdcSectorSize])
	}
	f.dmaAddr += uint32(len(trackData))
	f.sectorCount = 0
	f.status = f.baseStatus()
	f.queueInterrupt()
	return nil
}

func (f *FDC) failTypeIIStatus(extra byte) error {
	f.typeI = false
	f.dmaOK = false
	f.status = f.baseStatus() | extra
	f.queueInterrupt()
	return nil
}

func (f *FDC) commandSectorCount(cmd byte) (count int, multi bool) {
	count = int(f.sectorCount)
	if count <= 0 {
		count = 1
	}
	multi = cmd&fdcCmdFlagMultiSector != 0
	if !multi {
		count = 1
	}
	return count, multi
}

func (f *FDC) queueInterrupt() {
	vector := f.vector
	f.pending = append(f.pending, Interrupt{Level: 5, Vector: &vector})
	if f.irq != nil {
		f.irq(true)
	}
}

func (f *FDC) baseStatus() byte {
	var status byte
	if f.typeI && f.headTrack == 0 {
		status |= fdcStatusTrack0
	}
	if f.hasSelectedDisk() {
		status |= fdcStatusMotorOn
	}
	if f.diskAWriteProtected {
		status |= fdcStatusWriteProtect
	}
	return status
}

func (f *FDC) floppySelected() bool {
	return f.control&dmaDRQFloppy != 0 && f.control&dmaCSACSI == 0
}

func (f *FDC) acsiSelected() bool {
	return f.control&dmaCSACSI != 0
}

func (f *FDC) SetDriveControl(portA byte) {
	driveASelected := portA&0x02 == 0
	driveBSelected := portA&0x04 == 0

	switch {
	case driveASelected && !driveBSelected:
		f.selectedDrive = 0
	case driveBSelected && !driveASelected:
		f.selectedDrive = 1
	case driveASelected && driveBSelected:
		// Invalid in hardware, but keep backward behavior: prefer drive A.
		f.selectedDrive = 0
	default:
		f.selectedDrive = -1
	}

	if driveASelected != driveBSelected {
		if portA&0x01 == 0 {
			f.selectedSide = 1
		} else {
			f.selectedSide = 0
		}
	}
}

func (f *FDC) hasSelectedDisk() bool {
	return f.selectedDrive == 0 && len(f.diskA) != 0
}

func (f *FDC) hasHardDisk0() bool {
	return len(f.hardDisk0) != 0
}

func (f *FDC) diskOffset(track, sector int) (int, bool) {
	if !f.hasSelectedDisk() || f.sides <= 0 || f.sectorsPerTrack <= 0 {
		return 0, false
	}
	if track < 0 || track >= f.tracks {
		return 0, false
	}
	if sector <= 0 || sector > f.sectorsPerTrack {
		return 0, false
	}
	if f.selectedSide < 0 || f.selectedSide >= f.sides {
		return 0, false
	}

	lba := ((track * f.sides) + f.selectedSide) * f.sectorsPerTrack
	lba += sector - 1
	offset := lba * fdcSectorSize
	if offset < 0 || offset+fdcSectorSize > len(f.diskA) {
		return 0, false
	}
	return offset, true
}

func (f *FDC) trackInRange(track int) bool {
	if f.tracks <= 0 {
		return track >= 0 && track <= 255
	}
	return track >= 0 && track < f.tracks
}

func (f *FDC) clampTrack(track int) int {
	maxTrack := 255
	if f.tracks > 0 {
		maxTrack = f.tracks - 1
	}
	if track < 0 {
		return 0
	}
	if track > maxTrack {
		return maxTrack
	}
	return track
}

func inferFloppyGeometry(size int) (sectorsPerTrack, sides, tracks int) {
	if size <= 0 || size%fdcSectorSize != 0 {
		return 0, 0, 0
	}

	type candidate struct {
		sectorsPerTrack int
		sides           int
		tracks          int
		score           int
	}

	best := candidate{}
	for _, candidateSPT := range []int{9, 10, 11, 18} {
		for _, candidateSides := range []int{2, 1} {
			bytesPerTrack := candidateSPT * candidateSides * fdcSectorSize
			if size%bytesPerTrack != 0 {
				continue
			}

			candidateTracks := size / bytesPerTrack
			if candidateTracks <= 0 || candidateTracks > 255 {
				continue
			}

			score := 0
			if candidateTracks == 80 {
				score += 100
			}
			if candidateSides == 2 {
				score += 10
			}
			score -= abs(candidateTracks - 80)

			if score > best.score {
				best = candidate{
					sectorsPerTrack: candidateSPT,
					sides:           candidateSides,
					tracks:          candidateTracks,
					score:           score,
				}
			}
		}
	}

	if best.sectorsPerTrack != 0 {
		return best.sectorsPerTrack, best.sides, best.tracks
	}

	return size / fdcSectorSize, 1, 1
}

func acsiCommandLength(first byte) int {
	opcode := first & 0x1F
	switch {
	case opcode == 0x1F:
		// ICD/Supra extended command method: actual SCSI opcode is in byte 1.
		return 0
	case opcode <= 0x1F:
		return 6
	default:
		return acsiDefaultCommandSize
	}
}

func acsiExtendedCommandLength(opcode byte) int {
	switch opcode {
	case 0x25, 0x28, 0x2A, 0x32, 0x3F, 0x52, 0x5F, 0x72, 0x7F, 0x92, 0x9F, 0xB2, 0xBF, 0xD2, 0xDF, 0xF2, 0xFF:
		return 11 // 1-byte ICD wrapper + 10-byte SCSI CDB
	default:
		return 7 // 1-byte ICD wrapper + 6-byte SCSI CDB
	}
}

func (f *FDC) execACSI(cmd []byte) error {
	if len(cmd) == 0 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}

	target := int(cmd[0] >> 5)
	opcode := cmd[0] & 0x1F
	if target != 0 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
	if opcode == 0x1F {
		return f.execACSISCSICmd(cmd)
	}

	switch opcode {
	case 0x00: // TEST UNIT READY
		if !f.hasHardDisk0() {
			f.finishACSI(acsiStatusCheckCondition, acsiSenseNotReady, true)
			return nil
		}
		f.finishACSI(acsiStatusGood, acsiSenseNone, false)
		return nil
	case 0x03: // REQUEST SENSE
		return f.execACSIRequestSense(cmd)
	case 0x08: // READ(6)
		return f.execACSIReadWrite(cmd, false)
	case 0x0A: // WRITE(6)
		return f.execACSIReadWrite(cmd, true)
	case 0x12: // INQUIRY
		return f.execACSIInquiry(cmd)
	case 0x1A: // MODE SENSE(6)
		return f.execACSIModeSense(cmd)
	case 0x1B: // START/STOP UNIT
		f.finishACSI(acsiStatusGood, acsiSenseNone, false)
		return nil
	default:
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
}

func (f *FDC) execACSISCSICmd(cmd []byte) error {
	if len(cmd) < 2 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
	opcode := cmd[1]
	cdb := cmd[1:]
	switch opcode {
	case 0x25: // READ CAPACITY(10)
		return f.execACSIReadCapacity10(cdb)
	case 0x28: // READ(10)
		return f.execACSIReadWrite10(cdb, false)
	case 0x2A: // WRITE(10)
		return f.execACSIReadWrite10(cdb, true)
	default:
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
}

func (f *FDC) execACSIRequestSense(cmd []byte) error {
	length := int(cmd[4])
	if length == 0 {
		length = 4
	}
	payload := make([]byte, length)
	if length > 0 {
		payload[0] = 0x70
	}
	if length > 2 {
		payload[2] = f.acsiSense
	}
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, payload); err != nil {
			f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
			return nil
		}
	}
	f.dmaAddr += uint32(len(payload))
	f.finishACSI(acsiStatusGood, acsiSenseNone, false)
	return nil
}

func (f *FDC) execACSIInquiry(cmd []byte) error {
	length := int(cmd[4])
	if length == 0 {
		length = 36
	}
	payload := make([]byte, length)
	if length > 0 {
		payload[0] = 0x00
	}
	if length > 1 {
		payload[1] = 0x00
	}
	if length > 2 {
		payload[2] = 0x01
	}
	if length > 4 {
		payload[4] = byte(maxInt(0, length-5))
	}
	copyAt(payload, 8, []byte("GoST    "))
	copyAt(payload, 16, []byte("Virtual ACSI HD "))
	copyAt(payload, 32, []byte("1.0 "))
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, payload); err != nil {
			f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
			return nil
		}
	}
	f.dmaAddr += uint32(len(payload))
	f.finishACSI(acsiStatusGood, acsiSenseNone, false)
	return nil
}

func (f *FDC) execACSIModeSense(cmd []byte) error {
	length := int(cmd[4])
	if length == 0 {
		length = 4
	}
	payload := make([]byte, length)
	if len(payload) > 0 {
		payload[0] = byte(maxInt(0, len(payload)-1))
	}
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, payload); err != nil {
			f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
			return nil
		}
	}
	f.dmaAddr += uint32(len(payload))
	f.finishACSI(acsiStatusGood, acsiSenseNone, false)
	return nil
}

func (f *FDC) execACSIReadWrite(cmd []byte, write bool) error {
	if !f.hasHardDisk0() {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseNotReady, true)
		return nil
	}
	if write && f.hardDisk0WriteProtected {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseWriteProtect, true)
		return nil
	}

	lba := int(uint32(cmd[1]&0x1F)<<16 | uint32(cmd[2])<<8 | uint32(cmd[3]))
	count := int(cmd[4])
	if count == 0 {
		count = 256
	}
	if f.sectorCount != 0 {
		count = int(f.sectorCount)
	}
	if lba < 0 || count < 0 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
	totalSectors := len(f.hardDisk0) / fdcSectorSize
	if lba+count > totalSectors {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}

	start := lba * fdcSectorSize
	end := start + count*fdcSectorSize
	if write {
		buffer := make([]byte, end-start)
		if f.ram != nil {
			if err := f.ram.CopyOut(f.dmaAddr, buffer); err != nil {
				f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
				return nil
			}
		}
		copy(f.hardDisk0[start:end], buffer)
	} else {
		if f.ram != nil {
			if err := f.ram.LoadAt(f.dmaAddr, f.hardDisk0[start:end]); err != nil {
				f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
				return nil
			}
		}
	}

	f.dmaAddr += uint32(end - start)
	f.sectorCount = 0
	f.finishACSI(acsiStatusGood, acsiSenseNone, false)
	return nil
}

func (f *FDC) execACSIReadCapacity10(cdb []byte) error {
	if !f.hasHardDisk0() {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseNotReady, true)
		return nil
	}
	if len(cdb) < 10 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}

	totalSectors := len(f.hardDisk0) / fdcSectorSize
	if totalSectors == 0 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseNotReady, true)
		return nil
	}

	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], uint32(totalSectors-1))
	binary.BigEndian.PutUint32(payload[4:8], uint32(fdcSectorSize))
	if f.ram != nil {
		if err := f.ram.LoadAt(f.dmaAddr, payload); err != nil {
			f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
			return nil
		}
	}
	f.dmaAddr += uint32(len(payload))
	f.finishACSI(acsiStatusGood, acsiSenseNone, false)
	return nil
}

func (f *FDC) execACSIReadWrite10(cdb []byte, write bool) error {
	if len(cdb) < 10 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}
	if !f.hasHardDisk0() {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseNotReady, true)
		return nil
	}
	if write && f.hardDisk0WriteProtected {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseWriteProtect, true)
		return nil
	}

	lba := int(binary.BigEndian.Uint32(cdb[2:6]))
	count := int(binary.BigEndian.Uint16(cdb[7:9]))
	if count == 0 {
		count = 65536
	}
	if f.sectorCount != 0 {
		count = int(f.sectorCount)
	}
	if lba < 0 || count <= 0 {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}

	totalSectors := len(f.hardDisk0) / fdcSectorSize
	if lba+count > totalSectors {
		f.finishACSI(acsiStatusCheckCondition, acsiSenseIllegalReq, true)
		return nil
	}

	start := lba * fdcSectorSize
	end := start + count*fdcSectorSize
	if write {
		buffer := make([]byte, end-start)
		if f.ram != nil {
			if err := f.ram.CopyOut(f.dmaAddr, buffer); err != nil {
				f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
				return nil
			}
		}
		copy(f.hardDisk0[start:end], buffer)
	} else {
		if f.ram != nil {
			if err := f.ram.LoadAt(f.dmaAddr, f.hardDisk0[start:end]); err != nil {
				f.finishACSI(acsiStatusCheckCondition, acsiSenseMediumError, true)
				return nil
			}
		}
	}

	f.dmaAddr += uint32(end - start)
	f.sectorCount = 0
	f.finishACSI(acsiStatusGood, acsiSenseNone, false)
	return nil
}

func (f *FDC) finishACSI(status, sense byte, failed bool) {
	f.acsiStatus = status
	f.acsiSense = sense
	if failed {
		f.dmaOK = false
	} else {
		f.dmaOK = true
	}
	f.queueInterrupt()
}

func copyAt(dst []byte, offset int, src []byte) {
	if offset >= len(dst) {
		return
	}
	n := copy(dst[offset:], src)
	if n < len(src) {
		for i := offset + n; i < len(dst); i++ {
			dst[i] = ' '
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

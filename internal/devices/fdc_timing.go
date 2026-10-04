package devices

// WD1772 disk-surface timing. The ST clocks the WD1772 at 8 MHz, so its
// nominal step rates and settle time hold, and the drive turns at 300 rpm:
// one revolution is 200 ms, a double-density track 6250 MFM bytes of 32 µs.
// A command on a drive with media computes its result at once, but its status
// stays busy and its interrupt is held until this much machine time passes.
const (
	fdcCyclesPerMs      = 8_000
	fdcRevolutionCycles = 200 * fdcCyclesPerMs
	fdcDDTrackBytes     = 6250
	fdcHDTrackBytes     = 2 * fdcDDTrackBytes
	// fdcSpinUpRevs is how many index pulses a spin-up waits for.
	fdcSpinUpRevs = 6
	// fdcMotorOffCycles is the idle time (10 index pulses) after which the
	// WD1772 drops the motor line.
	fdcMotorOffCycles = 10 * fdcRevolutionCycles
	// fdcRNFRevs is how many revolutions a Type II command searches for a
	// missing sector before it reports Record Not Found.
	fdcRNFRevs      = 5
	fdcSettleCycles = 15 * fdcCyclesPerMs

	// Standard ST track layout: a 60-byte post-index gap, then per sector 16
	// bytes of sync and address mark, the 6-byte ID field, 22+12+4 bytes of
	// gap, sync and data mark, 512 data bytes, CRC and a 40-byte gap.
	fdcGap1Bytes        = 60
	fdcIDSyncBytes      = 16
	fdcIDFieldBytes     = 6
	fdcIDToDataEndBytes = fdcIDFieldBytes + 22 + 12 + 4 + fdcSectorSize + 2
	fdcSectorRecord     = fdcIDSyncBytes + fdcIDToDataEndBytes + 40
)

// fdcStepRateCycles is the Type I step rate selected by command bits 1-0.
var fdcStepRateCycles = [4]uint64{6 * fdcCyclesPerMs, 12 * fdcCyclesPerMs, 2 * fdcCyclesPerMs, 3 * fdcCyclesPerMs}

// SetFastFloppy makes floppy commands complete at once instead of taking
// WD1772 time. A command already running finishes on schedule.
func (f *FDC) SetFastFloppy(fast bool) {
	f.fast = fast
}

// Advance turns the disk and finishes a timed command once its time is up.
func (f *FDC) Advance(cycles uint64) {
	f.rotation = (f.rotation + cycles) % fdcRevolutionCycles
	if f.busyCycles > 0 {
		if cycles < f.busyCycles {
			f.busyCycles -= cycles
			return
		}
		cycles -= f.busyCycles
		f.busyCycles = 0
		f.finishTimedCommand()
	}
	if f.motorOn {
		f.motorIdle += cycles
		if f.motorIdle >= fdcMotorOffCycles {
			f.motorOn = false
		}
	}
}

// NextEventCycles reports when the running command completes.
func (f *FDC) NextEventCycles() (uint64, bool) {
	return f.busyCycles, f.busyCycles != 0
}

// runTimed executes cmd through exec with its completion interrupt held, then
// keeps the controller busy for the command's duration.
func (f *FDC) runTimed(cmd byte, exec func() error) error {
	var delay uint64
	if !f.fast {
		delay = f.commandCycles(cmd)
	}
	f.motorOn = true
	f.motorIdle = 0
	f.holdInterrupt = true
	err := exec()
	f.holdInterrupt = false
	if !f.heldInterrupt {
		return err
	}
	if delay == 0 {
		f.heldInterrupt = false
		f.queueInterrupt()
		return err
	}
	f.busyCycles = delay
	f.status |= fdcStatusBusy
	return err
}

// cancelTimedCommand drops a command still in progress, as Force Interrupt does.
func (f *FDC) cancelTimedCommand() {
	f.busyCycles = 0
	f.heldInterrupt = false
}

func (f *FDC) finishTimedCommand() {
	f.status &^= fdcStatusBusy
	f.motorIdle = 0
	if f.heldInterrupt {
		f.heldInterrupt = false
		f.queueInterrupt()
	}
}

// commandCycles is how long cmd takes on the selected drive, given the motor,
// head and rotational position before it starts.
func (f *FDC) commandCycles(cmd byte) uint64 {
	var t uint64
	if !f.motorOn && cmd&fdcCmdFlagNoSpinUp == 0 {
		// Spin-up ends on the sixth index pulse.
		t = f.cyclesToTrackByte(0, 0) + (fdcSpinUpRevs-1)*fdcRevolutionCycles
	}

	switch {
	case cmd&0x80 == 0: // Type I: restore, seek, step
		t += f.typeISteps(cmd) * fdcStepRateCycles[cmd&0x03]
		if cmd&fdcCmdFlagVerify != 0 {
			t += fdcSettleCycles
			t += f.cyclesToNextID(t) + fdcIDFieldBytes*f.byteCycles()
		}
	case cmd&0xC0 == 0x80: // Type II: read/write sector
		if cmd&fdcCmdFlagVerify != 0 { // E flag: head settle
			t += fdcSettleCycles
		}
		t += f.sectorCycles(cmd, t)
	case cmd&0xF0 == fdcCmdReadAddr:
		if cmd&fdcCmdFlagVerify != 0 {
			t += fdcSettleCycles
		}
		t += f.cyclesToNextID(t) + fdcIDFieldBytes*f.byteCycles()
	default: // read/write track: index pulse to index pulse
		if cmd&fdcCmdFlagVerify != 0 {
			t += fdcSettleCycles
		}
		t += f.cyclesToTrackByte(t, 0) + fdcRevolutionCycles
	}
	return t
}

func (f *FDC) typeISteps(cmd byte) uint64 {
	switch {
	case cmd&0xF0 == fdcCmdRestore:
		return uint64(f.headTrack)
	case cmd&0xF0 == fdcCmdSeek:
		return uint64(max(int(f.data), int(f.track)) - min(int(f.data), int(f.track)))
	default:
		return 1
	}
}

// sectorCycles is the time from t until the last sector of a Type II command
// has passed under the head, or until the search for a missing one gives up.
func (f *FDC) sectorCycles(cmd byte, t uint64) uint64 {
	disk := f.selectedDisk()
	count, _ := f.commandSectorCount(cmd)
	first := int(f.sector)
	if disk == nil || first <= 0 || first+count-1 > disk.sectorsPerTrack || !f.trackInRange(int(f.track)) {
		return f.cyclesToTrackByte(t, 0) + (fdcRNFRevs-1)*fdcRevolutionCycles
	}
	wait := f.cyclesToTrackByte(t, f.sectorIDByte(first))
	bytes := uint64((count-1)*f.sectorRecordBytes() + fdcIDToDataEndBytes)
	return wait + bytes*f.byteCycles()
}

// cyclesToNextID is the time from t until the next ID field starts.
func (f *FDC) cyclesToNextID(t uint64) uint64 {
	disk := f.selectedDisk()
	if disk == nil || disk.sectorsPerTrack <= 0 {
		return f.cyclesToTrackByte(t, 0) + (fdcRNFRevs-1)*fdcRevolutionCycles
	}
	best := uint64(fdcRevolutionCycles)
	for sector := 1; sector <= disk.sectorsPerTrack; sector++ {
		best = min(best, f.cyclesToTrackByte(t, f.sectorIDByte(sector)))
	}
	return best
}

// cyclesToTrackByte is the time from t until track byte b is under the head;
// byte 0 is the index pulse.
func (f *FDC) cyclesToTrackByte(t uint64, b int) uint64 {
	pos := (f.rotation + t) % fdcRevolutionCycles
	target := uint64(b) * f.byteCycles() % fdcRevolutionCycles
	return (target + fdcRevolutionCycles - pos) % fdcRevolutionCycles
}

// sectorIDByte is the track byte where sector's ID field starts.
func (f *FDC) sectorIDByte(sector int) int {
	return fdcGap1Bytes + (sector-1)*f.sectorRecordBytes() + fdcIDSyncBytes
}

// sectorRecordBytes is one sector's share of the track, squeezed for the
// 10- and 11-sector formats.
func (f *FDC) sectorRecordBytes() int {
	disk := f.selectedDisk()
	if disk == nil || disk.sectorsPerTrack <= 0 {
		return fdcSectorRecord
	}
	return min(fdcSectorRecord, (f.trackBytes()-fdcGap1Bytes)/disk.sectorsPerTrack)
}

// trackBytes is the MFM bytes per revolution: high density above 11 sectors.
func (f *FDC) trackBytes() int {
	if disk := f.selectedDisk(); disk != nil && disk.sectorsPerTrack > 11 {
		return fdcHDTrackBytes
	}
	return fdcDDTrackBytes
}

func (f *FDC) byteCycles() uint64 {
	return fdcRevolutionCycles / uint64(f.trackBytes())
}

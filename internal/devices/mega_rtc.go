package devices

import (
	"time"

	cpu "github.com/jenska/m68kemu"
)

// The Mega ST and Mega STE carry a Ricoh RP5C15 real-time clock. Its sixteen
// 4-bit registers sit on the odd bytes of $FFFC21-$FFFC3F.
const (
	megaRTCBase = 0xFFFC20
	megaRTCEnd  = 0xFFFC40 // exclusive

	megaRTCRegSecondsLow  = 0x0
	megaRTCRegSecondsHigh = 0x1
	megaRTCRegMinutesLow  = 0x2
	megaRTCRegMinutesHigh = 0x3
	megaRTCRegHoursLow    = 0x4
	megaRTCRegHoursHigh   = 0x5
	megaRTCRegDayOfWeek   = 0x6
	megaRTCRegDayLow      = 0x7
	megaRTCRegDayHigh     = 0x8
	megaRTCRegMonthLow    = 0x9
	megaRTCRegMonthHigh   = 0xA
	megaRTCRegYearLow     = 0xB
	megaRTCRegYearHigh    = 0xC
	megaRTCRegMode        = 0xD
	megaRTCRegTest        = 0xE
	megaRTCRegReset       = 0xF

	megaRTCTimeRegs = 13 // registers 0-12 are banked

	megaRTCModeBank1    = 0x01
	megaRTCModeTimerEn  = 0x08
	megaRTCBank1Select  = 0xA // bank 1: 12/24-hour select
	megaRTC24Hour       = 0x01
	megaRTCHoursHighPM  = 0x02 // 12-hour mode: PM flag in the hours tens digit
	megaRTCYearBase     = 1980 // TOS stores the year as an offset from 1980
	megaRTCUnusedNibble = 0xF0 // the chip drives only D0-D3
)

// MegaRTC models the RP5C15 clock of the Mega machines. Bank 0 is the running
// clock: its digits as last set (or read) at baseAt, advanced by the host time
// elapsed since then. Bank 1 (alarm, 12/24-hour select, leap-year counter) is
// plain nibble storage, which is what TOS and EmuTOS use to detect the chip.
type MegaRTC struct {
	now    func() time.Time
	base   [megaRTCTimeRegs]byte
	baseAt time.Time
	mode   byte
	test   byte
	bank1  [megaRTCTimeRegs]byte
}

func NewMegaRTC() *MegaRTC {
	return newMegaRTCWithClock(time.Now)
}

func newMegaRTCWithClock(now func() time.Time) *MegaRTC {
	r := &MegaRTC{now: now, mode: megaRTCModeTimerEn}
	r.bank1[megaRTCBank1Select] = megaRTC24Hour
	r.baseAt = now()
	r.base = r.encode(r.baseAt)
	return r
}

func (r *MegaRTC) AddressRange() (uint32, uint32) {
	return megaRTCBase, megaRTCEnd - 1
}

func (r *MegaRTC) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (r *MegaRTC) Read(size cpu.Size, address uint32) (uint32, error) {
	if size != cpu.Byte {
		// The clock sits on the low byte of the data bus.
		return 0xFF00 | uint32(r.readByte(address|1)), nil
	}
	if address&1 == 0 {
		return 0xFF, nil
	}
	return uint32(r.readByte(address)), nil
}

func (r *MegaRTC) Peek(size cpu.Size, address uint32) (uint32, error) {
	return r.Read(size, address)
}

func (r *MegaRTC) Write(size cpu.Size, address uint32, value uint32) error {
	if size == cpu.Byte && address&1 == 0 {
		return nil
	}
	r.writeReg(megaRTCReg(address|1), byte(value)&0x0F)
	return nil
}

// Reset leaves the clock alone: it is battery-backed and keeps running across
// a machine reset.
func (r *MegaRTC) Reset() {}

func megaRTCReg(address uint32) int {
	return int(((address - megaRTCBase) >> 1) & 0x0F)
}

func (r *MegaRTC) readByte(address uint32) byte {
	return megaRTCUnusedNibble | r.readReg(megaRTCReg(address))
}

func (r *MegaRTC) readReg(reg int) byte {
	switch {
	case reg == megaRTCRegMode:
		return r.mode
	case reg == megaRTCRegTest, reg == megaRTCRegReset:
		return 0 // write-only
	case r.mode&megaRTCModeBank1 != 0:
		return r.bank1[reg]
	default:
		r.advance()
		return r.base[reg]
	}
}

func (r *MegaRTC) writeReg(reg int, value byte) {
	switch {
	case reg == megaRTCRegMode:
		r.advance()
		if r.mode&megaRTCModeTimerEn == 0 && value&megaRTCModeTimerEn != 0 {
			r.baseAt = r.now() // restarting: count from here
		}
		r.mode = value
	case reg == megaRTCRegTest:
		r.test = value
	case reg == megaRTCRegReset:
		// Alarm/timer reset and the 1 Hz/16 Hz outputs have no effect here.
	case r.mode&megaRTCModeBank1 != 0:
		r.bank1[reg] = value
	default:
		r.advance()
		r.base[reg] = value
	}
}

// advance rolls the bank 0 digits forward by the whole seconds elapsed since
// baseAt. Digits are stored as written and only normalised once time actually
// moves on, so a guest setting the clock digit by digit never sees an
// intermediate date such as 31 February rolled over.
func (r *MegaRTC) advance() {
	if r.mode&megaRTCModeTimerEn == 0 {
		return
	}
	elapsed := r.now().Sub(r.baseAt).Truncate(time.Second)
	if elapsed <= 0 {
		return
	}
	r.base = r.encode(r.decode().Add(elapsed))
	r.baseAt = r.baseAt.Add(elapsed)
}

func (r *MegaRTC) is24Hour() bool {
	return r.bank1[megaRTCBank1Select]&megaRTC24Hour != 0
}

func (r *MegaRTC) encode(t time.Time) [megaRTCTimeRegs]byte {
	var d [megaRTCTimeRegs]byte
	setBCD := func(low, high int, value int) {
		d[low] = byte(value % 10)
		d[high] = byte(value / 10 % 10)
	}
	setBCD(megaRTCRegSecondsLow, megaRTCRegSecondsHigh, t.Second())
	setBCD(megaRTCRegMinutesLow, megaRTCRegMinutesHigh, t.Minute())
	hour := t.Hour()
	if r.is24Hour() {
		setBCD(megaRTCRegHoursLow, megaRTCRegHoursHigh, hour)
	} else {
		pm := hour >= 12
		if hour %= 12; hour == 0 {
			hour = 12
		}
		setBCD(megaRTCRegHoursLow, megaRTCRegHoursHigh, hour)
		if pm {
			d[megaRTCRegHoursHigh] |= megaRTCHoursHighPM
		}
	}
	d[megaRTCRegDayOfWeek] = byte(t.Weekday())
	setBCD(megaRTCRegDayLow, megaRTCRegDayHigh, t.Day())
	setBCD(megaRTCRegMonthLow, megaRTCRegMonthHigh, int(t.Month()))
	setBCD(megaRTCRegYearLow, megaRTCRegYearHigh, t.Year()-megaRTCYearBase)
	return d
}

// decode reads the bank 0 digits as a time. Out-of-range values are
// normalised the way time.Date does.
func (r *MegaRTC) decode() time.Time {
	d := r.base
	bcd := func(low, high int) int { return int(d[high])*10 + int(d[low]) }
	hour := bcd(megaRTCRegHoursLow, megaRTCRegHoursHigh)
	if !r.is24Hour() {
		hour = int(d[megaRTCRegHoursHigh]&0x01)*10 + int(d[megaRTCRegHoursLow])
		hour %= 12
		if d[megaRTCRegHoursHigh]&megaRTCHoursHighPM != 0 {
			hour += 12
		}
	}
	return time.Date(
		megaRTCYearBase+bcd(megaRTCRegYearLow, megaRTCRegYearHigh),
		time.Month(bcd(megaRTCRegMonthLow, megaRTCRegMonthHigh)),
		bcd(megaRTCRegDayLow, megaRTCRegDayHigh),
		hour,
		bcd(megaRTCRegMinutesLow, megaRTCRegMinutesHigh),
		bcd(megaRTCRegSecondsLow, megaRTCRegSecondsHigh),
		0, r.baseAt.Location(),
	)
}

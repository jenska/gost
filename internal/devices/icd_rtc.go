package devices

import "time"

const (
	icdRTCRegSecondsLow = iota
	icdRTCRegSecondsHigh
	icdRTCRegMinutesLow
	icdRTCRegMinutesHigh
	icdRTCRegHoursLow
	icdRTCRegHoursHigh
	icdRTCRegDayOfWeek
	icdRTCRegDayLow
	icdRTCRegDayHigh
	icdRTCRegMonthLow
	icdRTCRegMonthHigh
	icdRTCRegYearLow
	icdRTCRegYearHigh

	icdRTCRegCount = 13
	icdRTC24Hour   = 0x08

	icdRTCCmdEnd    = 0x40
	icdRTCCmdRead   = 0x80
	icdRTCCmdBegin  = 0xC0
	icdRTCCmdSelect = 0x20
	icdRTCCmdWrite  = 0x10
)

// ICDRTC models the nibble-oriented real-time clock protocol used by ICD's
// AdSCSI Plus ST hardware.
type ICDRTC struct {
	now        func() time.Time
	clockBase  time.Time
	clockSetAt time.Time
	active     bool
	selected   int
	readLatch  byte
	selectHigh bool
	armed      bool
	shadow     [icdRTCRegCount]byte
}

func NewICDRTC() *ICDRTC {
	rtc := &ICDRTC{now: time.Now}
	rtc.Reset()
	return rtc
}

func (r *ICDRTC) Reset() {
	now := r.nowTime()
	r.clockBase = now
	r.clockSetAt = now
	r.active = false
	r.selected = 0
	r.readLatch = 0
	r.selectHigh = false
	r.armed = false
	r.shadow = r.encodeRegisters(now)
}

func (r *ICDRTC) Active() bool {
	return r != nil && r.active
}

func (r *ICDRTC) ReadDataWord() uint16 {
	if r == nil {
		return 0
	}
	return uint16(r.readLatch & 0x0F)
}

// HandleCommand consumes a DMA data byte when it belongs to the ICD RTC
// protocol, returning true when the clock handled it.
func (r *ICDRTC) HandleCommand(value byte) bool {
	if r == nil {
		return false
	}

	switch {
	case value == icdRTCCmdEnd:
		r.active = false
		r.selectHigh = false
		r.armed = false
		return true
	case value&0xF0 == (icdRTCCmdBegin | icdRTCCmdWrite):
		r.beginSession()
		if r.armed {
			r.writeSelectedNibble(value & 0x0F)
		}
		r.selectHigh = false
		r.armed = false
		return true
	case value&0xF0 == (icdRTCCmdBegin | icdRTCCmdSelect):
		r.beginSession()
		r.selectRegister(int(value & 0x0F))
		r.selectHigh = true
		return true
	case value&0xF0 == icdRTCCmdRead:
		r.beginSession()
		r.readLatch = r.readRegister(r.selected)
		r.selectHigh = false
		r.armed = false
		return true
	case value&0xC0 == icdRTCCmdBegin:
		r.beginSession()
		if r.selectHigh {
			r.selectHigh = false
			r.armed = true
			return true
		}
		if r.armed {
			return true
		}
		r.selectRegister(int(value & 0x0F))
		return true
	default:
		return r.active
	}
}

func (r *ICDRTC) beginSession() {
	if !r.active {
		r.shadow = r.encodeRegisters(r.currentClock())
	}
	r.active = true
}

func (r *ICDRTC) nowTime() time.Time {
	if r.now == nil {
		r.now = time.Now
	}
	return r.now().Local().Truncate(time.Second)
}

func (r *ICDRTC) currentClock() time.Time {
	if r.clockBase.IsZero() {
		now := r.nowTime()
		r.clockBase = now
		r.clockSetAt = now
	}
	elapsed := r.nowTime().Sub(r.clockSetAt)
	if elapsed < 0 {
		elapsed = 0
	}
	return r.clockBase.Add(elapsed).Truncate(time.Second)
}

func (r *ICDRTC) selectRegister(index int) {
	if index < 0 || index >= icdRTCRegCount {
		return
	}
	r.selected = index
}

func (r *ICDRTC) readRegister(index int) byte {
	regs := r.encodeRegisters(r.currentClock())
	if index < 0 || index >= len(regs) {
		return 0
	}
	return regs[index]
}

func (r *ICDRTC) writeSelectedNibble(value byte) {
	if r.selected < 0 || r.selected >= len(r.shadow) {
		return
	}
	r.shadow[r.selected] = value & 0x0F

	second := int(r.shadow[icdRTCRegSecondsHigh])*10 + int(r.shadow[icdRTCRegSecondsLow])
	minute := int(r.shadow[icdRTCRegMinutesHigh])*10 + int(r.shadow[icdRTCRegMinutesLow])
	hour := int(r.shadow[icdRTCRegHoursHigh]&0x03)*10 + int(r.shadow[icdRTCRegHoursLow])
	day := int(r.shadow[icdRTCRegDayHigh])*10 + int(r.shadow[icdRTCRegDayLow])
	month := int(r.shadow[icdRTCRegMonthHigh])*10 + int(r.shadow[icdRTCRegMonthLow])
	year := int(r.shadow[icdRTCRegYearHigh])*10 + int(r.shadow[icdRTCRegYearLow])
	if second < 0 || second > 59 || minute < 0 || minute > 59 || hour < 0 || hour > 23 {
		return
	}
	if month < 1 || month > 12 {
		return
	}
	fullYear := 1900 + year
	maxDay := daysIn(fullYear, time.Month(month))
	if day < 1 || day > maxDay {
		return
	}

	current := r.currentClock()
	updated := time.Date(fullYear, time.Month(month), day, hour, minute, second, 0, current.Location())
	now := r.nowTime()
	r.clockBase = updated
	r.clockSetAt = now
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (r *ICDRTC) encodeRegisters(current time.Time) [icdRTCRegCount]byte {
	year := max(current.Year()-1900, 0)
	return [icdRTCRegCount]byte{
		byte(current.Second() % 10),
		byte(current.Second() / 10),
		byte(current.Minute() % 10),
		byte(current.Minute() / 10),
		byte(current.Hour() % 10),
		byte(current.Hour()/10) | icdRTC24Hour,
		byte(current.Weekday() % 7),
		byte(current.Day() % 10),
		byte(current.Day() / 10),
		byte(int(current.Month()) % 10),
		byte(int(current.Month()) / 10),
		byte(year % 10),
		byte(year / 10),
	}
}

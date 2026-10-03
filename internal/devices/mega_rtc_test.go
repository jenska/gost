package devices

import (
	"testing"
	"time"

	cpu "github.com/jenska/m68kemu"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func megaRTCAddr(reg int) uint32 { return megaRTCBase + 1 + uint32(reg)*2 }

func readMegaRTC(t *testing.T, r *MegaRTC, reg int) byte {
	t.Helper()
	v, err := r.Read(cpu.Byte, megaRTCAddr(reg))
	if err != nil {
		t.Fatalf("read reg %X: %v", reg, err)
	}
	if v&0xF0 != megaRTCUnusedNibble {
		t.Fatalf("reg %X: upper nibble %02x, want %02x", reg, v&0xF0, megaRTCUnusedNibble)
	}
	return byte(v & 0x0F)
}

func writeMegaRTC(t *testing.T, r *MegaRTC, reg int, value byte) {
	t.Helper()
	if err := r.Write(cpu.Byte, megaRTCAddr(reg), uint32(value)); err != nil {
		t.Fatalf("write reg %X: %v", reg, err)
	}
}

func readMegaRTCTime(t *testing.T, r *MegaRTC) [megaRTCTimeRegs]byte {
	t.Helper()
	var d [megaRTCTimeRegs]byte
	for reg := range d {
		d[reg] = readMegaRTC(t, r, reg)
	}
	return d
}

func TestMegaRTCReadsHostTime(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, time.October, 3, 14, 7, 59, 0, time.Local)}
	r := newMegaRTCWithClock(clock.now)

	want := [megaRTCTimeRegs]byte{9, 5, 7, 0, 4, 1, byte(time.Saturday), 3, 0, 0, 1, 6, 4}
	if got := readMegaRTCTime(t, r); got != want {
		t.Fatalf("time digits = %v, want %v", got, want)
	}

	clock.t = clock.t.Add(1500 * time.Millisecond) // 14:08:00.5
	if got := readMegaRTC(t, r, megaRTCRegMinutesLow); got != 8 {
		t.Fatalf("minutes after rollover = %d, want 8", got)
	}
	if got := readMegaRTC(t, r, megaRTCRegSecondsLow); got != 0 {
		t.Fatalf("seconds after rollover = %d, want 0", got)
	}
}

// TestMegaRTCBank1HoldsAlarmDigits mirrors the EmuTOS detection: select bank 1,
// write the alarm minute digits, and expect them back.
func TestMegaRTCBank1HoldsAlarmDigits(t *testing.T) {
	r := newMegaRTCWithClock((&fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)}).now)

	mode := readMegaRTC(t, r, megaRTCRegMode)
	writeMegaRTC(t, r, megaRTCRegMode, mode&0x0C|megaRTCModeBank1)
	writeMegaRTC(t, r, megaRTCRegMinutesLow, 10)
	writeMegaRTC(t, r, megaRTCRegMinutesHigh, 5)
	if lo, hi := readMegaRTC(t, r, megaRTCRegMinutesLow), readMegaRTC(t, r, megaRTCRegMinutesHigh); lo != 10 || hi != 5 {
		t.Fatalf("bank 1 alarm minutes = %d/%d, want 10/5", lo, hi)
	}

	writeMegaRTC(t, r, megaRTCRegMode, mode&0x0C)
	if got := readMegaRTC(t, r, megaRTCRegMinutesLow); got != 0 {
		t.Fatalf("bank 0 minutes = %d, want 0 (bank 1 writes must not touch the clock)", got)
	}
}

func TestMegaRTCSetTimeDigitByDigit(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)}
	r := newMegaRTCWithClock(clock.now)

	// 2027-03-31 23:59:58: the day digits pass through 31 February on the way.
	set := [megaRTCTimeRegs]byte{8, 5, 9, 5, 3, 2, 0, 1, 3, 3, 0, 7, 4}
	for reg, v := range set {
		if reg == megaRTCRegDayOfWeek {
			continue
		}
		writeMegaRTC(t, r, reg, v)
	}
	got := readMegaRTCTime(t, r)
	got[megaRTCRegDayOfWeek] = 0
	if got != set {
		t.Fatalf("digits right after setting = %v, want %v", got, set)
	}

	clock.t = clock.t.Add(3 * time.Second) // 2027-04-01 00:00:01
	want := [megaRTCTimeRegs]byte{1, 0, 0, 0, 0, 0, byte(time.Thursday), 1, 0, 4, 0, 7, 4}
	if got := readMegaRTCTime(t, r); got != want {
		t.Fatalf("digits 3s later = %v, want %v", got, want)
	}
}

func TestMegaRTCStopsWhileTimerDisabled(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)}
	r := newMegaRTCWithClock(clock.now)

	writeMegaRTC(t, r, megaRTCRegMode, 0) // TIMER EN off
	clock.t = clock.t.Add(5 * time.Second)
	if got := readMegaRTC(t, r, megaRTCRegSecondsLow); got != 0 {
		t.Fatalf("seconds while stopped = %d, want 0", got)
	}

	writeMegaRTC(t, r, megaRTCRegMode, megaRTCModeTimerEn)
	clock.t = clock.t.Add(2 * time.Second)
	if got := readMegaRTC(t, r, megaRTCRegSecondsLow); got != 2 {
		t.Fatalf("seconds after restart = %d, want 2", got)
	}
}

func TestMegaRTC12HourMode(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 15, 30, 0, 0, time.Local)}
	r := newMegaRTCWithClock(clock.now)

	writeMegaRTC(t, r, megaRTCRegMode, megaRTCModeTimerEn|megaRTCModeBank1)
	writeMegaRTC(t, r, megaRTCBank1Select, 0) // 12-hour mode
	writeMegaRTC(t, r, megaRTCRegMode, megaRTCModeTimerEn)

	// Switching modes does not rewrite the stored digits; re-encode them as
	// the guest would by setting the time again.
	r.base = r.encode(clock.t)
	if lo, hi := readMegaRTC(t, r, megaRTCRegHoursLow), readMegaRTC(t, r, megaRTCRegHoursHigh); lo != 3 || hi != megaRTCHoursHighPM {
		t.Fatalf("12-hour digits = %d/%d, want 3/PM", lo, hi)
	}
	if got := r.decode(); got.Hour() != 15 {
		t.Fatalf("decoded hour = %d, want 15", got.Hour())
	}
}

func TestMegaRTCEvenBytesReadOpen(t *testing.T) {
	r := NewMegaRTC()
	v, err := r.Read(cpu.Byte, megaRTCBase)
	if err != nil {
		t.Fatalf("read even byte: %v", err)
	}
	if v != 0xFF {
		t.Fatalf("even byte = %02x, want ff", v)
	}
}

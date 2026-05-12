package devices

import (
	"testing"
	"time"

	cpu "github.com/jenska/m68kemu"
)

func TestICDRTCReadRegistersThroughFDC(t *testing.T) {
	now := time.Date(2026, time.May, 12, 14, 35, 48, 0, time.Local)
	rtc := NewICDRTC()
	rtc.now = func() time.Time { return now }
	rtc.Reset()

	fdc := NewFDC(NewRAM(0, 1024*1024), nil)
	fdc.AttachICDRTC(rtc)

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaCSACSI); err != nil {
		t.Fatalf("select ACSI: %v", err)
	}

	cases := []struct {
		reg  byte
		want byte
	}{
		{icdRTCRegSecondsLow, 8},
		{icdRTCRegSecondsHigh, 4},
		{icdRTCRegMinutesLow, 5},
		{icdRTCRegMinutesHigh, 3},
		{icdRTCRegHoursLow, 4},
		{icdRTCRegHoursHigh, 0x09},
		{icdRTCRegDayOfWeek, byte(now.Weekday() % 7)},
		{icdRTCRegDayLow, 2},
		{icdRTCRegDayHigh, 1},
		{icdRTCRegMonthLow, 5},
		{icdRTCRegMonthHigh, 0},
		{icdRTCRegYearLow, 6},
		{icdRTCRegYearHigh, 12},
	}

	for _, tc := range cases {
		got := readICDRTCRegisterViaFDC(t, fdc, tc.reg)
		if got != tc.want {
			t.Fatalf("register %d = %x, want %x", tc.reg, got, tc.want)
		}
	}
}

func TestICDRTCReadRegistersAdvanceWithHostClock(t *testing.T) {
	now := time.Date(2026, time.May, 12, 14, 35, 48, 0, time.Local)
	rtc := NewICDRTC()
	rtc.now = func() time.Time { return now }
	rtc.Reset()

	fdc := NewFDC(NewRAM(0, 1024*1024), nil)
	fdc.AttachICDRTC(rtc)

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaCSACSI); err != nil {
		t.Fatalf("select ACSI: %v", err)
	}

	now = now.Add(3 * time.Second)
	if got := readICDRTCRegisterViaFDC(t, fdc, icdRTCRegSecondsLow); got != 1 {
		t.Fatalf("seconds low after clock advance = %x, want 1", got)
	}
	if got := readICDRTCRegisterViaFDC(t, fdc, icdRTCRegSecondsHigh); got != 5 {
		t.Fatalf("seconds high after clock advance = %x, want 5", got)
	}
}

func TestICDRTCWriteRegistersThroughFDC(t *testing.T) {
	now := time.Date(2026, time.May, 12, 14, 35, 48, 0, time.Local)
	rtc := NewICDRTC()
	rtc.now = func() time.Time { return now }
	rtc.Reset()

	fdc := NewFDC(NewRAM(0, 1024*1024), nil)
	fdc.AttachICDRTC(rtc)

	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetControl, dmaCSACSI); err != nil {
		t.Fatalf("select ACSI: %v", err)
	}

	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegYearHigh, 13)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegYearLow, 1)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegMonthHigh, 1)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegMonthLow, 2)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegDayHigh, 2)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegDayLow, 5)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegHoursHigh, 0x0A)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegHoursLow, 3)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegMinutesHigh, 5)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegMinutesLow, 9)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegSecondsHigh, 5)
	writeICDRTCRegisterViaFDC(t, fdc, icdRTCRegSecondsLow, 8)

	current := rtc.currentClock()
	if !current.Equal(time.Date(2031, time.December, 25, 23, 59, 58, 0, time.Local)) {
		t.Fatalf("current clock = %v, want 2031-12-25 23:59:58 local", current)
	}
}

func readICDRTCRegisterViaFDC(t *testing.T, fdc *FDC, reg byte) byte {
	t.Helper()
	for _, value := range []uint16{
		uint16(icdRTCCmdBegin | reg),
		uint16(icdRTCCmdBegin | icdRTCCmdSelect | reg),
		uint16(icdRTCCmdBegin | reg),
		uint16(icdRTCCmdRead | reg),
	} {
		if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, uint32(value)); err != nil {
			t.Fatalf("write RTC command %02x: %v", value, err)
		}
	}
	got, err := fdc.Read(cpu.Word, fdcBase+fdcOffsetData)
	if err != nil {
		t.Fatalf("read RTC data: %v", err)
	}
	if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, icdRTCCmdBegin); err != nil {
		t.Fatalf("write RTC keepalive: %v", err)
	}
	return byte(got & 0x0F)
}

func writeICDRTCRegisterViaFDC(t *testing.T, fdc *FDC, reg byte, value byte) {
	t.Helper()
	for _, command := range []uint16{
		uint16(icdRTCCmdBegin | reg),
		uint16(icdRTCCmdBegin | icdRTCCmdSelect | reg),
		uint16(icdRTCCmdBegin | reg),
		uint16(icdRTCCmdBegin | (value & 0x0F)),
		uint16(icdRTCCmdBegin | icdRTCCmdWrite | (value & 0x0F)),
		uint16(icdRTCCmdBegin | (value & 0x0F)),
	} {
		if err := fdc.Write(cpu.Word, fdcBase+fdcOffsetData, uint32(command)); err != nil {
			t.Fatalf("write RTC command %02x: %v", command, err)
		}
	}
}

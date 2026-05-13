package devices

import (
	"testing"

	"github.com/jenska/gost/internal/config"
)

func TestMFPTimerQueuesInterrupt(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERA, 0x20); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRA, 0x20); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTADR, 1); err != nil {
		t.Fatalf("write timer data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTACR, 1); err != nil {
		t.Fatalf("write timer control: %v", err)
	}

	mfp.Advance(14)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected 1 interrupt, got %d", len(irqs))
	}
	if irqs[0].Level != 6 {
		t.Fatalf("unexpected interrupt level: got %d want 6", irqs[0].Level)
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x4D {
		t.Fatalf("unexpected vector: %+v", irqs[0].Vector)
	}
}

func TestMFPTimerCQueuesInterrupt(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x20); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x20); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDR, 1); err != nil {
		t.Fatalf("write timer c data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDCR, 0x10); err != nil {
		t.Fatalf("write timer cd control: %v", err)
	}

	mfp.Advance(14)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected 1 interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x45 {
		t.Fatalf("unexpected vector: %+v", irqs[0].Vector)
	}
}

func TestMFPSoftwareEOIBlocksLowerPriorityInterrupts(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x48); err != nil {
		t.Fatalf("write vector base with software eoi: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERA, 0x21); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRA, 0x21); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTADR, 1); err != nil {
		t.Fatalf("write timer a data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTBDR, 1); err != nil {
		t.Fatalf("write timer b data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTACR, 1); err != nil {
		t.Fatalf("write timer a control: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTBCR, 1); err != nil {
		t.Fatalf("write timer b control: %v", err)
	}

	mfp.Advance(14)

	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected 1 interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x4D {
		t.Fatalf("unexpected vector for highest priority interrupt: %+v", irqs[0].Vector)
	}

	isra, err := mfp.Read(1, mfpBase+mfpISRA)
	if err != nil {
		t.Fatalf("read ISRA: %v", err)
	}
	if isr := byte(isra); isr&0x20 == 0 {
		t.Fatalf("timer A should be in service, ISRA=%02x", isr)
	}

	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected lower priority interrupt to be blocked, got %d", len(irqs))
	}

	if err := mfp.Write(1, mfpBase+mfpISRA, 0xDF); err != nil {
		t.Fatalf("clear in-service bit: %v", err)
	}

	irqs = mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected 1 interrupt after software eoi, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x48 {
		t.Fatalf("unexpected vector after software eoi: %+v", irqs[0].Vector)
	}
}

func TestMFPWritingPendingRegisterClearsPendingInterrupt(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpIERA, 0x20); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRA, 0x20); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTADR, 1); err != nil {
		t.Fatalf("write timer a data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTACR, 1); err != nil {
		t.Fatalf("write timer a control: %v", err)
	}

	mfp.Advance(14)

	if err := mfp.Write(1, mfpBase+mfpIPRA, 0x00); err != nil {
		t.Fatalf("clear pending register: %v", err)
	}

	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected pending interrupt to be cleared, got %d", len(irqs))
	}
}

func TestMFPTimerAccumulatesFractionalCPUClock(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x20); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x20); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDR, 1); err != nil {
		t.Fatalf("write timer c data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDCR, 0x10); err != nil {
		t.Fatalf("write timer cd control: %v", err)
	}

	mfp.Advance(13)
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected no interrupt before 14 CPU cycles, got %d", len(irqs))
	}

	mfp.Advance(1)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected 1 interrupt after 14 CPU cycles, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x45 {
		t.Fatalf("unexpected vector: %+v", irqs[0].Vector)
	}
}

func TestMFPAutoEOITimerCRepeats(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x20); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x20); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDR, 1); err != nil {
		t.Fatalf("write timer c data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDCR, 0x10); err != nil {
		t.Fatalf("write timer cd control: %v", err)
	}

	mfp.Advance(14)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected first timer c interrupt, got %d", len(irqs))
	}

	mfp.Advance(14)
	irqs = mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected recurring timer c interrupt under auto-EOI, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x45 {
		t.Fatalf("unexpected recurring timer c vector: %+v", irqs[0].Vector)
	}
}

func TestMFPTimerNextEventCycles(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpTCDR, 1); err != nil {
		t.Fatalf("write timer c data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDCR, 0x10); err != nil {
		t.Fatalf("write timer cd control: %v", err)
	}

	cycles, ok := mfp.NextEventCycles()
	if !ok {
		t.Fatalf("expected enabled timer to report a next event")
	}
	if cycles != 14 {
		t.Fatalf("unexpected next event cycles: got %d want 14", cycles)
	}

	mfp.Advance(13)
	cycles, ok = mfp.NextEventCycles()
	if !ok {
		t.Fatalf("expected enabled timer to keep reporting a next event")
	}
	if cycles != 1 {
		t.Fatalf("unexpected next event after partial advance: got %d want 1", cycles)
	}
}

func TestMFPSoftwareEOIPreventsDuplicateTimerDispatchBeforeServiceClear(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x48); err != nil {
		t.Fatalf("write vector base with software eoi: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x20); err != nil {
		t.Fatalf("write interrupt enable: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x20); err != nil {
		t.Fatalf("write interrupt mask: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDR, 1); err != nil {
		t.Fatalf("write timer c data: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpTCDCR, 0x10); err != nil {
		t.Fatalf("write timer cd control: %v", err)
	}

	mfp.Advance(14)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected first timer c interrupt, got %d", len(irqs))
	}

	mfp.Advance(14)
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected duplicate timer c interrupt to stay blocked until service clear, got %d", len(irqs))
	}

	if err := mfp.Write(1, mfpBase+mfpISRB, 0xDF); err != nil {
		t.Fatalf("clear timer c in-service bit: %v", err)
	}

	irqs = mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected pending timer c interrupt after service clear, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x45 {
		t.Fatalf("unexpected vector after service clear: %+v", irqs[0].Vector)
	}
}

func TestMFPGPIPBit4ReflectsACIAInterruptLine(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	idle, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read idle GPIP: %v", err)
	}
	if byte(idle)&0x10 == 0 {
		t.Fatalf("expected idle ACIA line to read high, GPIP=%02x", byte(idle))
	}

	mfp.SetACIAInterrupt(true)
	active, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read active GPIP: %v", err)
	}
	if byte(active)&0x10 != 0 {
		t.Fatalf("expected asserted ACIA line to read low, GPIP=%02x", byte(active))
	}

	mfp.SetACIAInterrupt(false)
	cleared, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read cleared GPIP: %v", err)
	}
	if byte(cleared)&0x10 == 0 {
		t.Fatalf("expected cleared ACIA line to read high, GPIP=%02x", byte(cleared))
	}
}

func TestMFPGPIPBit0ReflectsPrinterBusyLine(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})

	ready, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read ready GPIP: %v", err)
	}
	if byte(ready)&0x01 != 0 {
		t.Fatalf("expected ready printer BUSY line to read low, GPIP=%02x", byte(ready))
	}

	mfp.SetPrinterBusy(true)
	busy, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read busy GPIP: %v", err)
	}
	if byte(busy)&0x01 == 0 {
		t.Fatalf("expected busy printer BUSY line to read high, GPIP=%02x", byte(busy))
	}

	mfp.SetPrinterBusy(false)
	cleared, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read cleared GPIP: %v", err)
	}
	if byte(cleared)&0x01 != 0 {
		t.Fatalf("expected cleared printer BUSY line to read low, GPIP=%02x", byte(cleared))
	}
}

func TestMFPGPIPBit7ReflectsMonitorType(t *testing.T) {
	cfg := config.Config{ClockHz: 8_000_000, ColorMonitor: false}
	mfp := NewMFP(&cfg)

	mono, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read mono GPIP: %v", err)
	}
	if byte(mono)&0x80 != 0 {
		t.Fatalf("expected monochrome monitor to clear GPIP bit 7, GPIP=%02x", byte(mono))
	}

	cfg.ColorMonitor = true
	color, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read color GPIP: %v", err)
	}
	if byte(color)&0x80 == 0 {
		t.Fatalf("expected color monitor to set GPIP bit 7, GPIP=%02x", byte(color))
	}
}

func TestMFPGPIPDDRSelectsOutputLatchOverExternalInputs(t *testing.T) {
	cfg := config.Config{ClockHz: 8_000_000, ColorMonitor: false}
	mfp := NewMFP(&cfg)

	if err := mfp.Write(1, mfpBase+mfpGPIP, 0x80); err != nil {
		t.Fatalf("write GPIP latch: %v", err)
	}
	input, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read input GPIP: %v", err)
	}
	if byte(input)&0x80 != 0 {
		t.Fatalf("expected DDR input bit 7 to ignore output latch and follow mono monitor low, GPIP=%02x", byte(input))
	}

	if err := mfp.Write(1, mfpBase+mfpDDR, 0x80); err != nil {
		t.Fatalf("write DDR: %v", err)
	}
	output, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read output GPIP: %v", err)
	}
	if byte(output)&0x80 == 0 {
		t.Fatalf("expected DDR output bit 7 to read GPIP latch high, GPIP=%02x", byte(output))
	}

	if err := mfp.Write(1, mfpBase+mfpGPIP, 0x00); err != nil {
		t.Fatalf("clear GPIP latch: %v", err)
	}
	output, err = mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read cleared output GPIP: %v", err)
	}
	if byte(output)&0x80 != 0 {
		t.Fatalf("expected DDR output bit 7 to read GPIP latch low, GPIP=%02x", byte(output))
	}
}

func TestMFPGPIPDDRInputBitsIgnoreOutputLatchForACIAAndRTC(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})
	rtc := NewICDRTC()
	mfp.AttachICDRTC(rtc)

	if err := mfp.Write(1, mfpBase+mfpGPIP, 0x30); err != nil {
		t.Fatalf("write GPIP latch: %v", err)
	}
	mfp.SetACIAInterrupt(true)
	rtc.HandleCommand(icdRTCCmdBegin)

	input, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read input GPIP: %v", err)
	}
	if byte(input)&0x30 != 0 {
		t.Fatalf("expected input ACIA/RTC bits to follow active-low external lines, GPIP=%02x", byte(input))
	}

	if err := mfp.Write(1, mfpBase+mfpDDR, 0x30); err != nil {
		t.Fatalf("write DDR: %v", err)
	}
	output, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read output GPIP: %v", err)
	}
	if byte(output)&0x30 != 0x30 {
		t.Fatalf("expected output ACIA/RTC bits to read high from GPIP latch, GPIP=%02x", byte(output))
	}
}

func TestMFPGPIPAERDefaultDetectsFallingACIAEdge(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x40); err != nil {
		t.Fatalf("enable ACIA GPIP interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x40); err != nil {
		t.Fatalf("mask ACIA GPIP interrupt: %v", err)
	}

	mfp.SetACIAInterrupt(true)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected falling ACIA edge interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x46 {
		t.Fatalf("unexpected falling ACIA edge vector: %+v", irqs[0].Vector)
	}

	mfp.SetACIAInterrupt(false)
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected rising ACIA edge to be ignored by default AER, got %d", len(irqs))
	}
}

func TestMFPGPIPAERDetectsRisingACIAEdgeWhenConfigured(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpAER, 0x10); err != nil {
		t.Fatalf("write AER: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x40); err != nil {
		t.Fatalf("enable ACIA GPIP interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x40); err != nil {
		t.Fatalf("mask ACIA GPIP interrupt: %v", err)
	}

	mfp.SetACIAInterrupt(true)
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected falling ACIA edge to be ignored by rising AER, got %d", len(irqs))
	}

	mfp.SetACIAInterrupt(false)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected rising ACIA edge interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x46 {
		t.Fatalf("unexpected rising ACIA edge vector: %+v", irqs[0].Vector)
	}
}

func TestMFPGPIPAERIgnoresEdgesOnDDRConfiguredOutputs(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})

	if err := mfp.Write(1, mfpBase+mfpDDR, 0x10); err != nil {
		t.Fatalf("write DDR: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x40); err != nil {
		t.Fatalf("enable ACIA GPIP interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x40); err != nil {
		t.Fatalf("mask ACIA GPIP interrupt: %v", err)
	}

	mfp.SetACIAInterrupt(true)
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected DDR output ACIA edge to be ignored, got %d", len(irqs))
	}

	if err := mfp.Write(1, mfpBase+mfpDDR, 0x00); err != nil {
		t.Fatalf("clear DDR: %v", err)
	}
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected no latent ACIA edge after returning bit to input, got %d", len(irqs))
	}
}

func TestMFPGPIPAERDetectsFallingRTCEdgeOnGPIP5(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})
	rtc := NewICDRTC()
	mfp.AttachICDRTC(rtc)

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERB, 0x80); err != nil {
		t.Fatalf("enable RTC GPIP interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRB, 0x80); err != nil {
		t.Fatalf("mask RTC GPIP interrupt: %v", err)
	}

	rtc.HandleCommand(icdRTCCmdBegin)
	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected falling RTC edge interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x47 {
		t.Fatalf("unexpected falling RTC edge vector: %+v", irqs[0].Vector)
	}

	rtc.HandleCommand(icdRTCCmdEnd)
	if irqs := mfp.DrainInterrupts(); len(irqs) != 0 {
		t.Fatalf("expected rising RTC edge to be ignored by default AER, got %d", len(irqs))
	}
}

func TestMFPGPIPBit5DefaultsHighWithoutICDRTC(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})

	value, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read GPIP: %v", err)
	}
	if byte(value)&0x20 == 0 {
		t.Fatalf("expected GPIP bit 5 to idle high, GPIP=%02x", byte(value))
	}
}

func TestMFPGPIPBit5ReflectsFDCInterruptLine(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})

	idle, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read idle GPIP: %v", err)
	}
	if byte(idle)&0x20 == 0 {
		t.Fatalf("expected idle FDC line to read high, GPIP=%02x", byte(idle))
	}

	mfp.SetFDCInterrupt(true)
	active, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read active GPIP: %v", err)
	}
	if byte(active)&0x20 != 0 {
		t.Fatalf("expected active FDC line to read low, GPIP=%02x", byte(active))
	}

	mfp.SetFDCInterrupt(false)
	cleared, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read cleared GPIP: %v", err)
	}
	if byte(cleared)&0x20 == 0 {
		t.Fatalf("expected cleared FDC line to read high, GPIP=%02x", byte(cleared))
	}
}

func TestMFPGPIPBit5ReflectsICDRTCSession(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000, ColorMonitor: false})
	rtc := NewICDRTC()
	mfp.AttachICDRTC(rtc)

	idle, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read idle GPIP: %v", err)
	}
	if byte(idle)&0x20 == 0 {
		t.Fatalf("expected idle RTC line to read high, GPIP=%02x", byte(idle))
	}

	rtc.HandleCommand(icdRTCCmdBegin)
	active, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read active GPIP: %v", err)
	}
	if byte(active)&0x20 != 0 {
		t.Fatalf("expected active RTC line to read low, GPIP=%02x", byte(active))
	}

	rtc.HandleCommand(icdRTCCmdEnd)
	cleared, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read cleared GPIP: %v", err)
	}
	if byte(cleared)&0x20 == 0 {
		t.Fatalf("expected ended RTC line to read high, GPIP=%02x", byte(cleared))
	}
}

func TestMFPRS232ReceiveByteSetsStatusAndInterrupt(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})
	mfp.AttachRS232(NewRS232())

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERA, 0x10); err != nil {
		t.Fatalf("enable USART receive interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRA, 0x10); err != nil {
		t.Fatalf("mask USART receive interrupt: %v", err)
	}

	mfp.PushRS232Input([]byte("A"))

	status, err := mfp.Read(1, mfpBase+mfpRSR)
	if err != nil {
		t.Fatalf("read RSR: %v", err)
	}
	if byte(status)&mfpRSRBufferFull == 0 {
		t.Fatalf("expected RSR buffer-full bit, got %02x", byte(status))
	}

	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected receive interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x4C {
		t.Fatalf("unexpected receive vector: %+v", irqs[0].Vector)
	}

	data, err := mfp.Read(1, mfpBase+mfpUDR)
	if err != nil {
		t.Fatalf("read UDR: %v", err)
	}
	if byte(data) != 'A' {
		t.Fatalf("unexpected received byte: got %q want A", byte(data))
	}

	status, err = mfp.Read(1, mfpBase+mfpRSR)
	if err != nil {
		t.Fatalf("read RSR after UDR: %v", err)
	}
	if byte(status)&mfpRSRBufferFull != 0 {
		t.Fatalf("expected RSR buffer-full bit to clear, got %02x", byte(status))
	}
}

func TestMFPRS232ReceiveQueuesMultipleBytes(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})
	mfp.AttachRS232(NewRS232())
	if err := mfp.Write(1, mfpBase+mfpIERA, 0x10); err != nil {
		t.Fatalf("enable USART receive interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRA, 0x10); err != nil {
		t.Fatalf("mask USART receive interrupt: %v", err)
	}

	mfp.PushRS232Input([]byte("AB"))

	first, err := mfp.Read(1, mfpBase+mfpUDR)
	if err != nil {
		t.Fatalf("read first UDR: %v", err)
	}
	if byte(first) != 'A' {
		t.Fatalf("first received byte = %q, want A", byte(first))
	}

	status, err := mfp.Read(1, mfpBase+mfpRSR)
	if err != nil {
		t.Fatalf("read RSR after first UDR: %v", err)
	}
	if byte(status)&mfpRSRBufferFull == 0 {
		t.Fatalf("expected second queued byte to set RSR, got %02x", byte(status))
	}

	second, err := mfp.Read(1, mfpBase+mfpUDR)
	if err != nil {
		t.Fatalf("read second UDR: %v", err)
	}
	if byte(second) != 'B' {
		t.Fatalf("second received byte = %q, want B", byte(second))
	}
}

func TestMFPRS232TransmitCapturesOutputAndInterrupts(t *testing.T) {
	mfp := NewMFP(&config.Config{ClockHz: 8_000_000})
	rs232 := NewRS232()
	mfp.AttachRS232(rs232)

	if err := mfp.Write(1, mfpBase+mfpVR, 0x40); err != nil {
		t.Fatalf("write vector base: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIERA, 0x04); err != nil {
		t.Fatalf("enable USART transmit interrupt: %v", err)
	}
	if err := mfp.Write(1, mfpBase+mfpIMRA, 0x04); err != nil {
		t.Fatalf("mask USART transmit interrupt: %v", err)
	}

	if err := mfp.Write(1, mfpBase+mfpUDR, 'Z'); err != nil {
		t.Fatalf("write UDR: %v", err)
	}
	if got, want := string(rs232.Output()), "Z"; got != want {
		t.Fatalf("RS232 output = %q, want %q", got, want)
	}

	status, err := mfp.Read(1, mfpBase+mfpTSR)
	if err != nil {
		t.Fatalf("read TSR: %v", err)
	}
	if byte(status)&mfpTSRBufferEmpty == 0 {
		t.Fatalf("expected TSR buffer-empty bit, got %02x", byte(status))
	}

	irqs := mfp.DrainInterrupts()
	if len(irqs) != 1 {
		t.Fatalf("expected transmit interrupt, got %d", len(irqs))
	}
	if irqs[0].Vector == nil || *irqs[0].Vector != 0x4A {
		t.Fatalf("unexpected transmit vector: %+v", irqs[0].Vector)
	}
}

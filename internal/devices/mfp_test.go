package devices

import "testing"

func TestMFPTimerQueuesInterrupt(t *testing.T) {
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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
	mfp := NewMFP(8_000_000)

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

func TestMFPGPIPBit7ReflectsMonitorType(t *testing.T) {
	mfp := NewMFP(8_000_000)

	mono, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read mono GPIP: %v", err)
	}
	if byte(mono)&0x80 != 0 {
		t.Fatalf("expected monochrome monitor to clear GPIP bit 7, GPIP=%02x", byte(mono))
	}

	mfp.SetColorMonitor(true)
	color, err := mfp.Read(1, mfpBase+mfpGPIP)
	if err != nil {
		t.Fatalf("read color GPIP: %v", err)
	}
	if byte(color)&0x80 == 0 {
		t.Fatalf("expected color monitor to set GPIP bit 7, GPIP=%02x", byte(color))
	}
}

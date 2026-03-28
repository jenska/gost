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

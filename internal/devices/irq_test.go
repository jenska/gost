package devices

// irqLine is the level-sensitive interrupt line a peripheral drives onto the
// CPU. The machine aggregates these; the unit tests below use drainIRQ.
type irqLine interface {
	PendingIRQ() (level, vector uint8)
	AckIRQ(level uint8)
}

// drainIRQ takes the interrupt a device's line is currently asserting and
// lowers it, the way the machine's old DrainInterrupts helper did. It returns
// nil when the line is idle.
func drainIRQ(line irqLine) []Interrupt {
	level, vector := line.PendingIRQ()
	if level == 0 {
		return nil
	}
	line.AckIRQ(level)
	return []Interrupt{{Level: level, Vector: vector}}
}

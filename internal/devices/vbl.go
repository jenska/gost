package devices

// VBLSource generates the ST's 50 Hz vertical blank autovector interrupt.
type VBLSource struct {
	cyclesPerVBL uint64
	counter      uint64
	pending      []Interrupt
}

func NewVBLSource(clockHz, frameHz uint64) *VBLSource {
	if frameHz == 0 {
		frameHz = 50
	}
	cyclesPerVBL := clockHz / frameHz
	if cyclesPerVBL == 0 {
		cyclesPerVBL = 1
	}
	return &VBLSource{
		cyclesPerVBL: cyclesPerVBL,
		counter:      cyclesPerVBL,
	}
}

func (v *VBLSource) Advance(cycles uint64) {
	for cycles >= v.counter {
		cycles -= v.counter
		v.counter = v.cyclesPerVBL
		v.pending = append(v.pending, Interrupt{Level: 4, Vector: nil})
	}
	v.counter -= cycles
}

func (v *VBLSource) DrainInterrupts() []Interrupt {
	if len(v.pending) == 0 {
		return nil
	}
	out := append([]Interrupt(nil), v.pending...)
	v.pending = v.pending[:0]
	return out
}

package devices

// MIDI is the byte endpoint behind ACIA channel 1. Timing remains ACIA-level
// for now; this device only owns host-injected input and guest-transmitted
// output bytes.
type MIDI struct {
	input  []byte
	output []byte
}

func NewMIDI() *MIDI {
	return &MIDI{}
}

func (m *MIDI) PushInput(data []byte) {
	if len(data) == 0 {
		return
	}
	m.input = append(m.input, data...)
}

func (m *MIDI) PopInput() (byte, bool) {
	if len(m.input) == 0 {
		return 0, false
	}
	value := m.input[0]
	copy(m.input, m.input[1:])
	m.input = m.input[:len(m.input)-1]
	return value, true
}

func (m *MIDI) InputAvailable() bool {
	return len(m.input) != 0
}

func (m *MIDI) WriteOutput(value byte) {
	m.output = append(m.output, value)
}

func (m *MIDI) Output() []byte {
	if len(m.output) == 0 {
		return nil
	}
	return append([]byte(nil), m.output...)
}

func (m *MIDI) ClearOutput() {
	m.output = m.output[:0]
}

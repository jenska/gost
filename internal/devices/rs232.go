package devices

// RS232 is the host-facing endpoint behind the ST's MFP USART. It intentionally
// stores bytes, not line-level timing: the MFP owns register status and
// interrupt delivery, while this device provides deterministic input/output
// buffers for tests and frontend integrations.
type RS232 struct {
	input  []byte
	output []byte
}

func NewRS232() *RS232 {
	return &RS232{}
}

func (r *RS232) PushInput(data []byte) {
	if len(data) == 0 {
		return
	}
	r.input = append(r.input, data...)
}

func (r *RS232) PopInput() (byte, bool) {
	if len(r.input) == 0 {
		return 0, false
	}
	value := r.input[0]
	copy(r.input, r.input[1:])
	r.input = r.input[:len(r.input)-1]
	return value, true
}

func (r *RS232) InputAvailable() bool {
	return len(r.input) != 0
}

func (r *RS232) WriteOutput(value byte) {
	r.output = append(r.output, value)
}

func (r *RS232) Output() []byte {
	if len(r.output) == 0 {
		return nil
	}
	return append([]byte(nil), r.output...)
}

func (r *RS232) ClearOutput() {
	r.output = r.output[:0]
}

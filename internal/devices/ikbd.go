package devices

// IKBD implements a small queue-based model of the keyboard controller.
type IKBD struct {
	queue []byte
}

func NewIKBD() *IKBD {
	return &IKBD{}
}

func (i *IKBD) Reset() {
	i.queue = i.queue[:0]
}

func (i *IKBD) PushKey(scancode byte, pressed bool) {
	if pressed {
		i.queue = append(i.queue, scancode&0x7F)
		return
	}
	i.queue = append(i.queue, scancode|0x80)
}

func (i *IKBD) PushMouse(dx, dy int, buttons byte) {
	i.queue = append(i.queue, 0xF8|(buttons&0x03), byte(dx), byte(dy))
}

func (i *IKBD) HandleCommand(cmd byte) {
	// Minimal command handling keeps the interface predictable for bring-up.
	switch cmd {
	case 0x80:
		i.queue = append(i.queue, 0xF6)
	case 0x08:
		i.queue = append(i.queue, 0xF7)
	}
}

func (i *IKBD) HasData() bool {
	return len(i.queue) > 0
}

func (i *IKBD) ReadByte() (byte, bool) {
	if len(i.queue) == 0 {
		return 0, false
	}
	value := i.queue[0]
	i.queue = i.queue[1:]
	return value, true
}

package devices

// IKBD implements a small queue-based model of the keyboard controller.
type IKBD struct {
	queue     []byte
	command   [6]byte
	commandAt int
	remaining int
}

func NewIKBD() *IKBD {
	return &IKBD{}
}

func (i *IKBD) Reset() {
	i.queue = i.queue[:0]
	i.commandAt = 0
	i.remaining = 0
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
	if i.commandAt == 0 {
		i.command[0] = cmd
		i.commandAt = 1
		i.remaining = ikbdCommandExtraBytes(cmd)
		if i.remaining > 0 {
			return
		}
	} else {
		i.command[i.commandAt] = cmd
		i.commandAt++
		i.remaining--
		if i.remaining > 0 {
			return
		}
	}

	switch i.command[0] {
	case 0x80:
		if i.commandAt >= 2 && i.command[1] == 0x01 {
			i.queue = append(i.queue, 0xF1)
		}
	}

	i.commandAt = 0
	i.remaining = 0
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

func ikbdCommandExtraBytes(cmd byte) int {
	switch cmd {
	case 0x07, 0x17, 0x80:
		return 1
	case 0x0A, 0x0B, 0x0C, 0x21, 0x22:
		return 2
	case 0x20:
		return 3
	case 0x09:
		return 4
	case 0x0E:
		return 5
	case 0x19, 0x1B:
		return 6
	default:
		return 0
	}
}

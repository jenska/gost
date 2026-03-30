package devices

import "io"

type ikbdMouseMode byte

const (
	ikbdMouseModeRelative ikbdMouseMode = iota
	ikbdMouseModeAbsolute
)

// IKBD implements a small queue-based model of the keyboard controller.
type IKBD struct {
	queue            []byte
	command          [6]byte
	commandAt        int
	remaining        int
	mouseDisabled    bool
	outputPaused     bool
	invertMouseY     bool
	lastMouseButtons byte
	mouseMode        ikbdMouseMode
	absX             uint16
	absY             uint16
	absMaxX          uint16
	absMaxY          uint16
	absScaleX        uint8
	absScaleY        uint8
	absButtons       byte
}

func NewIKBD() *IKBD {
	return &IKBD{}
}

func (i *IKBD) Reset() {
	i.queue = i.queue[:0]
	i.commandAt = 0
	i.remaining = 0
	i.mouseDisabled = false
	i.outputPaused = false
	i.invertMouseY = false
	i.lastMouseButtons = 0
	i.mouseMode = ikbdMouseModeRelative
	i.absX = 0
	i.absY = 0
	i.absMaxX = 0
	i.absMaxY = 0
	i.absScaleX = 1
	i.absScaleY = 1
	i.absButtons = 0
}

func (i *IKBD) PushKey(scancode byte, pressed bool) {
	if pressed {
		i.queue = append(i.queue, scancode&0x7F)
		return
	}
	i.queue = append(i.queue, scancode|0x80)
}

func (i *IKBD) PushMouse(dx, dy int, buttons byte) {
	if i.outputPaused {
		return
	}
	if i.mouseDisabled {
		i.updateAbsoluteButtons(buttons & 0x03)
		return
	}
	if i.invertMouseY {
		dy = -dy
	}

	buttons &= 0x03
	switch i.mouseMode {
	case ikbdMouseModeAbsolute:
		i.updateAbsoluteButtons(buttons)
		i.absX = clampAbsoluteCoordinate(i.absX, dx, i.absMaxX, i.absScaleX)
		i.absY = clampAbsoluteCoordinate(i.absY, dy, i.absMaxY, i.absScaleY)
	default:
		for dx != 0 || dy != 0 || buttons != i.lastMouseButtons {
			stepX := clampMouseDelta(dx)
			stepY := clampMouseDelta(dy)
			i.queue = append(i.queue, 0xF8|buttons, byte(int8(stepX)), byte(int8(stepY)))
			i.lastMouseButtons = buttons
			dx -= stepX
			dy -= stepY
		}
	}
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
	case 0x08:
		i.mouseDisabled = false
		i.outputPaused = false
		i.mouseMode = ikbdMouseModeRelative
	case 0x09:
		i.mouseDisabled = false
		i.outputPaused = false
		i.mouseMode = ikbdMouseModeAbsolute
		i.absMaxX = uint16(i.command[1])<<8 | uint16(i.command[2])
		i.absMaxY = uint16(i.command[3])<<8 | uint16(i.command[4])
		i.absX = 0
		i.absY = 0
		i.absButtons = 0
	case 0x0C:
		i.absScaleX = clampMouseScale(i.command[1])
		i.absScaleY = clampMouseScale(i.command[2])
	case 0x0D:
		if i.mouseMode == ikbdMouseModeAbsolute {
			i.queueAbsolutePosition()
		}
	case 0x0E:
		i.absX = uint16(i.command[2])<<8 | uint16(i.command[3])
		i.absY = uint16(i.command[4])<<8 | uint16(i.command[5])
		if i.absX > i.absMaxX {
			i.absX = i.absMaxX
		}
		if i.absY > i.absMaxY {
			i.absY = i.absMaxY
		}
	case 0x0F:
		i.invertMouseY = true
	case 0x10:
		i.invertMouseY = false
	case 0x11:
		i.outputPaused = false
	case 0x12:
		i.mouseDisabled = true
	case 0x13:
		i.outputPaused = true
	case 0x80:
		if i.commandAt >= 2 && i.command[1] == 0x01 {
			i.outputPaused = false
			i.mouseDisabled = false
			i.invertMouseY = false
			i.lastMouseButtons = 0
			i.mouseMode = ikbdMouseModeRelative
			i.absX = 0
			i.absY = 0
			i.absMaxX = 0
			i.absMaxY = 0
			i.absScaleX = 1
			i.absScaleY = 1
			i.absButtons = 0
			i.queue = append(i.queue, 0xF1)
		}
	}

	i.commandAt = 0
	i.remaining = 0
}

func (i *IKBD) HasData() bool {
	return len(i.queue) > 0
}

func (i *IKBD) ReadByte() (byte, error) {
	if len(i.queue) == 0 {
		return 0, io.EOF
	}
	value := i.queue[0]
	i.queue = i.queue[1:]
	return value, nil
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

func clampMouseDelta(delta int) int {
	switch {
	case delta > 127:
		return 127
	case delta < -128:
		return -128
	default:
		return delta
	}
}

func clampMouseScale(value byte) uint8 {
	if value == 0 {
		return 1
	}
	return uint8(value)
}

func clampAbsoluteCoordinate(current uint16, delta int, max uint16, scale uint8) uint16 {
	if delta == 0 {
		return current
	}
	step := delta / int(scale)
	if step == 0 {
		if delta > 0 {
			step = 1
		} else {
			step = -1
		}
	}
	next := int(current) + step
	if next < 0 {
		next = 0
	}
	if next > int(max) {
		next = int(max)
	}
	return uint16(next)
}

func (i *IKBD) updateAbsoluteButtons(buttons byte) {
	const (
		rightDown = 0x01
		rightUp   = 0x02
		leftDown  = 0x04
		leftUp    = 0x08
	)
	prev := i.lastMouseButtons
	if prev&0x01 == 0 && buttons&0x01 != 0 {
		i.absButtons |= rightDown
	}
	if prev&0x01 != 0 && buttons&0x01 == 0 {
		i.absButtons |= rightUp
	}
	if prev&0x02 == 0 && buttons&0x02 != 0 {
		i.absButtons |= leftDown
	}
	if prev&0x02 != 0 && buttons&0x02 == 0 {
		i.absButtons |= leftUp
	}
	i.lastMouseButtons = buttons
}

func (i *IKBD) queueAbsolutePosition() {
	i.queue = append(
		i.queue,
		0xF7,
		i.absButtons,
		byte(i.absX>>8),
		byte(i.absX),
		byte(i.absY>>8),
		byte(i.absY),
	)
	i.absButtons = 0
}

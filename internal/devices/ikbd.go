package devices

import (
	"io"
)

// ikbdMouseMode controls whether mouse motion is reported as deltas or as an
// absolute position.
type ikbdMouseMode byte

const (
	ikbdMouseModeRelative ikbdMouseMode = iota
	ikbdMouseModeAbsolute
)

const (
	// Supported IKBD command bytes used by the emulator's command parser.
	ikbdCmdSetRelativeMouseMode byte = 0x08
	ikbdCmdSetAbsoluteMouseMode byte = 0x09
	ikbdCmdSetMouseScale        byte = 0x0C
	ikbdCmdInterrogateMouse     byte = 0x0D
	ikbdCmdLoadMousePosition    byte = 0x0E
	ikbdCmdSetYBottomUp         byte = 0x0F
	ikbdCmdSetYTopDown          byte = 0x10
	ikbdCmdResumeOutput         byte = 0x11
	ikbdCmdDisableMouse         byte = 0x12
	ikbdCmdPauseOutput          byte = 0x13
	ikbdCmdReset                byte = 0x80
)

const (
	// IKBD packet and reply bytes emitted back to the ACIA.
	ikbdReplySelfTestOK         byte = 0xF1
	ikbdPacketAbsolutePosition  byte = 0xF7
	ikbdPacketRelativeMouseBase byte = 0xF8
)

// IKBD implements a small queue-based model of the keyboard controller.
type IKBD struct {
	queue     []byte
	command   [7]byte
	commandAt int
	remaining int
	// mouseDisabled suppresses motion packets while still allowing stateful
	// command handling.
	mouseDisabled bool
	// outputPaused temporarily stops outbound mouse data.
	outputPaused bool
	// invertMouseY flips the Y axis for subsequent host mouse motion.
	invertMouseY     bool
	lastMouseButtons byte
	// mouseMode selects relative or absolute mouse reporting.
	mouseMode ikbdMouseMode
	absX      uint16
	absY      uint16
	absMaxX   uint16
	absMaxY   uint16
	// absScale* scales host mouse deltas before applying them to absolute
	// coordinates.
	absScaleX  uint8
	absScaleY  uint8
	absButtons byte
}

// NewIKBD constructs an IKBD in its default power-on state.
func NewIKBD() *IKBD {
	return &IKBD{}
}

// Reset clears queued data and restores the controller state to defaults.
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

// PushKey enqueues a make or break scancode for delivery over the ACIA link.
func (i *IKBD) PushKey(scancode byte, pressed bool) {
	if pressed {
		i.queue = append(i.queue, scancode&0x7F)
		return
	}
	i.queue = append(i.queue, scancode|0x80)
}

// PushMouse converts host mouse movement into IKBD packets for the current
// reporting mode.
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
			i.queue = append(i.queue, ikbdPacketRelativeMouseBase|buttons, byte(int8(stepX)), byte(int8(stepY)))
			i.lastMouseButtons = buttons
			dx -= stepX
			dy -= stepY
		}
	}
}

// HandleCommand accumulates command bytes until the selected IKBD command has
// all of its parameters, then applies the supported behavior.
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

	// Only the subset of IKBD commands needed by the emulator is modeled here.
	switch i.command[0] {
	case ikbdCmdSetRelativeMouseMode:
		// Return to the default streaming mouse mode.
		i.mouseDisabled = false
		i.outputPaused = false
		i.mouseMode = ikbdMouseModeRelative
	case ikbdCmdSetAbsoluteMouseMode:
		// Switch to absolute reporting and reset the tracked tablet position.
		i.mouseDisabled = false
		i.outputPaused = false
		i.mouseMode = ikbdMouseModeAbsolute
		i.absMaxX = uint16(i.command[1])<<8 | uint16(i.command[2])
		i.absMaxY = uint16(i.command[3])<<8 | uint16(i.command[4])
		i.absX = 0
		i.absY = 0
		i.absButtons = 0
	case ikbdCmdSetMouseScale:
		// Apply host-to-IKBD scaling for absolute position updates.
		i.absScaleX = clampMouseScale(i.command[1])
		i.absScaleY = clampMouseScale(i.command[2])
	case ikbdCmdInterrogateMouse:
		// Report the current absolute position when absolute mode is active.
		if i.mouseMode == ikbdMouseModeAbsolute {
			i.queueAbsolutePosition()
		}
	case ikbdCmdLoadMousePosition:
		// Seed the absolute pointer position from the command payload.
		i.absX = uint16(i.command[2])<<8 | uint16(i.command[3])
		i.absY = uint16(i.command[4])<<8 | uint16(i.command[5])
		if i.absX > i.absMaxX {
			i.absX = i.absMaxX
		}
		if i.absY > i.absMaxY {
			i.absY = i.absMaxY
		}
	case ikbdCmdSetYBottomUp:
		i.invertMouseY = true
	case ikbdCmdSetYTopDown:
		i.invertMouseY = false
	case ikbdCmdResumeOutput:
		i.outputPaused = false
	case ikbdCmdDisableMouse:
		i.mouseDisabled = true
	case ikbdCmdPauseOutput:
		i.outputPaused = true
	case ikbdCmdReset:
		// `0x80 0x01` performs the reset sequence expected during startup.
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
			i.queue = append(i.queue, ikbdReplySelfTestOK)
		}
	}

	i.commandAt = 0
	i.remaining = 0
}

// HasData reports whether a byte is ready to be read by the ACIA.
func (i *IKBD) HasData() bool {
	return len(i.queue) > 0
}

// ReadByte pops the next queued IKBD byte.
func (i *IKBD) ReadByte() (byte, error) {
	if len(i.queue) == 0 {
		return 0, io.EOF
	}
	value := i.queue[0]
	i.queue = i.queue[1:]
	return value, nil
}

// ikbdCommandExtraBytes returns how many parameter bytes follow a command byte.
func ikbdCommandExtraBytes(cmd byte) int {
	switch cmd {
	case 0x07, 0x17, ikbdCmdReset:
		return 1
	case 0x0A, 0x0B, ikbdCmdSetMouseScale, 0x21, 0x22:
		return 2
	case 0x20:
		return 3
	case ikbdCmdSetAbsoluteMouseMode:
		return 4
	case ikbdCmdLoadMousePosition:
		return 5
	case 0x19, 0x1B:
		return 6
	default:
		return 0
	}
}

// clampMouseDelta limits relative mouse motion to the signed 8-bit packet
// range used by the IKBD.
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

// clampMouseScale treats a zero scale factor as 1 so absolute mode still
// advances.
func clampMouseScale(value byte) uint8 {
	if value == 0 {
		return 1
	}
	return uint8(value)
}

// clampAbsoluteCoordinate applies a scaled delta and clamps the result to the
// configured absolute axis range.
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
		ikbdPacketAbsolutePosition,
		i.absButtons,
		byte(i.absX>>8),
		byte(i.absX),
		byte(i.absY>>8),
		byte(i.absY),
	)
	i.absButtons = 0
}

package inputmap

type Key int

const (
	KeyUnknown Key = iota
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeySpace
	KeyEnter
	KeyNumpadEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeyShiftLeft
	KeyShiftRight
	KeyControlLeft
	KeyControlRight
	KeyAltLeft
	KeyAltRight
	KeyCapsLock
	KeyMinus
	KeyEqual
	KeyBracketLeft
	KeyBracketRight
	KeySemicolon
	KeyQuote
	KeyBackquote
	KeyBackslash
	KeyComma
	KeyPeriod
	KeySlash
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyHome
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
)

func AtariScancode(key Key) (byte, bool) {
	switch key {
	case KeyA:
		return 0x1E, true
	case KeyB:
		return 0x30, true
	case KeyC:
		return 0x2E, true
	case KeyD:
		return 0x20, true
	case KeyE:
		return 0x12, true
	case KeyF:
		return 0x21, true
	case KeyG:
		return 0x22, true
	case KeyH:
		return 0x23, true
	case KeyI:
		return 0x17, true
	case KeyJ:
		return 0x24, true
	case KeyK:
		return 0x25, true
	case KeyL:
		return 0x26, true
	case KeyM:
		return 0x32, true
	case KeyN:
		return 0x31, true
	case KeyO:
		return 0x18, true
	case KeyP:
		return 0x19, true
	case KeyQ:
		return 0x10, true
	case KeyR:
		return 0x13, true
	case KeyS:
		return 0x1F, true
	case KeyT:
		return 0x14, true
	case KeyU:
		return 0x16, true
	case KeyV:
		return 0x2F, true
	case KeyW:
		return 0x11, true
	case KeyX:
		return 0x2D, true
	case KeyY:
		return 0x15, true
	case KeyZ:
		return 0x2C, true
	case Key0:
		return 0x0B, true
	case Key1:
		return 0x02, true
	case Key2:
		return 0x03, true
	case Key3:
		return 0x04, true
	case Key4:
		return 0x05, true
	case Key5:
		return 0x06, true
	case Key6:
		return 0x07, true
	case Key7:
		return 0x08, true
	case Key8:
		return 0x09, true
	case Key9:
		return 0x0A, true
	case KeySpace:
		return 0x39, true
	case KeyEnter, KeyNumpadEnter:
		return 0x1C, true
	case KeyEscape:
		return 0x01, true
	case KeyBackspace:
		return 0x0E, true
	case KeyTab:
		return 0x0F, true
	case KeyShiftLeft:
		return 0x2A, true
	case KeyShiftRight:
		return 0x36, true
	case KeyControlLeft, KeyControlRight:
		return 0x1D, true
	case KeyAltLeft, KeyAltRight:
		return 0x38, true
	case KeyCapsLock:
		return 0x3A, true
	case KeyMinus:
		return 0x0C, true
	case KeyEqual:
		return 0x0D, true
	case KeyBracketLeft:
		return 0x1A, true
	case KeyBracketRight:
		return 0x1B, true
	case KeySemicolon:
		return 0x27, true
	case KeyQuote:
		return 0x28, true
	case KeyBackquote:
		return 0x29, true
	case KeyBackslash:
		return 0x2B, true
	case KeyComma:
		return 0x33, true
	case KeyPeriod:
		return 0x34, true
	case KeySlash:
		return 0x35, true
	case KeyArrowUp:
		return 0x48, true
	case KeyArrowDown:
		return 0x50, true
	case KeyArrowLeft:
		return 0x4B, true
	case KeyArrowRight:
		return 0x4D, true
	case KeyHome:
		return 0x47, true
	case KeyInsert:
		return 0x52, true
	case KeyDelete:
		return 0x53, true
	case KeyF1:
		return 0x3B, true
	case KeyF2:
		return 0x3C, true
	case KeyF3:
		return 0x3D, true
	case KeyF4:
		return 0x3E, true
	case KeyF5:
		return 0x3F, true
	case KeyF6:
		return 0x40, true
	case KeyF7:
		return 0x41, true
	case KeyF8:
		return 0x42, true
	case KeyF9:
		return 0x43, true
	case KeyF10:
		return 0x44, true
	default:
		return 0, false
	}
}

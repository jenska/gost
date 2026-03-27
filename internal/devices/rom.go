package devices

import (
	"fmt"

	"github.com/jenska/m68kemu"
)

// ROM exposes immutable bytes at one or more aliased addresses.
type ROM struct {
	aliases []uint32
	data    []byte
}

func NewROM(data []byte, aliases ...uint32) *ROM {
	cloned := append([]byte(nil), data...)
	return &ROM{
		aliases: append([]uint32(nil), aliases...),
		data:    cloned,
	}
}

func (r *ROM) Contains(address uint32) bool {
	for _, base := range r.aliases {
		if address >= base && address < base+uint32(len(r.data)) {
			return true
		}
	}
	return false
}

func (r *ROM) WaitStates(m68kemu.Size, uint32) uint32 {
	return 4
}

func (r *ROM) offset(address uint32, size m68kemu.Size) (uint32, error) {
	for _, base := range r.aliases {
		if address < base {
			continue
		}
		offset := address - base
		if offset+uint32(size) <= uint32(len(r.data)) {
			return offset, nil
		}
	}
	return 0, m68kemu.BusError(address)
}

func (r *ROM) Read(size m68kemu.Size, address uint32) (uint32, error) {
	offset, err := r.offset(address, size)
	if err != nil {
		return 0, err
	}

	return r.readAtOffset(size, offset)
}

func (r *ROM) readAtOffset(size m68kemu.Size, offset uint32) (uint32, error) {
	if offset+uint32(size) > uint32(len(r.data)) {
		return 0, m68kemu.BusError(offset)
	}

	switch size {
	case m68kemu.Byte:
		return uint32(r.data[offset]), nil
	case m68kemu.Word:
		return uint32(readUint16BE(r.data, offset)), nil
	case m68kemu.Long:
		return readUint32BE(r.data, offset), nil
	default:
		return 0, fmt.Errorf("unsupported ROM read size %d", size)
	}
}

func (r *ROM) Write(m68kemu.Size, uint32, uint32) error {
	return nil
}

func (r *ROM) Reset() {}

func (r *ROM) Bytes() []byte {
	return append([]byte(nil), r.data...)
}

func (r *ROM) Slice(address uint32, size m68kemu.Size) (uint32, error) {
	return r.Read(size, address)
}

// OverlayROM exposes the reset vectors at low memory during reset.
type OverlayROM struct {
	rom     *ROM
	ram     *RAM
	enabled bool
}

func NewOverlayROM(rom *ROM, ram *RAM) *OverlayROM {
	return &OverlayROM{rom: rom, ram: ram, enabled: true}
}

func (o *OverlayROM) Contains(address uint32) bool {
	return o.enabled && address < 8
}

func (o *OverlayROM) Read(size m68kemu.Size, address uint32) (uint32, error) {
	return o.rom.readAtOffset(size, address)
}

func (o *OverlayROM) Write(size m68kemu.Size, address uint32, value uint32) error {
	return o.ram.Write(size, address, value)
}

func (o *OverlayROM) Reset() {
	o.enabled = true
}

func (o *OverlayROM) Enable() {
	o.enabled = true
}

func (o *OverlayROM) Disable() {
	o.enabled = false
}

func (o *OverlayROM) Enabled() bool {
	return o.enabled
}

const memoryConfigBase = 0xFF8000

// MemoryConfig lets ROM disable its reset-time low-memory overlay.
type MemoryConfig struct {
	overlay *OverlayROM
	value   byte
}

func NewMemoryConfig(overlay *OverlayROM) *MemoryConfig {
	return &MemoryConfig{overlay: overlay}
}

func (m *MemoryConfig) Contains(address uint32) bool {
	return address >= memoryConfigBase && address < memoryConfigBase+2
}

func (m *MemoryConfig) Read(size m68kemu.Size, address uint32) (uint32, error) {
	if size == m68kemu.Word {
		return uint32(m.value), nil
	}
	return uint32(m.value), nil
}

func (m *MemoryConfig) Write(size m68kemu.Size, address uint32, value uint32) error {
	m.value = byte(value)
	if address == memoryConfigBase+1 || size == m68kemu.Word {
		m.overlay.Disable()
	}
	return nil
}

func (m *MemoryConfig) Reset() {
	m.value = 0
	m.overlay.Enable()
}

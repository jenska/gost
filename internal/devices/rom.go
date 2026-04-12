package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
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

func (r *ROM) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (r *ROM) offset(address uint32, size cpu.Size) (uint32, error) {
	for _, base := range r.aliases {
		if address < base {
			continue
		}
		offset := address - base
		if offset+uint32(size) <= uint32(len(r.data)) {
			return offset, nil
		}
	}
	return 0, cpu.BusError(address)
}

func (r *ROM) Read(size cpu.Size, address uint32) (uint32, error) {
	offset, err := r.offset(address, size)
	if err != nil {
		return 0, err
	}

	return r.readAtOffset(size, offset)
}

func (r *ROM) Peek(size cpu.Size, address uint32) (uint32, error) {
	return r.Read(size, address)
}

func (r *ROM) readAtOffset(size cpu.Size, offset uint32) (uint32, error) {
	if offset+uint32(size) > uint32(len(r.data)) {
		return 0, cpu.BusError(offset)
	}

	switch size {
	case cpu.Byte:
		return uint32(r.data[offset]), nil
	case cpu.Word:
		return uint32(readUint16BE(r.data, offset)), nil
	case cpu.Long:
		return readUint32BE(r.data, offset), nil
	default:
		return 0, fmt.Errorf("unsupported ROM read size %d", size)
	}
}

func (r *ROM) Write(cpu.Size, uint32, uint32) error {
	return nil
}

func (r *ROM) Reset() {}

func (r *ROM) Bytes() []byte {
	return append([]byte(nil), r.data...)
}

func (r *ROM) Slice(address uint32, size cpu.Size) (uint32, error) {
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

func (o *OverlayROM) Read(size cpu.Size, address uint32) (uint32, error) {
	return o.rom.readAtOffset(size, address)
}

func (o *OverlayROM) Peek(size cpu.Size, address uint32) (uint32, error) {
	return o.Read(size, address)
}

func (o *OverlayROM) Write(size cpu.Size, address uint32, value uint32) error {
	return o.ram.Write(size, address, value)
}

func (o *OverlayROM) Reset() {
	// A CPU RESET should not remap the boot ROM over low memory again.
}

func (o *OverlayROM) ColdReset() {
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

const (
	mmuBankSize128K = 128 * 1024
	mmuBankSize512K = 512 * 1024
	mmuBankSize2M   = 2 * 1024 * 1024
)

// MemoryConfig lets ROM disable its reset-time low-memory overlay.
type MemoryConfig struct {
	overlay      *OverlayROM
	value        byte
	ramBank0Size uint32
	ramBank1Size uint32
	mmuBank0Size uint32
	mmuBank1Size uint32
}

type MemoryAddressState uint8

const (
	memoryAddressInvalid MemoryAddressState = iota
	memoryAddressPresent
	memoryAddressAbsent
)

func NewMemoryConfig(overlay *OverlayROM, ramSize uint32) *MemoryConfig {
	bank0, bank1, value := physicalBanksForRAM(ramSize)
	m := &MemoryConfig{
		overlay:      overlay,
		value:        value,
		ramBank0Size: bank0,
		ramBank1Size: bank1,
	}
	m.setValue(value)
	return m
}

func (m *MemoryConfig) Contains(address uint32) bool {
	return address >= memoryConfigBase && address < memoryConfigBase+2
}

func (m *MemoryConfig) Read(size cpu.Size, address uint32) (uint32, error) {
	if size == cpu.Word {
		return uint32(m.value), nil
	}
	return uint32(m.value), nil
}

func (m *MemoryConfig) Peek(size cpu.Size, address uint32) (uint32, error) {
	return m.Read(size, address)
}

func (m *MemoryConfig) Write(size cpu.Size, address uint32, value uint32) error {
	switch size {
	case cpu.Byte:
		m.setValue(byte(value))
	case cpu.Word:
		m.setValue(byte(value))
	default:
		m.setValue(byte(value))
	}
	if address == memoryConfigBase+1 || size == cpu.Word {
		m.overlay.Disable()
	}
	return nil
}

func (m *MemoryConfig) Reset() {
	// A CPU RESET does not restore the MMU power-on bank configuration.
}

func (m *MemoryConfig) ColdReset() {
	m.setValue(physicalConfigValue(m.ramBank0Size, m.ramBank1Size))
	m.overlay.Enable()
}

func (m *MemoryConfig) LogicalSize() uint32 {
	size := m.mmuBank0Size + m.mmuBank1Size
	if size == 0 {
		return m.ramBank0Size + m.ramBank1Size
	}
	return size
}

func (m *MemoryConfig) TranslateAddress(address uint32) (uint32, bool) {
	offset, state := m.ResolveAddress(address)
	return offset, state == memoryAddressPresent
}

func (m *MemoryConfig) ResolveAddress(address uint32) (uint32, MemoryAddressState) {
	if address >= m.LogicalSize() {
		return 0, memoryAddressInvalid
	}

	bankStart := uint32(0)
	ramBankSize := m.ramBank0Size
	mmuBankSize := m.mmuBank0Size
	if address >= m.mmuBank0Size {
		bankStart = m.ramBank0Size
		ramBankSize = m.ramBank1Size
		mmuBankSize = m.mmuBank1Size
	}
	if mmuBankSize == 0 {
		return 0, memoryAddressInvalid
	}
	if ramBankSize == 0 {
		return 0, memoryAddressAbsent
	}

	translated := translateSTBank(address, ramBankSize, mmuBankSize)
	return bankStart + translated, memoryAddressPresent
}

func (m *MemoryConfig) setValue(value byte) {
	m.value = value
	m.mmuBank0Size = mmuBankSize((value >> 2) & 0x03)
	m.mmuBank1Size = mmuBankSize(value & 0x03)
}

func mmuBankSize(code byte) uint32 {
	switch code {
	case 0:
		return mmuBankSize128K
	case 1:
		return mmuBankSize512K
	case 2:
		return mmuBankSize2M
	default:
		return 0
	}
}

func physicalBanksForRAM(ramSize uint32) (uint32, uint32, byte) {
	switch ramSize {
	case 128 * 1024:
		return mmuBankSize128K, 0, physicalConfigValue(mmuBankSize128K, 0)
	case 256 * 1024:
		return mmuBankSize128K, mmuBankSize128K, 0x00
	case 512 * 1024:
		return mmuBankSize512K, 0, physicalConfigValue(mmuBankSize512K, 0)
	case 1024 * 1024:
		return mmuBankSize512K, mmuBankSize512K, 0x05
	case 2 * 1024 * 1024:
		return mmuBankSize2M, 0, physicalConfigValue(mmuBankSize2M, 0)
	case 4 * 1024 * 1024:
		return mmuBankSize2M, mmuBankSize2M, 0x0A
	default:
		return ramSize, 0, physicalConfigValue(ramSize, 0)
	}
}

func physicalConfigValue(bank0, bank1 uint32) byte {
	return byte(mmuBankCode(bank0)<<2 | mmuBankCode(bank1))
}

func mmuBankCode(size uint32) byte {
	switch size {
	case 0:
		// MMU code 3 marks the bank as absent.
		return 3
	case mmuBankSize128K:
		return 0
	case mmuBankSize512K:
		return 1
	case mmuBankSize2M:
		return 2
	default:
		return 0
	}
}

func translateSTBank(address, ramBankSize, mmuBankSize uint32) uint32 {
	var translated uint32

	switch ramBankSize {
	case mmuBankSize2M:
		switch mmuBankSize {
		case mmuBankSize2M:
			translated = address
		case mmuBankSize512K:
			translated = ((address & 0xFFC00) << 1) | (address & 0x7FF)
		default:
			translated = ((address & 0x7FE00) << 2) | (address & 0x7FF)
		}
	case mmuBankSize512K:
		switch mmuBankSize {
		case mmuBankSize2M:
			translated = ((address & 0xFF800) >> 1) | (address & 0x3FF)
		case mmuBankSize512K:
			translated = address
		default:
			translated = ((address & 0x3FE00) << 1) | (address & 0x3FF)
		}
	default:
		switch mmuBankSize {
		case mmuBankSize2M:
			translated = ((address & 0x7F800) >> 2) | (address & 0x1FF)
		case mmuBankSize512K:
			translated = ((address & 0x3FC00) >> 1) | (address & 0x1FF)
		default:
			translated = address
		}
	}

	return translated & (ramBankSize - 1)
}

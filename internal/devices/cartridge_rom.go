package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

const (
	cartridgeROMBase = 0xFA0000
	cartridgeROMSize = 128 * 1024
)

// CartridgeROM models the ST cartridge slot at $FA0000-$FBFFFF. The image is
// read-only and may be smaller than the physical window; unpopulated bytes read
// back as $FF, matching an erased ROM/open cartridge byte value.
type CartridgeROM struct {
	data []byte
}

func NewCartridgeROM(image []byte) (*CartridgeROM, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("cartridge ROM image is empty")
	}
	if len(image) > cartridgeROMSize {
		return nil, fmt.Errorf("cartridge ROM image is %d bytes, maximum is %d bytes", len(image), cartridgeROMSize)
	}
	return &CartridgeROM{data: append([]byte(nil), image...)}, nil
}

func (c *CartridgeROM) Contains(address uint32) bool {
	return address >= cartridgeROMBase && address < cartridgeROMBase+cartridgeROMSize
}

func (c *CartridgeROM) Read(size cpu.Size, address uint32) (uint32, error) {
	switch size {
	case cpu.Byte:
		return uint32(c.readByte(address)), nil
	case cpu.Word:
		return uint32(c.readByte(address))<<8 | uint32(c.readByte(address+1)), nil
	case cpu.Long:
		return uint32(c.readByte(address))<<24 |
			uint32(c.readByte(address+1))<<16 |
			uint32(c.readByte(address+2))<<8 |
			uint32(c.readByte(address+3)), nil
	default:
		return 0, nil
	}
}

func (c *CartridgeROM) Peek(size cpu.Size, address uint32) (uint32, error) {
	return c.Read(size, address)
}

func (c *CartridgeROM) Write(cpu.Size, uint32, uint32) error {
	return nil
}

func (c *CartridgeROM) Reset() {}

func (c *CartridgeROM) Bytes() []byte {
	return append([]byte(nil), c.data...)
}

func (c *CartridgeROM) readByte(address uint32) byte {
	offset := address - cartridgeROMBase
	if offset >= uint32(len(c.data)) {
		return 0xFF
	}
	return c.data[offset]
}

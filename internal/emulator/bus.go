package emulator

import "github.com/jenska/m68kemu"

// STBus owns device ordering and exposes the bus interface expected by m68kemu.
type STBus struct {
	bus *m68kemu.Bus
}

func NewSTBus(devices ...m68kemu.Device) *STBus {
	bus := m68kemu.NewBus(devices...)
	bus.SetWaitStates(4)
	return &STBus{bus: bus}
}

func (b *STBus) AddDevice(device m68kemu.Device) {
	b.bus.AddDevice(device)
}

func (b *STBus) Read(size m68kemu.Size, address uint32) (uint32, error) {
	return b.bus.Read(size, address)
}

func (b *STBus) Write(size m68kemu.Size, address uint32, value uint32) error {
	return b.bus.Write(size, address, value)
}

func (b *STBus) Reset() {
	b.bus.Reset()
}

func (b *STBus) CPUAddressBus() m68kemu.AddressBus {
	return b.bus
}

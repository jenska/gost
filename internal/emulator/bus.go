package emulator

import cpu "github.com/jenska/m68kemu"

// STBus owns device ordering and exposes the bus interface expected by m68kemu.
type STBus struct {
	bus *cpu.Bus
}

func NewSTBus(devices ...cpu.Device) *STBus {
	bus := cpu.NewBus(devices...)
	bus.SetWaitStates(4)
	return &STBus{bus: bus}
}

func (b *STBus) AddDevice(device cpu.Device) {
	b.bus.AddDevice(device)
}

func (b *STBus) Read(size cpu.Size, address uint32) (uint32, error) {
	return b.bus.Read(size, address)
}

func (b *STBus) Write(size cpu.Size, address uint32, value uint32) error {
	return b.bus.Write(size, address, value)
}

func (b *STBus) Reset() {
	b.bus.Reset()
}

func (b *STBus) CPUAddressBus() cpu.AddressBus {
	return b.bus
}

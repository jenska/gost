package devices

import (
	cpu "github.com/jenska/m68kemu"
	ym2149 "github.com/jenska/ym2149/emulation"
)

const (
	psgBase              = 0xFF8800
	psgSize              = 4
	stPSGClockHz  uint64 = 2_000_000
	psgSampleRate        = 48_000
)

type PSG struct {
	address       byte
	clockDomain   *ym2149.ClockDomain
	chip          *ym2149.Chip
	portAObserver func(byte)
}

func NewPSG(cpuClockHz uint64) *PSG {
	if cpuClockHz == 0 {
		cpuClockHz = 8_000_000
	}
	return &PSG{
		clockDomain: ym2149.NewPSGClockDomain(int(cpuClockHz), int(stPSGClockHz)),
		chip:        ym2149.NewWithDefaults(int(stPSGClockHz), psgSampleRate),
	}
}

func (p *PSG) Contains(address uint32) bool {
	return address >= psgBase && address < psgBase+psgSize
}

func (p *PSG) WaitStates(cpu.Size, uint32) uint32 {
	return 2
}

func (p *PSG) Reset() {
	p.address = 0
	p.clockDomain.Reset()
	p.chip.Reset()
	p.notifyPortA()
}

func (p *PSG) Read(cpu.Size, uint32) (uint32, error) {
	return uint32(p.chip.ReadData()), nil
}

func (p *PSG) Write(size cpu.Size, address uint32, value uint32) error {
	switch address - psgBase {
	case 0, 1:
		p.address = byte(value) & 0x0F
		p.chip.SelectRegister(p.address)
	case 2, 3:
		p.chip.WriteData(byte(value))
		if p.address == 7 || p.address == 14 {
			p.notifyPortA()
		}
	}
	return nil
}

func (p *PSG) Advance(cycles uint64) {
	if cycles == 0 {
		return
	}
	chipCycles := p.clockDomain.Advance(uint32(cycles))
	if chipCycles == 0 {
		return
	}
	p.chip.Step(chipCycles)
}

func (p *PSG) DrainMonoF32(dst []float32) int {
	return p.chip.DrainMonoF32(dst)
}

func (p *PSG) OutputSampleRate() int {
	return p.chip.OutputSampleRate()
}

func (p *PSG) SetPortAObserver(observer func(byte)) {
	p.portAObserver = observer
	p.notifyPortA()
}

func (p *PSG) notifyPortA() {
	if p.portAObserver == nil {
		return
	}
	p.portAObserver(p.chip.Ports().AOutput)
}

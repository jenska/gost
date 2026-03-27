package emulator

import (
	"fmt"
	"io"
	"os"

	"github.com/jenska/gost/internal/devices"
	"github.com/jenska/m68kemu"
)

const (
	defaultROMHighAlias = 0xFC0000
	secondaryROMAlias   = 0xE00000
	stepQuantumCycles   = 512
)

type Machine struct {
	cfg          Config
	bus          *STBus
	cpu          m68kemu.CPU
	ram          *devices.RAM
	rom          *devices.ROM
	overlayROM   *devices.OverlayROM
	memoryConfig *devices.MemoryConfig
	shifter      *devices.Shifter
	mfp          *devices.MFP
	ikbd         *devices.IKBD
	acia         *devices.ACIA
	fdc          *devices.FDC
	psg          *devices.PSG
	vbl          *devices.VBLSource
	clocked      []devices.Clocked
	irqSources   []devices.InterruptSource
	frameCycles  uint64
	traceMode    string
	traceWriter  io.Writer
	frameCounter uint64
}

func LoadROM(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func LoadDiskImage(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func NewMachine(cfg Config, romImage []byte) (*Machine, error) {
	if len(romImage) == 0 {
		return nil, fmt.Errorf("ROM image is required")
	}
	if cfg.RAMSize == 0 {
		cfg.RAMSize = DefaultRAMSize
	}
	if cfg.ClockHz == 0 {
		cfg.ClockHz = DefaultClockHz
	}
	if cfg.FrameHz == 0 {
		cfg.FrameHz = DefaultFrameHz
	}

	ram := devices.NewRAM(0x000000, cfg.RAMSize)
	rom := devices.NewROM(romImage, defaultROMHighAlias, secondaryROMAlias)
	overlayROM := devices.NewOverlayROM(rom, ram)
	memoryConfig := devices.NewMemoryConfig(overlayROM)
	openBus := devices.NewOpenBus(
		devices.AddressRange{Start: cfg.RAMSize, End: secondaryROMAlias},
		devices.AddressRange{Start: secondaryROMAlias + uint32(len(romImage)), End: defaultROMHighAlias},
	)
	shifter := devices.NewShifter(ram)
	mfp := devices.NewMFP()
	ikbd := devices.NewIKBD()
	acia := devices.NewACIA(ikbd)
	fdc := devices.NewFDC()
	psg := devices.NewPSG()
	vbl := devices.NewVBLSource(cfg.ClockHz, cfg.FrameHz)

	bus := NewSTBus(
		overlayROM,
		ram,
		memoryConfig,
		shifter,
		mfp,
		acia,
		fdc,
		psg,
		rom,
		openBus,
	)

	cpu, err := m68kemu.NewCPU(bus.CPUAddressBus())
	if err != nil {
		return nil, err
	}

	machine := &Machine{
		cfg:          cfg,
		bus:          bus,
		cpu:          cpu,
		ram:          ram,
		rom:          rom,
		overlayROM:   overlayROM,
		memoryConfig: memoryConfig,
		shifter:      shifter,
		mfp:          mfp,
		ikbd:         ikbd,
		acia:         acia,
		fdc:          fdc,
		psg:          psg,
		vbl:          vbl,
		clocked:      []devices.Clocked{mfp, acia, fdc, vbl},
		irqSources:   []devices.InterruptSource{vbl, mfp, acia, fdc},
		frameCycles:  cfg.ClockHz / cfg.FrameHz,
		traceMode:    cfg.Trace,
		traceWriter:  io.Discard,
	}

	if cfg.Trace == "cpu" {
		machine.EnableTrace("cpu", os.Stdout)
	}

	return machine, nil
}

func (m *Machine) EnableTrace(mode string, writer io.Writer) {
	m.traceMode = mode
	if writer == nil {
		writer = io.Discard
	}
	m.traceWriter = writer

	switch mode {
	case "cpu":
		m.cpu.SetTracer(func(info m68kemu.TraceInfo) {
			fmt.Fprintf(m.traceWriter, "pc=%06x sr=%04x cycles=%d\n", info.PC, info.SR, m.cpu.Cycles())
		})
	case "boot":
		m.enableBootTrace()
	default:
		m.cpu.SetTracer(nil)
	}
}

func (m *Machine) enableBootTrace() {
	m.cpu.SetTracer(func(info m68kemu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if pc < 0xE00000 || pc > 0xE00200 {
			return
		}
		fmt.Fprintf(m.traceWriter, "pc=%06x sr=%04x cycles=%d d0=%08x a7=%08x\n",
			pc, info.SR, m.cpu.Cycles(), uint32(info.Registers.D[0]), info.Registers.A[7])
	})

	for _, addr := range []uint32{
		0x000008,
		0x000010,
		0x00002C,
		0xFF8001,
		0xFF8006,
		0xFA0000,
		0xFA0004,
	} {
		m.addAccessTrace(addr)
	}
}

func (m *Machine) addAccessTrace(address uint32) {
	m.cpu.AddBreakpoint(m68kemu.Breakpoint{
		Address: address,
		OnRead:  true,
		OnWrite: true,
		Halt:    false,
		Callback: func(event m68kemu.BreakpointEvent) error {
			fmt.Fprintf(m.traceWriter, "access=%s addr=%06x pc=%06x sr=%04x d0=%08x a7=%08x\n",
				event.Type, event.Address&0xFFFFFF, event.Registers.PC&0xFFFFFF,
				event.Registers.SR, uint32(event.Registers.D[0]), event.Registers.A[7])
			return nil
		},
	})
}

func (m *Machine) Reset() error {
	m.bus.Reset()
	m.frameCounter = 0
	return m.cpu.Reset()
}

func (m *Machine) StepFrame() (bool, error) {
	target := m.cpu.Cycles() + m.frameCycles

	for m.cpu.Cycles() < target {
		remaining := target - m.cpu.Cycles()
		quantum := remaining
		if quantum > stepQuantumCycles {
			quantum = stepQuantumCycles
		}

		before := m.cpu.Cycles()
		if err := m.cpu.RunCycles(quantum); err != nil {
			return false, err
		}
		advanced := m.cpu.Cycles() - before
		m.advanceDevices(advanced)
		m.dispatchInterrupts()
	}

	m.frameCounter++
	return m.shifter.Render(m.cpu.Cycles()), nil
}

func (m *Machine) advanceDevices(cycles uint64) {
	for _, device := range m.clocked {
		device.Advance(cycles)
	}
}

func (m *Machine) dispatchInterrupts() {
	for _, source := range m.irqSources {
		for _, irq := range source.DrainInterrupts() {
			_ = m.cpu.RequestInterrupt(irq.Level, irq.Vector)
		}
	}
}

func (m *Machine) FrameBuffer() []byte {
	return m.shifter.FrameBuffer()
}

func (m *Machine) Dimensions() (int, int) {
	return m.shifter.Dimensions()
}

func (m *Machine) Registers() m68kemu.Registers {
	return m.cpu.Registers()
}

func (m *Machine) Cycles() uint64 {
	return m.cpu.Cycles()
}

func (m *Machine) PushKey(scancode byte, pressed bool) {
	m.ikbd.PushKey(scancode, pressed)
}

func (m *Machine) PushMouse(dx, dy int, buttons byte) {
	m.ikbd.PushMouse(dx, dy, buttons)
}

func (m *Machine) InsertFloppy(side int, image []byte) error {
	if side != 0 {
		return fmt.Errorf("only floppy side 0 is supported")
	}
	return m.fdc.InsertDisk(image)
}

func (m *Machine) RequestInterrupt(level uint8, vector *uint8) error {
	return m.cpu.RequestInterrupt(level, vector)
}

func (m *Machine) LoadIntoRAM(address uint32, payload []byte) error {
	return m.ram.LoadAt(address, payload)
}

func (m *Machine) ROM() *devices.ROM {
	return m.rom
}

package emulator

import (
	"fmt"
	"io"
	"os"

	"github.com/jenska/gost/internal/devices"
	"github.com/jenska/m68kdasm"
	"github.com/jenska/m68kemu"
)

const (
	defaultROMHighAlias = 0xFC0000
	secondaryROMAlias   = 0xE00000
	stepQuantumCycles   = 512
	bootTraceStart      = 0xE00000
	bootTraceEnd        = 0xE01000
)

var bootTraceAddresses = []uint32{
	0x000008,
	0x000010,
	0x00002C,
	0x000420,
	0x000424,
	0x000426,
	0x00042E,
	0x00043A,
	0x00051A,
	0x0005A4,
	0x0005A8,
	0x200008,
	0x200010,
	0xFF8001,
	0xFF8006,
	0xFF8201,
	0xFF8203,
	0xFF8240,
	0xFF8260,
	0xFF820D,
	0xFA0000,
	0xFA0004,
}

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
	if cfg.TraceStart == 0 && cfg.TraceEnd == 0 {
		cfg.TraceStart = bootTraceStart
		cfg.TraceEnd = bootTraceEnd
	}

	ram := devices.NewRAM(0x000000, cfg.RAMSize)
	rom := devices.NewROM(romImage, defaultROMHighAlias, secondaryROMAlias)
	overlayROM := devices.NewOverlayROM(rom, ram)
	memoryConfig := devices.NewMemoryConfig(overlayROM, cfg.RAMSize)
	ram.SetMemoryConfig(memoryConfig)
	glue := devices.NewGLUE()
	openBus := devices.NewOpenBus(
		devices.AddressRange{Start: cfg.RAMSize, End: secondaryROMAlias},
		devices.AddressRange{Start: secondaryROMAlias + uint32(len(romImage)), End: defaultROMHighAlias},
		devices.AddressRange{Start: 0xFF8000, End: 0x1000000},
	)
	shifter := devices.NewShifter(ram)
	mfp := devices.NewMFP(cfg.ClockHz)
	ikbd := devices.NewIKBD()
	acia := devices.NewACIA(ikbd)
	fdc := devices.NewFDC()
	psg := devices.NewPSG()
	vbl := devices.NewVBLSource(cfg.ClockHz, cfg.FrameHz)

	bus := NewSTBus(
		overlayROM,
		ram,
		memoryConfig,
		glue,
		shifter,
		mfp,
		acia,
		fdc,
		psg,
		openBus,
		rom,
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
	m.cpu.SetTracer(nil)
	m.cpu.SetBusTracer(nil)
	m.cpu.SetExceptionTracer(nil)

	switch mode {
	case "cpu":
		m.cpu.SetTracer(func(info m68kemu.TraceInfo) {
			fmt.Fprintf(m.traceWriter, "pc=%06x sr=%04x cycles=%d\n", info.PC, info.SR, m.cpu.Cycles())
		})
	case "cpu-verbose":
		logger := m68kemu.NewVerboseLogger(m.cpu, m.bus.CPUAddressBus(), m.traceWriter, m68kemu.VerboseLoggerOptions{
			IncludeCycles: true,
		})
		m.cpu.SetTracer(logger.Trace)
	case "boot":
		m.enableBootTrace(false)
	case "boot-verbose":
		m.enableBootTrace(true)
	default:
		m.cpu.SetTracer(nil)
	}
}

func (m *Machine) enableBootTrace(verbose bool) {
	m.cpu.SetBusTracer(func(info m68kemu.BusAccessInfo) {
		address := info.Address & 0xFFFFFF
		if info.InstructionFetch || !isBootTraceAddress(address) {
			return
		}
		regs := m.cpu.Registers()
		fmt.Fprintf(m.traceWriter, "access=%s addr=%06x pc=%06x sr=%04x d0=%08x a7=%08x value=%s\n",
			traceAccessKind(info.Write),
			address,
			regs.PC&0xFFFFFF,
			regs.SR,
			uint32(regs.D[0]),
			regs.A[7],
			traceValueString(info.Size, info.Value),
		)
	})
	m.cpu.SetExceptionTracer(func(info m68kemu.ExceptionInfo) {
		if info.FaultValid {
			fmt.Fprintf(m.traceWriter, "exception vector=%d pc=%06x newpc=%06x opcode=%04x fault=%06x sr=%04x newsr=%04x\n",
				info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.FaultAddress&0xFFFFFF, info.SR, info.NewSR)
			return
		}
		fmt.Fprintf(m.traceWriter, "exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x\n",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR)
	})

	if verbose {
		logger := m68kemu.NewVerboseLogger(m.cpu, m.bus.CPUAddressBus(), m.traceWriter, m68kemu.VerboseLoggerOptions{
			IncludeRegisters: true,
			IncludeCycles:    true,
		})
		m.cpu.SetTracer(func(info m68kemu.TraceInfo) {
			pc := info.PC & 0xFFFFFF
			if !m.tracePCInRange(pc) {
				return
			}
			logger.Trace(info)
		})
	} else {
		m.cpu.SetTracer(func(info m68kemu.TraceInfo) {
			pc := info.PC & 0xFFFFFF
			if !m.tracePCInRange(pc) {
				return
			}
			fmt.Fprintf(m.traceWriter,
				"pc=%06x sr=%04x cycles=%d d0=%08x d6=%08x d7=%08x a0=%08x a1=%08x a3=%08x a7=%08x ins=%s\n",
				pc,
				info.SR,
				m.cpu.Cycles(),
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[6]),
				uint32(info.Registers.D[7]),
				info.Registers.A[0],
				info.Registers.A[1],
				info.Registers.A[3],
				info.Registers.A[7],
				m.decodeTraceInstruction(info),
			)
		})
	}
}

func (m *Machine) tracePCInRange(pc uint32) bool {
	start := m.cfg.TraceStart & 0xFFFFFF
	end := m.cfg.TraceEnd & 0xFFFFFF
	if start == 0 && end == 0 {
		start = bootTraceStart
		end = bootTraceEnd
	}
	if end < start {
		start, end = end, start
	}
	pc &= 0xFFFFFF
	return pc >= start && pc <= end
}

func isBootTraceAddress(address uint32) bool {
	address &= 0xFFFFFF
	for _, candidate := range bootTraceAddresses {
		if candidate == address {
			return true
		}
	}
	return false
}

func traceAccessKind(write bool) string {
	if write {
		return "write"
	}
	return "read"
}

func traceValueString(size m68kemu.Size, value uint32) string {
	switch size {
	case m68kemu.Byte:
		return fmt.Sprintf("%02x", value&0xFF)
	case m68kemu.Word:
		return fmt.Sprintf("%04x", value&0xFFFF)
	default:
		return fmt.Sprintf("%08x", value)
	}
}

func (m *Machine) decodeTraceInstruction(info m68kemu.TraceInfo) string {
	if len(info.Bytes) >= 2 {
		inst, err := m68kdasm.Decode(append([]byte(nil), info.Bytes...), info.PC)
		if err == nil {
			return inst.Assembly()
		}
	}
	inst, err := m68kemu.DisassembleInstruction(m.bus.CPUAddressBus(), info.PC)
	if err != nil {
		return fmt.Sprintf("<decode error: %v>", err)
	}
	return inst.Assembly
}

func (m *Machine) Reset() error {
	m.bus.Reset()
	m.ram.ColdReset()
	m.overlayROM.ColdReset()
	m.memoryConfig.ColdReset()
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

func (m *Machine) RunUntil(options m68kemu.RunUntilOptions) (m68kemu.RunResult, error) {
	if options.MaxInstructions == 0 &&
		!options.StopOnException &&
		!options.StopOnIllegal &&
		options.StopOnPCRange == nil &&
		options.StopWhenPCOutside == nil {
		return m68kemu.RunResult{}, fmt.Errorf("RunUntil requires a stop condition or instruction limit")
	}

	startCycles := m.cpu.Cycles()
	var total m68kemu.RunResult
	for {
		if options.MaxInstructions > 0 && total.Instructions >= options.MaxInstructions {
			total.Reason = m68kemu.RunStopInstructionLimit
			total.PC = m.cpu.Registers().PC
			total.Cycles = m.cpu.Cycles() - startCycles
			return total, nil
		}

		stepOptions := options
		stepOptions.MaxInstructions = 1

		before := m.cpu.Cycles()
		result, err := m.cpu.RunUntil(stepOptions)
		advanced := m.cpu.Cycles() - before
		if advanced > 0 {
			m.advanceDevices(advanced)
			m.dispatchInterrupts()
		}
		if err != nil {
			return total, err
		}

		total.Instructions += result.Instructions
		total.Cycles = m.cpu.Cycles() - startCycles
		total.PC = result.PC
		if result.HasException {
			total.Exception = result.Exception
			total.HasException = true
		}

		if result.Reason != m68kemu.RunStopInstructionLimit {
			total.Reason = result.Reason
			return total, nil
		}
	}
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

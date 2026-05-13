package emulator

import (
	"fmt"
	"io"

	"github.com/jenska/m68kdasm"
	cpu "github.com/jenska/m68kemu"
)

// TraceMode defines the available trace modes for debugging.
type TraceMode string

const (
	// TraceModeNone disables tracing.
	TraceModeNone TraceMode = ""
	// TraceModeSimple outputs basic CPU trace: pc, sr, cycles.
	TraceModeSimple TraceMode = "cpu"
	// TraceModeSimpleVerbose outputs detailed CPU trace with registers.
	TraceModeSimpleVerbose TraceMode = "cpu-verbose"
	// TraceModeBootSimple traces bus accesses during early boot.
	TraceModeBootSimple TraceMode = "boot"
	// TraceModeBootVerbose traces bus accesses and CPU state during boot.
	TraceModeBootVerbose TraceMode = "boot-verbose"
	// TraceModeShifterSimple traces shifter frame operations.
	TraceModeShifterSimple TraceMode = "shifter"
	// TraceModeShifterVerbose traces shifter frame operations with detailed stats.
	TraceModeShifterVerbose TraceMode = "shifter-verbose"
)

// bootTraceAddressSet contains 24-bit addresses to trace during boot.
// These are critical memory locations where bus activity is monitored.
var bootTraceAddressSet = initBootTraceAddressSet()

// initBootTraceAddressSet creates a map of boot trace addresses for O(1) lookup.
func initBootTraceAddressSet() map[uint32]bool {
	addrs := []uint32{
		0x000008, // Exception vectors
		0x000010,
		0x00002C,
		0x000420, // System variables
		0x000424,
		0x000426,
		0x00042E,
		0x00043A,
		0x00051A,
		0x0005A4,
		0x0005A8,
		0x200008, // Boot ROM locations
		0x200010,
		0xFF8001, // Hardware registers
		0xFF8006,
		0xFF8201,
		0xFF8203,
		0xFF8240,
		0xFF8260,
		0xFF820D,
		0xFF8901,
		0xFFFA01,
		0xFA0000, // Probe region
		0xFA0004,
	}
	set := make(map[uint32]bool, len(addrs))
	for _, addr := range addrs {
		set[addr&0xFFFFFF] = true
	}
	return set
}

// EnableTrace activates a trace mode and sets the output writer.
func (m *Machine) EnableTrace(mode string, writer io.Writer) {
	m.cfg.Trace = mode
	if writer == nil {
		writer = io.Discard
	}
	m.traceWriter = writer
	m.cpu.SetTracer(nil)
	m.cpu.SetBusTracer(nil)
	m.cpu.SetExceptionTracer(nil)
	m.shifter.SetDebug(false)

	switch TraceMode(mode) {
	case TraceModeSimple:
		m.cpu.SetTracer(func(info cpu.TraceInfo) {
			fmt.Fprintf(m.traceWriter, "pc=%06x sr=%04x cycles=%d\n", info.PC, info.SR, m.cpu.Cycles())
		})
	case TraceModeSimpleVerbose:
		logger := cpu.NewVerboseLogger(m.cpu, m.bus, m.traceWriter, cpu.VerboseLoggerOptions{
			IncludeCycles: true,
		})
		m.cpu.SetTracer(logger.Trace)
	case TraceModeBootSimple:
		m.enableBootTrace(false)
	case TraceModeBootVerbose:
		m.enableBootTrace(true)
	case TraceModeShifterSimple, TraceModeShifterVerbose:
		m.shifter.SetDebug(true)
	default:
		m.cpu.SetTracer(nil)
	}
}

func (m *Machine) enableBootTrace(verbose bool) {
	m.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		address := info.Address & 0xFFFFFF
		if info.InstructionFetch || !bootTraceAddressSet[address] {
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
	m.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.FaultValid {
			fmt.Fprintf(m.traceWriter, "exception vector=%d pc=%06x newpc=%06x opcode=%04x fault=%06x sr=%04x newsr=%04x\n",
				info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.FaultAddress&0xFFFFFF, info.SR, info.NewSR)
			return
		}
		fmt.Fprintf(m.traceWriter, "exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x\n",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR)
	})

	if verbose {
		logger := cpu.NewVerboseLogger(m.cpu, m.bus, m.traceWriter, cpu.VerboseLoggerOptions{
			IncludeRegisters: true,
			IncludeCycles:    true,
		})
		m.cpu.SetTracer(func(info cpu.TraceInfo) {
			pc := info.PC & 0xFFFFFF
			if m.tracePCInRange(pc) {
				logger.Trace(info)
			}
		})
		return
	}

	m.cpu.SetTracer(func(info cpu.TraceInfo) {
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

func (m *Machine) tracePCInRange(pc uint32) bool {
	start := m.cfg.TraceStart & 0xFFFFFF
	end := m.cfg.TraceEnd & 0xFFFFFF
	if end < start {
		start, end = end, start
	}
	pc &= 0xFFFFFF
	return pc >= start && pc <= end
}

func isBootTraceAddress(address uint32) bool {
	return bootTraceAddressSet[address&0xFFFFFF]
}

func traceAccessKind(write bool) string {
	if write {
		return "write"
	}
	return "read"
}

func traceValueString(size cpu.Size, value uint32) string {
	switch size {
	case cpu.Byte:
		return fmt.Sprintf("%02x", value&0xFF)
	case cpu.Word:
		return fmt.Sprintf("%04x", value&0xFFFF)
	default:
		return fmt.Sprintf("%08x", value)
	}
}

func (m *Machine) decodeTraceInstruction(info cpu.TraceInfo) string {
	if len(info.Bytes) >= 2 {
		inst, err := m68kdasm.Decode(append([]byte(nil), info.Bytes...), info.PC)
		if err == nil {
			return inst.Assembly()
		}
	}
	inst, err := cpu.DisassembleInstruction(m.bus, info.PC)
	if err != nil {
		return fmt.Sprintf("<decode error: %v>", err)
	}
	return inst.Assembly
}

func (m *Machine) traceShifterFrame(rendered bool) {
	stats := m.shifter.DebugStats()
	displayW, displayH := m.shifter.DisplayDimensions()
	viewportX, viewportY, viewportW, viewportH := m.shifter.DisplayViewport()
	if m.cfg.Trace == string(TraceModeShifterVerbose) {
		fmt.Fprintf(m.traceWriter, "shifter frame=%d rendered=%t size=%dx%d display=%dx%d viewport=%d,%d,%d,%d base=%06x vaddr=%06x frame_pos=%d/%d render_ns=%d pixels=%d blank=%d words=%d faults=%d waits=%d totals{render_ns=%d pixels=%d blank=%d words=%d faults=%d waits=%d}\n",
			m.frameCounter,
			rendered,
			stats.LastWidth,
			stats.LastHeight,
			displayW,
			displayH,
			viewportX,
			viewportY,
			viewportW,
			viewportH,
			stats.ScreenBase&0xFFFFFF,
			stats.VideoAddress&0xFFFFFF,
			stats.FrameCyclePos,
			stats.FrameCycles,
			stats.LastRenderNanos,
			stats.LastPixelsDrawn,
			stats.LastBlankPixels,
			stats.LastVideoWords,
			stats.LastReadFaults,
			stats.LastWaitHits,
			stats.TotalRenderNanos,
			stats.TotalPixelsDrawn,
			stats.TotalBlankPixels,
			stats.TotalVideoWords,
			stats.TotalReadFaults,
			stats.TotalWaitHits,
		)
		return
	}
	fmt.Fprintf(m.traceWriter, "shifter frame=%d rendered=%t size=%dx%d display=%dx%d viewport=%d,%d,%d,%d render_ns=%d pixels=%d blank=%d words=%d waits=%d\n",
		m.frameCounter,
		rendered,
		stats.LastWidth,
		stats.LastHeight,
		displayW,
		displayH,
		viewportX,
		viewportY,
		viewportW,
		viewportH,
		stats.LastRenderNanos,
		stats.LastPixelsDrawn,
		stats.LastBlankPixels,
		stats.LastVideoWords,
		stats.LastWaitHits,
	)
}

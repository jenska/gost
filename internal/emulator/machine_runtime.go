package emulator

import (
	"fmt"

	"github.com/jenska/gost/internal/devices"
	cpu "github.com/jenska/m68kemu"
)

func (m *Machine) Reset() error {
	m.bus.Reset()
	m.ram.ColdReset()
	m.overlayROM.ColdReset()
	m.memoryConfig.ColdReset()
	m.cpuCycleCarry = 0
	m.cpuCycleDebt = 0
	m.frameCounter = 0
	return m.cpu.Reset()
}

// romMirrors maps the ROM image at each of its aliases, cut short at the end of
// the 24-bit address space. The cartridge port and the hardware registers come
// earlier on the bus, so they win where an alias overlaps them. With direct
// access the CPU reads the image without going through the bus; EmuTOS executes
// entirely from ROM, so this covers the bulk of the fetch traffic. The ST's
// GLUE acknowledges ROM accesses immediately, so neither path adds wait states.
func romMirrors(rom *devices.ROM, direct bool) []*devices.ROMMirror {
	var mirrors []*devices.ROMMirror
	for _, base := range rom.Aliases() {
		mirrors = append(mirrors, rom.Mirror(base, addressSpaceEnd, direct))
	}
	return mirrors
}

func (m *Machine) StepFrame() (bool, error) {
	m.shifter.BeginFrame()
	remainingHardwareCycles := m.frameCycles
	for remainingHardwareCycles > 0 {
		quantum := m.nextStepQuantum(remainingHardwareCycles)
		if err := m.runCPU(m.cpuCyclesForHardwareCycles(quantum)); err != nil {
			return false, err
		}
		m.advanceDevices(quantum)
		m.shifter.AdvanceFrame(quantum)
		remainingHardwareCycles -= quantum
	}

	m.frameCounter++
	rendered := m.shifter.EndFrame()
	if m.cfg.Trace == string(TraceModeShifterSimple) || m.cfg.Trace == string(TraceModeShifterVerbose) {
		m.traceShifterFrame(rendered)
	}
	return rendered, nil
}

// runCPU gives the CPU budget cycles. RunCycles only stops on an instruction
// boundary, so it usually overshoots; the overshoot is owed and deducted from
// the next budget, keeping the CPU locked to the machine clock instead of
// gaining a partial instruction on every quantum.
func (m *Machine) runCPU(budget uint64) error {
	if budget <= m.cpuCycleDebt {
		m.cpuCycleDebt -= budget
		return nil
	}
	budget -= m.cpuCycleDebt
	start := m.cpu.Cycles()
	if err := m.cpu.RunCycles(budget); err != nil {
		m.cpuCycleDebt = 0
		return err
	}
	m.cpuCycleDebt = m.cpu.Cycles() - start - budget
	return nil
}

func (m *Machine) nextStepQuantum(remainingHardwareCycles uint64) uint64 {
	quantum := min(remainingHardwareCycles, stepQuantumCycles)
	if next, ok := m.nextDeviceEventCycles(); ok && next < quantum {
		quantum = next
	}
	if quantum == 0 {
		return 1
	}
	return quantum
}

func (m *Machine) cpuCyclesForHardwareCycles(hardwareCycles uint64) uint64 {
	if hardwareCycles == 0 || m.cfg.ClockHz == 0 {
		return 0
	}
	if m.cfg.CPUClockHz == m.cfg.ClockHz {
		return hardwareCycles
	}

	total := hardwareCycles*m.cfg.CPUClockHz + m.cpuCycleCarry
	cpuCycles := total / m.cfg.ClockHz
	m.cpuCycleCarry = total % m.cfg.ClockHz
	return cpuCycles
}

func (m *Machine) nextDeviceEventCycles() (uint64, bool) {
	var minCycles uint64
	for _, device := range m.clocked {
		predictor, ok := device.(devices.EventPredictor)
		if !ok {
			continue
		}
		cycles, ok := predictor.NextEventCycles()
		if !ok || cycles == 0 {
			continue
		}
		if minCycles == 0 || cycles < minCycles {
			minCycles = cycles
		}
	}
	if minCycles == 0 {
		return 0, false
	}
	return minCycles, true
}

func (m *Machine) RunUntil(options cpu.RunUntilOptions) (cpu.RunResult, error) {
	if options.MaxInstructions == 0 &&
		!options.StopOnException &&
		!options.StopOnIllegal &&
		len(options.StopAtPC) == 0 &&
		options.StopOnPCRange == nil &&
		options.StopWhenPCOutside == nil &&
		options.StopOnBusAccess == nil &&
		options.StopPredicate == nil {
		return cpu.RunResult{}, fmt.Errorf("RunUntil requires a stop condition or instruction limit")
	}

	startCycles := m.cpu.Cycles()
	var total cpu.RunResult
	for {
		if options.MaxInstructions > 0 && total.Instructions >= options.MaxInstructions {
			total.Reason = cpu.RunStopInstructionLimit
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
		if result.HasBusAccess {
			total.BusAccess = result.BusAccess
			total.HasBusAccess = true
		}
		if result.HasInterrupt {
			total.Interrupt = result.Interrupt
			total.HasInterrupt = true
		}

		if result.Reason != cpu.RunStopInstructionLimit {
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

// machineIRQ presents the ST's device interrupt lines to the CPU core as a
// single level-sensitive source. The core samples it at every instruction
// boundary and drops requests it is currently masking, which replaces the
// machine's old hand-rolled drain-and-mask loop. Levels are distinct across
// sources (GLUE 2/4 < FDC 5 < MFP 6), so the highest asserted line wins and
// AckIRQ routes the acknowledgement back to the source that raised it.
type machineIRQ struct {
	glue *devices.GLUE
	mfp  *devices.MFP
	fdc  *devices.FDC
}

func (s machineIRQ) PendingIRQ() (level, vector uint8) {
	if s.mfp != nil {
		if l, v := s.mfp.PendingIRQ(); l > level {
			level, vector = l, v
		}
	}
	if s.fdc != nil {
		if l, v := s.fdc.PendingIRQ(); l > level {
			level, vector = l, v
		}
	}
	if s.glue != nil {
		if l, v := s.glue.PendingIRQ(); l > level {
			level, vector = l, v
		}
	}
	return level, vector
}

func (s machineIRQ) AckIRQ(level uint8) {
	switch {
	case level >= 6:
		if s.mfp != nil {
			s.mfp.AckIRQ(level)
		}
	case level == 5:
		if s.fdc != nil {
			s.fdc.AckIRQ(level)
		}
	default:
		if s.glue != nil {
			s.glue.AckIRQ(level)
		}
	}
}

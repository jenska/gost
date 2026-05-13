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
	m.frameCounter = 0
	return m.cpu.Reset()
}

func (m *Machine) StepFrame() (bool, error) {
	m.shifter.BeginFrame()
	remainingHardwareCycles := m.frameCycles
	for remainingHardwareCycles > 0 {
		m.dispatchInterrupts()

		quantum := m.nextStepQuantum(remainingHardwareCycles)
		cpuQuantum := m.cpuCyclesForHardwareCycles(quantum)
		if cpuQuantum > 0 {
			if err := m.cpu.RunCycles(cpuQuantum); err != nil {
				return false, err
			}
		}
		m.advanceDevices(quantum)
		m.shifter.AdvanceFrame(quantum)
		m.dispatchInterrupts()
		remainingHardwareCycles -= quantum
	}

	m.frameCounter++
	rendered := m.shifter.EndFrame()
	if m.cfg.Trace == string(TraceModeShifterSimple) || m.cfg.Trace == string(TraceModeShifterVerbose) {
		m.traceShifterFrame(rendered)
	}
	return rendered, nil
}

func (m *Machine) nextStepQuantum(remainingHardwareCycles uint64) uint64 {
	quantum := remainingHardwareCycles
	if quantum > stepQuantumCycles {
		quantum = stepQuantumCycles
	}
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

func (m *Machine) dispatchInterrupts() {
	for _, source := range m.irqSources {
		for _, irq := range source.DrainInterrupts() {
			_ = m.cpu.RequestInterrupt(irq.Level, irq.Vector)
		}
	}
}

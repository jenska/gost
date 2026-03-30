//go:build debugtests
// +build debugtests

package emulator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jenska/gost/internal/assets"
	cpu "github.com/jenska/m68kemu"
)

func TestDebugLateMFPState(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	dumpMFPState(t, machine, "reset")
	for frame := 0; frame < 60; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if frame == 19 || frame == 39 || frame == 49 || frame == 59 {
			dumpMFPState(t, machine, fmt.Sprintf("after frame %d", frame+1))
		}
	}
}

func TestDebugLateTrapStackBanks(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 20; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 320 {
			history = history[len(history)-320:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE12868 && pc <= 0xE128C4) ||
			(pc >= 0xE1298C && pc <= 0xE129AE) ||
			(pc >= 0xE12D1C && pc <= 0xE12E20) ||
			(pc >= 0xE13C34 && pc <= 0xE13D4C) {
			pushHistory(fmt.Sprintf(
				"pc=%06x sr=%04x usp=%08x ssp=%08x a7=%08x a3=%08x a4=%08x a5=%08x a6=%08x d0=%08x d2=%08x ins=%s",
				pc,
				info.SR,
				info.Registers.USP,
				info.Registers.SSP,
				info.Registers.A[7],
				info.Registers.A[3],
				info.Registers.A[4],
				info.Registers.A[5],
				info.Registers.A[6],
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[2]),
				machine.decodeTraceInstruction(info),
			))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if (addr >= 0x00649C && addr < 0x006520) ||
			(addr >= 0x007400 && addr < 0x007430) ||
			(addr >= 0x007470 && addr < 0x0074C0) ||
			(addr >= 0x007580 && addr < 0x0075A0) {
			regs := machine.cpu.Registers()
			pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x usp=%08x ssp=%08x a7=%08x",
				traceAccessKind(info.Write),
				addr,
				info.Size,
				info.Value,
				regs.PC&0xFFFFFF,
				regs.USP,
				regs.SSP,
				regs.A[7],
			))
		}
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR))
		if info.Vector == 4 && info.PC&0xFFFFFF == 0x00000A {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
		}
	})

	for frame := 20; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			return
		}
	}

	t.Fatalf("panic record was not set")
}

func TestDebugLowMemProcessPointerWrites(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 240 {
			history = history[len(history)-240:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE12690 && pc <= 0xE129CC) ||
			(pc >= 0xE12B80 && pc <= 0xE12BA6) ||
			(pc >= 0xE13C34 && pc <= 0xE13D4C) {
			pushHistory(fmt.Sprintf(
				"pc=%06x sr=%04x a7=%08x d0=%08x d2=%08x a0=%08x a2=%08x a3=%08x a4=%08x ins=%s",
				pc,
				info.SR,
				info.Registers.A[7],
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[2]),
				info.Registers.A[0],
				info.Registers.A[2],
				info.Registers.A[3],
				info.Registers.A[4],
				machine.decodeTraceInstruction(info),
			))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr != 0x00649C && addr != 0x0064A0 {
			return
		}
		regs := machine.cpu.Registers()
		pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
			traceAccessKind(info.Write),
			addr,
			info.Size,
			info.Value,
			regs.PC&0xFFFFFF,
			regs.A[7],
		))
		if addr == 0x0064A0 && info.Value == 0x00400018 {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
		}
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}
}

func TestDebugProcessPointerProducer(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	readLong := func(addr uint32) uint32 {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read long %06x: %v", addr, err)
		}
		return value
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 320 {
			history = history[len(history)-320:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE12698 && pc <= 0xE126FC) ||
			(pc >= 0xE12974 && pc <= 0xE129AE) ||
			(pc >= 0xE12B80 && pc <= 0xE12BA6) ||
			(pc >= 0xE13168 && pc <= 0xE13258) ||
			(pc >= 0xE133B0 && pc <= 0xE133E2) ||
			(pc >= 0xE138F4 && pc <= 0xE13914) {
			pushHistory(fmt.Sprintf(
				"pc=%06x sr=%04x a7=%08x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a2=%08x ins=%s",
				pc,
				info.SR,
				info.Registers.A[7],
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[1]),
				uint32(info.Registers.D[2]),
				uint32(info.Registers.D[3]),
				info.Registers.A[0],
				info.Registers.A[1],
				info.Registers.A[2],
				machine.decodeTraceInstruction(info),
			))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr != 0x00649C && addr != 0x0064A0 {
			return
		}
		regs := machine.cpu.Registers()
		pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
			traceAccessKind(info.Write),
			addr,
			info.Size,
			info.Value,
			regs.PC&0xFFFFFF,
			regs.A[7],
		))
	})

	pushHistory(fmt.Sprintf("frame=%d 649c=%08x 64a0=%08x", 0, readLong(0x649C), readLong(0x64A0)))
	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		current649C := readLong(0x649C)
		current64A0 := readLong(0x64A0)
		pushHistory(fmt.Sprintf("frame=%d 649c=%08x 64a0=%08x", frame+1, current649C, current64A0))
		if current649C == 0x00400018 || current64A0 == 0x00400018 {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			return
		}
	}

	fmt.Printf("%s\n", strings.Join(history, "\n"))
	t.Fatalf("did not observe 0x00400018 in 649c/64a0 within 120 frames")
}

func TestDebugProcessDescriptorInitialization(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	readLong := func(addr uint32) uint32 {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read long %06x: %v", addr, err)
		}
		return value
	}
	readByte := func(addr uint32) uint32 {
		value, err := machine.bus.Read(cpu.Byte, addr)
		if err != nil {
			t.Fatalf("read byte %06x: %v", addr, err)
		}
		return value
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 320 {
			history = history[len(history)-320:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if (addr >= 0x004620 && addr < 0x004680) ||
			addr == 0x000420 ||
			addr == 0x000424 ||
			addr == 0x00042E ||
			addr == 0x00043A ||
			addr == 0xFF8001 {
			regs := machine.cpu.Registers()
			pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
				traceAccessKind(info.Write),
				addr,
				info.Size,
				info.Value,
				regs.PC&0xFFFFFF,
				regs.A[7],
			))
		}
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		current649C := readLong(0x649C)
		if current649C != 0x00400018 {
			continue
		}

		pushHistory(fmt.Sprintf("frame=%d mmu=%02x 0420=%08x 0424=%08x 042e=%08x 043a=%08x",
			frame+1,
			readByte(0xFF8001),
			readLong(0x0420),
			readLong(0x0424),
			readLong(0x042E),
			readLong(0x043A),
		))
		pushHistory(fmt.Sprintf("globals 74b8=%08x 74bc=%08x 74be=%08x 74ca=%08x 74d8=%08x 74dc=%08x",
			readLong(0x74B8),
			readLong(0x74BC),
			readLong(0x74BE),
			readLong(0x74CA),
			readLong(0x74D8),
			readLong(0x74DC),
		))
		for _, addr := range []uint32{0x4632, 0x4644, 0x4656} {
			pushHistory(fmt.Sprintf("node %06x: %08x %08x %08x %08x",
				addr,
				readLong(addr),
				readLong(addr+4),
				readLong(addr+8),
				readLong(addr+12),
			))
		}
		fmt.Printf("%s\n", strings.Join(history, "\n"))
		return
	}

	t.Fatalf("did not reach descriptor state with 649c=0x00400018")
}

func TestDebugAltRAMProbeAccesses(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	readLong := func(addr uint32) uint32 {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read long %06x: %v", addr, err)
		}
		return value
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 400 {
			history = history[len(history)-400:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE02000 && pc <= 0xE02400) ||
			(pc >= 0xE13288 && pc <= 0xE1333C) {
			pushHistory(fmt.Sprintf(
				"pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a3=%08x ins=%s",
				pc,
				info.SR,
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[1]),
				uint32(info.Registers.D[2]),
				uint32(info.Registers.D[3]),
				info.Registers.A[0],
				info.Registers.A[3],
				machine.decodeTraceInstruction(info),
			))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if (addr >= 0x400000 && addr < 0x400040) ||
			(addr >= 0xFA0000 && addr < 0xFA0010) {
			regs := machine.cpu.Registers()
			pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
				traceAccessKind(info.Write),
				addr,
				info.Size,
				info.Value,
				regs.PC&0xFFFFFF,
				regs.A[7],
			))
		}
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x fault_valid=%t fault=%06x",
			info.Vector,
			info.PC&0xFFFFFF,
			info.NewPC&0xFFFFFF,
			info.Opcode,
			info.FaultValid,
			info.FaultAddress&0xFFFFFF,
		))
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if readLong(0x649C) == 0x00400018 {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			return
		}
	}

	fmt.Printf("%s\n", strings.Join(history, "\n"))
	t.Fatalf("did not reach descriptor state with 649c=0x00400018")
}

func TestDebugAltRAMDescriptorCaller(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 96 {
			history = history[len(history)-96:]
		}
	}

	var hit bool
	var snapshotRegs cpu.Registers
	var snapshotStack [8]uint32
	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE00060 && pc <= 0xE02220) || (pc >= 0xE13288 && pc <= 0xE132C4) {
			pushHistory(fmt.Sprintf("pc=%06x sr=%04x a7=%08x d0=%08x d1=%08x d2=%08x d3=%08x ins=%s",
				pc,
				info.SR,
				info.Registers.A[7],
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[1]),
				uint32(info.Registers.D[2]),
				uint32(info.Registers.D[3]),
				machine.decodeTraceInstruction(info),
			))
		}
		if pc == 0xE13288 && !hit {
			hit = true
			snapshotRegs = info.Registers
			for i := range snapshotStack {
				value, err := machine.ram.Read(cpu.Long, info.Registers.A[7]+uint32(i*4))
				if err != nil {
					snapshotStack[i] = 0xFFFFFFFF
					continue
				}
				snapshotStack[i] = value
			}
		}
	})

	for frame := 0; frame < 120 && !hit; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}
	if !hit {
		t.Fatalf("did not reach alt ram descriptor builder within 120 frames")
	}

	fmt.Printf("%s\n", strings.Join(history, "\n"))
	fmt.Printf("entry pc=%06x a7=%08x d0=%08x d1=%08x d2=%08x d3=%08x\n",
		snapshotRegs.PC&0xFFFFFF,
		snapshotRegs.A[7],
		uint32(snapshotRegs.D[0]),
		uint32(snapshotRegs.D[1]),
		uint32(snapshotRegs.D[2]),
		uint32(snapshotRegs.D[3]),
	)
	for i, value := range snapshotStack {
		fmt.Printf("stack[%02x]=%08x\n", i*4, value)
	}
}

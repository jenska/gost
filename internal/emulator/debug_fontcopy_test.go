//go:build debugtests
// +build debugtests

package emulator

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/devices"
	cpu "github.com/jenska/m68kemu"
)

func dumpDisassemblyRange(t *testing.T, machine *Machine, start, length uint32, label string) {
	t.Helper()

	lines, err := cpu.DisassembleMemoryRange(machine.bus.CPUAddressBus(), start, length)
	if err != nil {
		t.Fatalf("disassemble %s %06x+%x: %v", label, start, length, err)
	}

	fmt.Printf("disassembly %s %06x..%06x\n", label, start&0xFFFFFF, (start+length-1)&0xFFFFFF)
	for _, line := range lines {
		fmt.Printf("%s\n", line.String())
	}
}

func dumpMFPState(t *testing.T, machine *Machine, label string) {
	t.Helper()

	readByte := func(addr uint32) byte {
		value, err := machine.bus.Read(cpu.Byte, addr)
		if err != nil {
			t.Fatalf("read mfp register %06x: %v", addr, err)
		}
		return byte(value)
	}

	fmt.Printf(
		"mfp %s gpip=%02x iera=%02x ierb=%02x ipra=%02x iprb=%02x isra=%02x isrb=%02x imra=%02x imrb=%02x vr=%02x tacr=%02x tbcr=%02x tcdcr=%02x tadr=%02x tbdr=%02x tcdr=%02x tddr=%02x\n",
		label,
		readByte(0xFFFA01),
		readByte(0xFFFA07),
		readByte(0xFFFA09),
		readByte(0xFFFA0B),
		readByte(0xFFFA0D),
		readByte(0xFFFA0F),
		readByte(0xFFFA11),
		readByte(0xFFFA13),
		readByte(0xFFFA15),
		readByte(0xFFFA17),
		readByte(0xFFFA19),
		readByte(0xFFFA1B),
		readByte(0xFFFA1D),
		readByte(0xFFFA1F),
		readByte(0xFFFA21),
		readByte(0xFFFA23),
		readByte(0xFFFA25),
	)
}

func TestDebugFontCopy(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	counts := map[uint32]int{}
	lastValues := map[uint32]uint32{}
	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		address := info.Address & 0xFFFFFF
		if !info.Write || address < 0x323c || address >= 0x3300 {
			return
		}
		pc := machine.cpu.Registers().PC & 0xFFFFFF
		counts[pc]++
		lastValues[address] = info.Value
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	type entry struct {
		pc    uint32
		count int
	}
	var entries []entry
	for pc, count := range counts {
		entries = append(entries, entry{pc: pc, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count == entries[j].count {
			return entries[i].pc < entries[j].pc
		}
		return entries[i].count > entries[j].count
	})
	if len(entries) > 12 {
		entries = entries[:12]
	}
	for _, e := range entries {
		fmt.Printf("pc=%06x count=%d\n", e.pc, e.count)
	}

	for _, addr := range []uint32{0x323c + 50, 0x323c + 52, 0x323c + 80, 0x323c + 82} {
		value, err := machine.ram.Read(cpu.Word, addr)
		if err != nil {
			t.Fatalf("read font field %06x: %v", addr, err)
		}
		fmt.Printf("field_%06x=%04x\n", addr, value)
	}
}

func TestDebugFontCopyWriters(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if pc < 0xE13AE0 || pc > 0xE13BE8 {
			return
		}
		fmt.Printf("pc=%06x d0=%08x d1=%08x a0=%08x a1=%08x a6=%08x a7=%08x ins=%s\n",
			pc,
			uint32(info.Registers.D[0]),
			uint32(info.Registers.D[1]),
			info.Registers.A[0],
			info.Registers.A[1],
			info.Registers.A[6],
			info.Registers.A[7],
			machine.decodeTraceInstruction(info),
		)
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		address := info.Address & 0xFFFFFF
		if !info.Write {
			return
		}
		if address != 0x323c+50 && address != 0x323c+52 && address != 0x323c+80 && address != 0x323c+82 {
			return
		}
		regs := machine.cpu.Registers()
		fmt.Printf("write pc=%06x addr=%06x size=%d value=%08x a0=%08x a1=%08x\n",
			regs.PC&0xFFFFFF,
			address,
			info.Size,
			info.Value,
			regs.A[0],
			regs.A[1],
		)
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}
}

func TestDebugFontCopyCompareSource(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	srcBase := uint32(0xE15718)
	dstBase := uint32(0x323c)
	const size = 0x5a

	src := make([]byte, size)
	for i := range src {
		value, err := machine.rom.Read(cpu.Byte, srcBase+uint32(i))
		if err != nil {
			t.Fatalf("read rom byte %06x: %v", srcBase+uint32(i), err)
		}
		src[i] = byte(value)
	}

	dst := make([]byte, size)
	for i := range dst {
		value, err := machine.ram.Read(cpu.Byte, dstBase+uint32(i))
		if err != nil {
			t.Fatalf("read ram byte %06x: %v", dstBase+uint32(i), err)
		}
		dst[i] = byte(value)
	}

	fmt.Printf("src=%s\n", hex.EncodeToString(src))
	fmt.Printf("dst=%s\n", hex.EncodeToString(dst))
	for i := range src {
		if src[i] != dst[i] {
			fmt.Printf("diff offset=%02x src=%02x dst=%02x\n", i, src[i], dst[i])
		}
	}
}

func TestDebugReachLowRAMIllegalSite(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	result, err := machine.RunUntil(cpu.RunUntilOptions{
		MaxInstructions: 2_000_000,
		StopOnPCRange:   &cpu.AddressRange{Start: 0x00027300, End: 0x00027310},
	})
	if err != nil {
		t.Fatalf("run until low RAM site: %v", err)
	}
	fmt.Printf("reason=%v pc=%06x instructions=%d cycles=%d hasException=%v\n",
		result.Reason, result.PC&0xFFFFFF, result.Instructions, result.Cycles, result.HasException)

	regs := machine.Registers()
	fmt.Printf("regs pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a7=%08x\n",
		regs.PC&0xFFFFFF, regs.SR, uint32(regs.D[0]), uint32(regs.D[1]), uint32(regs.D[2]), uint32(regs.D[3]), regs.A[0], regs.A[1], regs.A[7])

	for addr := uint32(0x272f0); addr < 0x27320; addr += 2 {
		value, err := machine.ram.Read(cpu.Word, addr)
		if err != nil {
			t.Fatalf("read low RAM %06x: %v", addr, err)
		}
		fmt.Printf("%06x: %04x\n", addr, value)
	}
}

func TestDebugMFPStateAfterBoot(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	for _, addr := range []uint32{
		0xFFFA07, 0xFFFA09, 0xFFFA0B, 0xFFFA0D,
		0xFFFA0F, 0xFFFA11, 0xFFFA13, 0xFFFA15,
		0xFFFA17, 0xFFFA19, 0xFFFA1B, 0xFFFA1D,
		0xFFFA1F, 0xFFFA21, 0xFFFA23, 0xFFFA25,
	} {
		value, err := machine.bus.Read(cpu.Byte, addr)
		if err != nil {
			t.Fatalf("read mfp %06x: %v", addr, err)
		}
		fmt.Printf("%06x=%02x\n", addr&0xFFFFFF, value)
	}
}

func TestDebugFirstLateMFPInterrupt(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var logLines []string
	appendLine := func(line string) {
		logLines = append(logLines, line)
		if len(logLines) > 64 {
			logLines = logLines[len(logLines)-64:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		switch addr {
		case 0xFFFA09, 0xFFFA0D, 0xFFFA11, 0xFFFA15, 0xFFFA17, 0xFFFA1D, 0xFFFA23, 0xFFFA25:
			appendLine(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x",
				map[bool]string{true: "write", false: "read"}[info.Write],
				addr, info.Size, info.Value, machine.cpu.Registers().PC&0xFFFFFF))
		}
	})
	var hit *cpu.ExceptionInfo
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.Vector == 68 || info.Vector == 69 {
			copied := info
			hit = &copied
		}
	})

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if hit != nil {
			fmt.Printf("frame=%d vector=%d pc=%06x newpc=%06x opcode=%04x\n",
				frame,
				hit.Vector,
				hit.PC&0xFFFFFF,
				hit.NewPC&0xFFFFFF,
				hit.Opcode,
			)
			fmt.Printf("%s\n", strings.Join(logLines, "\n"))
			for _, addr := range []uint32{0xFFFA09, 0xFFFA0D, 0xFFFA11, 0xFFFA15, 0xFFFA17, 0xFFFA1D, 0xFFFA23, 0xFFFA25} {
				value, err := machine.bus.Read(cpu.Byte, addr)
				if err != nil {
					t.Fatalf("read mfp %06x: %v", addr, err)
				}
				fmt.Printf("%06x=%02x\n", addr&0xFFFFFF, value)
			}
			return
		}
	}

	t.Fatalf("did not reach late MFP vector 68/69 within 400 frames")
}

func TestDebugFirstVector68Interrupt(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var logLines []string
	appendLine := func(line string) {
		logLines = append(logLines, line)
		if len(logLines) > 64 {
			logLines = logLines[len(logLines)-64:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		switch addr {
		case 0xFFFA09, 0xFFFA0D, 0xFFFA11, 0xFFFA15, 0xFFFA17, 0xFFFA1D, 0xFFFA23, 0xFFFA25:
			appendLine(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x",
				map[bool]string{true: "write", false: "read"}[info.Write],
				addr, info.Size, info.Value, machine.cpu.Registers().PC&0xFFFFFF))
		}
	})
	var hit *cpu.ExceptionInfo
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.Vector == 68 {
			copied := info
			hit = &copied
		}
	})

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if hit != nil {
			fmt.Printf("frame=%d vector=%d pc=%06x newpc=%06x opcode=%04x\n",
				frame,
				hit.Vector,
				hit.PC&0xFFFFFF,
				hit.NewPC&0xFFFFFF,
				hit.Opcode,
			)
			fmt.Printf("%s\n", strings.Join(logLines, "\n"))
			for _, addr := range []uint32{0xFFFA09, 0xFFFA0D, 0xFFFA11, 0xFFFA15, 0xFFFA17, 0xFFFA1D, 0xFFFA23, 0xFFFA25} {
				value, err := machine.bus.Read(cpu.Byte, addr)
				if err != nil {
					t.Fatalf("read mfp %06x: %v", addr, err)
				}
				fmt.Printf("%06x=%02x\n", addr&0xFFFFFF, value)
			}
			return
		}
	}

	t.Fatalf("did not reach vector 68 within 400 frames")
}

func TestDebugTraceLowRAMExecution(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var hits []string
	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if pc < 0x22f0 || pc > 0x2320 {
			return
		}
		hits = append(hits, fmt.Sprintf("pc=%06x sr=%04x d0=%08x a0=%08x a1=%08x ins=%s",
			pc, info.SR, uint32(info.Registers.D[0]), info.Registers.A[0], info.Registers.A[1], machine.decodeTraceInstruction(info)))
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if !info.InstructionFetch || addr < 0x22f0 || addr > 0x2320 {
			return
		}
		hits = append(hits, fmt.Sprintf("fetch addr=%06x size=%d value=%08x pc=%06x",
			addr, info.Size, info.Value, machine.cpu.Registers().PC&0xFFFFFF))
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	if len(hits) == 0 {
		fmt.Println("no low RAM execution observed")
		return
	}
	fmt.Printf("%s\n", strings.Join(hits, "\n"))
}

func TestDebugScreenStateAfterPanicFrame(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	fmt.Printf("shifter_screen_base=%06x\n", machine.shifter.ScreenBase())
	for _, addr := range []uint32{0x044e, 0x0452, 0x0456, 0x045a, 0x045e} {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read lowmem %06x: %v", addr, err)
		}
		fmt.Printf("%06x=%08x\n", addr, value)
	}
}

func TestDebugEmuTOSPanicRecord(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	for _, addr := range []uint32{0x380, 0x3c4, 0x3c8, 0x3cc, 0x3ce, 0x3d0, 0x3d2} {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read panic record %06x: %v", addr, err)
		}
		fmt.Printf("%06x=%08x\n", addr, value)
	}
}

func TestDebugWritesNear2300(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	counts := map[uint32]int{}
	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if !info.Write || addr < 0x2200 || addr >= 0x2400 {
			return
		}
		pc := machine.cpu.Registers().PC & 0xFFFFFF
		counts[pc]++
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	type entry struct {
		pc    uint32
		count int
	}
	var entries []entry
	for pc, count := range counts {
		entries = append(entries, entry{pc: pc, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count == entries[j].count {
			return entries[i].pc < entries[j].pc
		}
		return entries[i].count > entries[j].count
	})
	if len(entries) > 20 {
		entries = entries[:20]
	}
	for _, e := range entries {
		fmt.Printf("pc=%06x count=%d\n", e.pc, e.count)
	}
	for _, addr := range []uint32{0x22f8, 0x22fa, 0x22fc, 0x22fe, 0x2300, 0x2302, 0x2304, 0x2306} {
		value, err := machine.ram.Read(cpu.Word, addr)
		if err != nil {
			t.Fatalf("read near2300 %06x: %v", addr, err)
		}
		fmt.Printf("%06x=%04x\n", addr, value)
	}
}

func TestDebugFirstPanicRecordSet(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 64 {
			history = history[len(history)-64:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pushHistory(fmt.Sprintf("pc=%06x sr=%04x d0=%08x a0=%08x a1=%08x a7=%08x ins=%s",
			info.PC&0xFFFFFF, info.SR, uint32(info.Registers.D[0]), info.Registers.A[0], info.Registers.A[1], info.Registers.A[7], machine.decodeTraceInstruction(info)))
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR))
	})

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, err := machine.ram.Read(cpu.Long, 0x380)
		if err != nil {
			t.Fatalf("read proc_lives: %v", err)
		}
		if lives == 0x12345678 {
			fmt.Printf("panic set on frame=%d\n", frame)
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			enum, _ := machine.ram.Read(cpu.Long, 0x3c4)
			fmt.Printf("proc_enum=%08x\n", enum)
			for _, addr := range []uint32{0x3cc, 0x3ce, 0x3d0, 0x3d2, 0x3d4, 0x3d6} {
				value, _ := machine.ram.Read(cpu.Word, addr)
				fmt.Printf("%06x=%04x\n", addr, value)
			}
			return
		}
	}

	t.Fatalf("panic record was not set within 120 frames")
}

func TestDebugPanicWindowBytes(t *testing.T) {
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

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if pc < 0xE02BD0 || pc > 0xE09450 {
			return
		}
		pushHistory(fmt.Sprintf("pc=%06x bytes=%x sr=%04x a7=%08x ins=%s",
			pc, info.Bytes, info.SR, info.Registers.A[7], machine.decodeTraceInstruction(info)))
	})

	for frame := 0; frame < 60; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			return
		}
	}

	t.Fatalf("panic record was not set within 60 frames")
}

func TestDebugPanicTriggerPoint(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var history []string
	pushHistory := func(line string) {
		history = append(history, line)
		if len(history) > 128 {
			history = history[len(history)-128:]
		}
	}

	triggered := false
	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pushHistory(fmt.Sprintf("pc=%06x bytes=%x sr=%04x a7=%08x ins=%s",
			info.PC&0xFFFFFF, info.Bytes, info.SR, info.Registers.A[7], machine.decodeTraceInstruction(info)))
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR))
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if !info.Write {
			return
		}
		if addr == 0x380 && info.Value == 0x12345678 {
			triggered = true
			pushHistory(fmt.Sprintf("write %06x=%08x at pc=%06x", addr, info.Value, machine.cpu.Registers().PC&0xFFFFFF))
		}
		if addr >= 0x380 && addr < 0x3e0 {
			pushHistory(fmt.Sprintf("write %06x size=%d value=%08x pc=%06x",
				addr, info.Size, info.Value, machine.cpu.Registers().PC&0xFFFFFF))
		}
	})

	for frame := 0; frame < 60; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if triggered {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			return
		}
	}

	t.Fatalf("panic trigger not observed within 60 frames")
}

func TestDebugPanicWithoutIRQs(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	machine.irqSources = nil

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, err := machine.ram.Read(cpu.Long, 0x380)
		if err != nil {
			t.Fatalf("read proc_lives: %v", err)
		}
		if lives == 0x12345678 {
			t.Fatalf("panic record was still set with IRQ delivery disabled at frame %d", frame)
		}
	}

	fmt.Println("no panic with IRQ delivery disabled")
}

func TestDebugPanicWithoutVBL(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	filteredClocked := make([]devices.Clocked, 0, len(machine.clocked))
	for _, device := range machine.clocked {
		if device == machine.vbl {
			continue
		}
		filteredClocked = append(filteredClocked, device)
	}
	machine.clocked = filteredClocked

	filteredIRQs := make([]devices.InterruptSource, 0, len(machine.irqSources))
	for _, source := range machine.irqSources {
		if source == machine.vbl {
			continue
		}
		filteredIRQs = append(filteredIRQs, source)
	}
	machine.irqSources = filteredIRQs

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			t.Fatalf("panic record was still set without VBL at frame %d", frame)
		}
	}

	fmt.Println("no panic without VBL")
}

func TestDebugPanicWithoutMFP(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	filteredClocked := make([]devices.Clocked, 0, len(machine.clocked))
	for _, device := range machine.clocked {
		if device == machine.mfp {
			continue
		}
		filteredClocked = append(filteredClocked, device)
	}
	machine.clocked = filteredClocked

	filteredIRQs := make([]devices.InterruptSource, 0, len(machine.irqSources))
	for _, source := range machine.irqSources {
		if source == machine.mfp {
			continue
		}
		filteredIRQs = append(filteredIRQs, source)
	}
	machine.irqSources = filteredIRQs

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			t.Fatalf("panic record was still set without MFP at frame %d", frame)
		}
	}

	fmt.Println("no panic without MFP")
}

func TestDebugPanicWithoutTimerD(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 30; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		ierb, _ := machine.bus.Read(cpu.Byte, 0xFFFA09)
		imrb, _ := machine.bus.Read(cpu.Byte, 0xFFFA15)
		_ = machine.bus.Write(cpu.Byte, 0xFFFA09, ierb&^0x10)
		_ = machine.bus.Write(cpu.Byte, 0xFFFA15, imrb&^0x10)
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			fmt.Printf("panic record set at frame %d with timer D masked\n", frame)
			return
		}
	}

	fmt.Println("no panic with timer D masked")
}

func TestDebugPanicWithoutTimerC(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		ierb, _ := machine.bus.Read(cpu.Byte, 0xFFFA09)
		imrb, _ := machine.bus.Read(cpu.Byte, 0xFFFA15)
		_ = machine.bus.Write(cpu.Byte, 0xFFFA09, ierb&^0x20)
		_ = machine.bus.Write(cpu.Byte, 0xFFFA15, imrb&^0x20)
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			t.Fatalf("panic record was still set with timer C masked at frame %d", frame)
		}
	}

	fmt.Println("no panic with timer C masked")
}

func TestDebugFirstMixedIRQException(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 20; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	result, err := machine.RunUntil(cpu.RunUntilOptions{
		MaxInstructions: 1_000_000,
		StopOnException: true,
	})
	if err != nil {
		t.Fatalf("run until exception: %v", err)
	}
	if !result.HasException {
		t.Fatalf("expected an exception, got reason=%v pc=%06x instructions=%d cycles=%d",
			result.Reason, result.PC&0xFFFFFF, result.Instructions, result.Cycles)
	}

	info := result.Exception
	fmt.Printf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x group0=%v faultvalid=%v\n",
		info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR, info.Group0, info.FaultValid)
	regs := machine.Registers()
	fmt.Printf("regs pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a7=%08x\n",
		regs.PC&0xFFFFFF, regs.SR, uint32(regs.D[0]), uint32(regs.D[1]), uint32(regs.D[2]), uint32(regs.D[3]), regs.A[0], regs.A[1], regs.A[7])
}

func TestDebugFirstNonInterruptException(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 20; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	for steps := 0; steps < 500_000; steps++ {
		result, err := machine.RunUntil(cpu.RunUntilOptions{
			MaxInstructions: 1,
			StopOnException: true,
		})
		if err != nil {
			t.Fatalf("run until exception: %v", err)
		}
		if !result.HasException {
			continue
		}
		if result.Exception.Vector >= 24 {
			continue
		}

		info := result.Exception
		fmt.Printf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x group0=%v faultvalid=%v\n",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR, info.Group0, info.FaultValid)
		regs := machine.Registers()
		fmt.Printf("regs pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a7=%08x\n",
			regs.PC&0xFFFFFF, regs.SR, uint32(regs.D[0]), uint32(regs.D[1]), uint32(regs.D[2]), uint32(regs.D[3]), regs.A[0], regs.A[1], regs.A[7])
		return
	}

	fmt.Println("no non-interrupt exception observed")
}

func TestDebugTraceIllegalVector(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 20; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	var hit bool
	var caught cpu.ExceptionInfo
	machine.EnableTrace("", nil)
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.Vector != 4 {
			return
		}
		hit = true
		caught = info
	})

	for frame := 0; frame < 120 && !hit; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	if !hit {
		t.Fatalf("illegal vector was not observed")
	}

	fmt.Printf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x group0=%v faultvalid=%v\n",
		caught.Vector, caught.PC&0xFFFFFF, caught.NewPC&0xFFFFFF, caught.Opcode, caught.SR, caught.NewSR, caught.Group0, caught.FaultValid)
	regs := machine.Registers()
	fmt.Printf("regs pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a7=%08x\n",
		regs.PC&0xFFFFFF, regs.SR, uint32(regs.D[0]), uint32(regs.D[1]), uint32(regs.D[2]), uint32(regs.D[3]), regs.A[0], regs.A[1], regs.A[7])
}

func TestDebugTraceLateIllegalHistory(t *testing.T) {
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
		if len(history) > 96 {
			history = history[len(history)-96:]
		}
	}

	var hit bool
	var caught cpu.ExceptionInfo
	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pushHistory(fmt.Sprintf("pc=%06x bytes=%x sr=%04x d0=%08x d1=%08x a0=%08x a1=%08x a7=%08x ins=%s",
			info.PC&0xFFFFFF, info.Bytes, info.SR, uint32(info.Registers.D[0]), uint32(info.Registers.D[1]), info.Registers.A[0], info.Registers.A[1], info.Registers.A[7], machine.decodeTraceInstruction(info)))
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.Vector != 4 {
			return
		}
		hit = true
		caught = info
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR))
	})

	for frame := 20; frame < 120 && !hit; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	if !hit {
		t.Fatalf("late illegal vector was not observed")
	}

	fmt.Printf("%s\n", strings.Join(history, "\n"))
	fmt.Printf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x\n",
		caught.Vector, caught.PC&0xFFFFFF, caught.NewPC&0xFFFFFF, caught.Opcode, caught.SR, caught.NewSR)
	for _, addr := range []uint32{0x2200, 0x2202, 0x2204, 0x2206, 0x2208, 0x220a, 0x220c, 0x220e, 0x2210, 0x2212, 0x2214, 0x2216, 0x2218, 0x221a, 0x221c, 0x221e, 0x2220, 0x2222, 0x2224, 0x2226, 0x2228, 0x222a, 0x222c, 0x222e, 0x2230, 0x2232, 0x2234} {
		value, _ := machine.ram.Read(cpu.Word, addr)
		fmt.Printf("%06x=%04x\n", addr, value)
	}
}

func TestDebugPanicWithInstructionGranularity(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 120; frame++ {
		target := machine.Cycles() + machine.frameCycles
		for machine.Cycles() < target {
			if _, err := machine.RunUntil(cpu.RunUntilOptions{MaxInstructions: 1}); err != nil {
				t.Fatalf("run single instruction at frame %d: %v", frame, err)
			}
		}
		lives, err := machine.ram.Read(cpu.Long, 0x380)
		if err != nil {
			t.Fatalf("read proc_lives: %v", err)
		}
		if lives == 0x12345678 {
			t.Fatalf("panic record was set with instruction-granularity scheduling at frame %d", frame)
		}
	}

	fmt.Println("no panic with instruction-granularity scheduling")
}

func TestDebugSingleStepLateBranchWindow(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := 0; frame < 20; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	result, err := machine.RunUntil(cpu.RunUntilOptions{
		MaxInstructions: 4_000_000,
		StopOnPCRange:   &cpu.AddressRange{Start: 0xE03CFE, End: 0xE03CFE},
	})
	if err != nil {
		t.Fatalf("run until branch window: %v", err)
	}
	if result.PC&0xFFFFFF != 0xE03CFE {
		t.Fatalf("unexpected stop pc=%06x reason=%v", result.PC&0xFFFFFF, result.Reason)
	}

	for step := 0; step < 12; step++ {
		regs := machine.Registers()
		fmt.Printf("before step=%d pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x a7=%08x\n",
			step, regs.PC&0xFFFFFF, regs.SR, uint32(regs.D[0]), uint32(regs.D[1]), uint32(regs.D[2]), regs.A[7])

		result, err := machine.RunUntil(cpu.RunUntilOptions{
			MaxInstructions: 1,
			StopOnException: true,
		})
		if err != nil {
			t.Fatalf("single-step %d: %v", step, err)
		}
		regs = machine.Registers()
		fmt.Printf("after  step=%d pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x a7=%08x reason=%v hasException=%v\n",
			step, regs.PC&0xFFFFFF, regs.SR, uint32(regs.D[0]), uint32(regs.D[1]), uint32(regs.D[2]), regs.A[7], result.Reason, result.HasException)
		if result.HasException {
			info := result.Exception
			fmt.Printf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x\n",
				info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR)
			return
		}
	}
}

func TestDebugLateIllegalStackWindow(t *testing.T) {
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
		if len(history) > 160 {
			history = history[len(history)-160:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE03CF0 && pc <= 0xE03D10) || (pc >= 0xE00800 && pc <= 0xE00920) {
			pushHistory(fmt.Sprintf("pc=%06x bytes=%x sr=%04x a7=%08x ins=%s",
				pc, info.Bytes, info.SR, info.Registers.A[7], machine.decodeTraceInstruction(info)))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr < 0x2220 || addr >= 0x2240 {
			return
		}
		regs := machine.cpu.Registers()
		pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
			traceAccessKind(info.Write), addr, info.Size, info.Value, regs.PC&0xFFFFFF, regs.A[7]))
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR))
	})

	for frame := 20; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			return
		}
	}

	t.Fatalf("panic record was not set")
}

func TestDebugIRQReturnStackWindow(t *testing.T) {
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
		if len(history) > 200 {
			history = history[len(history)-200:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE00750 && pc <= 0xE00850) || (pc >= 0xE00900 && pc <= 0xE00920) {
			pushHistory(fmt.Sprintf("pc=%06x bytes=%x sr=%04x a7=%08x ins=%s",
				pc, info.Bytes, info.SR, info.Registers.A[7], machine.decodeTraceInstruction(info)))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr < 0x0e00 || addr >= 0x0e40 {
			return
		}
		regs := machine.cpu.Registers()
		pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
			traceAccessKind(info.Write), addr, info.Size, info.Value, regs.PC&0xFFFFFF, regs.A[7]))
	})
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		pushHistory(fmt.Sprintf("exception vector=%d pc=%06x newpc=%06x opcode=%04x sr=%04x newsr=%04x",
			info.Vector, info.PC&0xFFFFFF, info.NewPC&0xFFFFFF, info.Opcode, info.SR, info.NewSR))
	})

	for frame := 20; frame < 120; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		lives, _ := machine.ram.Read(cpu.Long, 0x380)
		if lives == 0x12345678 {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			return
		}
	}

	t.Fatalf("panic record was not set")
}

func TestDebugLateVBLTimerCStackWindow(t *testing.T) {
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
		if len(history) > 240 {
			history = history[len(history)-240:]
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if (pc >= 0xE0075E && pc <= 0xE00840) || (pc >= 0xE13C20 && pc <= 0xE13D60) {
			pushHistory(fmt.Sprintf("pc=%06x bytes=%x sr=%04x a7=%08x ins=%s",
				pc, info.Bytes, info.SR, info.Registers.A[7], machine.decodeTraceInstruction(info)))
		}
	})
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if (addr >= 0x000740 && addr < 0x000780) ||
			(addr >= 0x0007440 && addr < 0x0007480) ||
			(addr >= 0x007580 && addr < 0x0075c0) ||
			(addr >= 0x000454 && addr < 0x000460) ||
			addr == 0xFFFA0D || addr == 0xFFFA11 || addr == 0xFFFA15 || addr == 0xFFFA1D {
			regs := machine.cpu.Registers()
			pushHistory(fmt.Sprintf("%s %06x size=%d value=%08x pc=%06x a7=%08x",
				traceAccessKind(info.Write), addr, info.Size, info.Value, regs.PC&0xFFFFFF, regs.A[7]))
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

func TestDebugDisassembleLateTrapPaths(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	dumpDisassemblyRange(t, machine, 0xE00072, 0x90, "early-probe-a")
	dumpDisassemblyRange(t, machine, 0xE0012C, 0x80, "early-probe-b")
	dumpDisassemblyRange(t, machine, 0xE00C4A, 0x40, "early-probe-c")
	dumpDisassemblyRange(t, machine, 0xE02120, 0xE0, "early-memory-init")
	dumpDisassemblyRange(t, machine, 0xE0E6DA, 0x50, "late-frame-builder")
	dumpDisassemblyRange(t, machine, 0xE0E640, 0x40, "late-frame-setup")
	dumpDisassemblyRange(t, machine, 0xE13060, 0x50, "late-search-helper")
	dumpDisassemblyRange(t, machine, 0xE13080, 0x30, "late-frame-reader")
	dumpDisassemblyRange(t, machine, 0xE13160, 0x60, "late-allocator")
	dumpDisassemblyRange(t, machine, 0xE13258, 0xC8, "late-descriptor-builder")
	dumpDisassemblyRange(t, machine, 0xE12690, 0x90, "late-call-prep-a")
	dumpDisassemblyRange(t, machine, 0xE12790, 0x80, "late-call-prep-b")
	dumpDisassemblyRange(t, machine, 0xE138F4, 0x60, "late-pointer-builder")
	dumpDisassemblyRange(t, machine, 0xE12800, 0xD0, "trap-dispatch-branch")
	dumpDisassemblyRange(t, machine, 0xE12870, 0x60, "trap-dispatch-prelude")
	dumpDisassemblyRange(t, machine, 0xE128CA, 0x120, "trap-dispatch")
	dumpDisassemblyRange(t, machine, 0xE1298C, 0x60, "trap-dispatch-tail")
	dumpDisassemblyRange(t, machine, 0xE12B80, 0x40, "trap-dispatch-exit")
	dumpDisassemblyRange(t, machine, 0xE12D1C, 0x80, "trap1-handler")
	dumpDisassemblyRange(t, machine, 0xE12D9A, 0x80, "late-trap-callee")
	dumpDisassemblyRange(t, machine, 0xE133B0, 0x40, "late-pointer-helper")
	dumpDisassemblyRange(t, machine, 0xE13C34, 0x120, "late-trap-wrapper")
	dumpDisassemblyRange(t, machine, 0xE11EE8, 0x80, "memory-chain-helper-a")
	dumpDisassemblyRange(t, machine, 0xE11F40, 0xC0, "memory-chain-helper-b")
}

func TestDebugDisassembleAltRAMCallers(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	dumpDisassemblyRange(t, machine, 0xE00C2E, 0x62, "alt-ram-probe-helper")
	dumpDisassemblyRange(t, machine, 0xE013D0, 0x50, "alt-ram-caller-a")
	dumpDisassemblyRange(t, machine, 0xE0CF10, 0x60, "alt-ram-caller-b")
	dumpDisassemblyRange(t, machine, 0xE0E150, 0x70, "alt-ram-init-helper")
	dumpDisassemblyRange(t, machine, 0xE13258, 0xC8, "alt-ram-builder")
}

func TestDebugLateIllegalInstructionSite(t *testing.T) {
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

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if pc >= 0xE24040 && pc <= 0xE240A0 {
			pushHistory(fmt.Sprintf(
				"pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a7=%08x ins=%s",
				pc,
				info.SR,
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[1]),
				uint32(info.Registers.D[2]),
				uint32(info.Registers.D[3]),
				info.Registers.A[0],
				info.Registers.A[1],
				info.Registers.A[7],
				machine.decodeTraceInstruction(info),
			))
		}
	})
	var found *cpu.ExceptionInfo
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.Vector == 4 && info.PC&0xFFFFFF == 0xE24088 {
			captured := info
			found = &captured
		}
	})

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if found != nil {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			fmt.Printf("illegal pc=%06x opcode=%04x newpc=%06x sr=%04x\n",
				found.PC&0xFFFFFF,
				found.Opcode,
				found.NewPC&0xFFFFFF,
				found.SR,
			)
			dumpDisassemblyRange(t, machine, 0xE24040, 0x70, "late-illegal-site")
			return
		}
	}

	t.Fatalf("did not reach late illegal instruction site within 400 frames")
}

func TestDebugLateAddressErrorSite(t *testing.T) {
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

	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		if pc >= 0xE25680 && pc <= 0xE256E0 {
			pushHistory(fmt.Sprintf(
				"pc=%06x sr=%04x d0=%08x d1=%08x d2=%08x d3=%08x a0=%08x a1=%08x a2=%08x a3=%08x a7=%08x ins=%s",
				pc,
				info.SR,
				uint32(info.Registers.D[0]),
				uint32(info.Registers.D[1]),
				uint32(info.Registers.D[2]),
				uint32(info.Registers.D[3]),
				info.Registers.A[0],
				info.Registers.A[1],
				info.Registers.A[2],
				info.Registers.A[3],
				info.Registers.A[7],
				machine.decodeTraceInstruction(info),
			))
		}
	})
	var found *cpu.ExceptionInfo
	machine.cpu.SetExceptionTracer(func(info cpu.ExceptionInfo) {
		if info.Vector == 3 && info.PC&0xFFFFFF == 0xE256B8 {
			captured := info
			found = &captured
		}
	})

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
		if found != nil {
			fmt.Printf("%s\n", strings.Join(history, "\n"))
			fmt.Printf("address-error pc=%06x opcode=%04x newpc=%06x sr=%04x fault-valid=%t fault=%06x\n",
				found.PC&0xFFFFFF,
				found.Opcode,
				found.NewPC&0xFFFFFF,
				found.SR,
				found.FaultValid,
				found.FaultAddress&0xFFFFFF,
			)
			for _, addr := range []uint32{0x0000D598, 0x0000D5B0, 0x00001140} {
				var words []string
				for i := 0; i < 24; i += 2 {
					value, err := machine.ram.Read(cpu.Word, addr+uint32(i))
					if err != nil {
						words = append(words, "????")
						continue
					}
					words = append(words, fmt.Sprintf("%04x", value))
				}
				fmt.Printf("mem %06x: %s\n", addr, strings.Join(words, " "))
			}
			dumpDisassemblyRange(t, machine, 0xE25680, 0x70, "late-address-error-site")
			return
		}
	}

	t.Fatalf("did not reach late address error site within 400 frames")
}

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

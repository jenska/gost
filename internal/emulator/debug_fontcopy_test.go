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

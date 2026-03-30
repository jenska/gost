//go:build debugtests
// +build debugtests

package emulator

import (
	"encoding/hex"
	"fmt"
	"sort"
	"testing"

	"github.com/jenska/gost/internal/assets"
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

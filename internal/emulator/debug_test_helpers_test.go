//go:build debugtests
// +build debugtests

package emulator

import (
	"fmt"
	"testing"

	cpu "github.com/jenska/m68kemu"
)

func dumpDisassemblyRange(t *testing.T, machine *Machine, start, length uint32, label string) {
	t.Helper()

	lines, err := cpu.DisassembleMemoryRange(machine.bus, start, length)
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

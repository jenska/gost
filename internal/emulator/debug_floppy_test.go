//go:build debugtests
// +build debugtests

package emulator

import (
	"fmt"
	"os"
	"testing"

	"github.com/jenska/gost/internal/assets"
	cpu "github.com/jenska/m68kemu"
)

func TestDebugPDATS321FloppyTraffic(t *testing.T) {
	disk, err := os.ReadFile("/Users/jens/projects/gost/downloads/atari-st/PDATS321.st")
	if err != nil {
		t.Fatalf("read floppy image: %v", err)
	}
	image := NewDiskImage(disk)

	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if err := machine.InsertFloppy(0, image); err != nil {
		t.Fatalf("insert floppy: %v", err)
	}

	var traffic int
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr < 0xFF8600 || addr >= 0xFF8610 {
			return
		}
		traffic++
		regs := machine.Registers()
		fmt.Printf("%s addr=%06x size=%d value=%08x pc=%06x sr=%04x\n",
			traceAccessKind(info.Write), addr, info.Size, info.Value, regs.PC&0xFFFFFF, regs.SR)
	})
	defer machine.cpu.SetBusTracer(nil)

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	fmt.Printf("fdc traffic=%d pc=%06x sr=%04x\n", traffic, machine.Registers().PC&0xFFFFFF, machine.Registers().SR)
}

func TestDebugPDATS321DMABuffer(t *testing.T) {
	disk, err := LoadDiskImage("/Users/jens/projects/gost/downloads/atari-st/PDATS321.msa")
	if err != nil {
		t.Fatalf("load floppy image: %v", err)
	}

	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if err := machine.InsertFloppy(0, disk); err != nil {
		t.Fatalf("insert floppy: %v", err)
	}

	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	for _, base := range []uint32{0x001004, 0x002004, 0x004004} {
		fmt.Printf("dma=%06x", base)
		for i := 0; i < 16; i++ {
			value, err := machine.ram.Read(cpu.Byte, base+uint32(i))
			if err != nil {
				t.Fatalf("read dma buffer %06x: %v", base+uint32(i), err)
			}
			fmt.Printf(" %02x", byte(value))
		}
		fmt.Println()
	}
}

func TestDebugPDATS321DesktopOpenDriveA(t *testing.T) {
	boot := func(withDisk bool) *Machine {
		t.Helper()
		machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
		if err != nil {
			t.Fatalf("create machine: %v", err)
		}
		if withDisk {
			disk, err := LoadDiskImage("/Users/jens/projects/gost/downloads/atari-st/PDATS321.msa")
			if err != nil {
				t.Fatalf("load floppy image: %v", err)
			}
			if err := machine.InsertFloppy(0, disk); err != nil {
				t.Fatalf("insert floppy: %v", err)
			}
		}
		for frame := 0; frame < 400; frame++ {
			if _, err := machine.StepFrame(); err != nil {
				t.Fatalf("step frame %d: %v", frame, err)
			}
		}
		return machine
	}

	doubleClick := func(machine *Machine, x, y int) {
		t.Helper()
		moveMouseTo(t, machine, x, y)
		clickMouse(t, machine)
		clickMouse(t, machine)
	}

	base := boot(false)
	withDisk := boot(true)

	const targetX, targetY = 44, 62
	doubleClick(base, targetX, targetY)
	doubleClick(withDisk, targetX, targetY)

	baseFrame := append([]byte(nil), base.FrameBuffer()...)
	withDiskFrame := withDisk.FrameBuffer()
	changed := 0
	for i := range baseFrame {
		if baseFrame[i] != withDiskFrame[i] {
			changed++
		}
	}
	fmt.Printf("drive-a target=(%d,%d) changed-bytes=%d\n", targetX, targetY, changed)
}

func TestDebugPDATS321DesktopSearchDriveA(t *testing.T) {
	for _, target := range [][2]int{
		{20, 22}, {28, 22}, {36, 22}, {44, 22},
		{20, 30}, {28, 30}, {36, 30}, {44, 30},
		{20, 38}, {28, 38}, {36, 38}, {44, 38},
		{20, 46}, {28, 46}, {36, 46}, {44, 46},
	} {
		machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
		if err != nil {
			t.Fatalf("create machine: %v", err)
		}
		disk, err := LoadDiskImage("/Users/jens/projects/gost/downloads/atari-st/PDATS321.msa")
		if err != nil {
			t.Fatalf("load floppy image: %v", err)
		}
		if err := machine.InsertFloppy(0, disk); err != nil {
			t.Fatalf("insert floppy: %v", err)
		}
		for frame := 0; frame < 400; frame++ {
			if _, err := machine.StepFrame(); err != nil {
				t.Fatalf("step frame %d: %v", frame, err)
			}
		}
		before := append([]byte(nil), machine.FrameBuffer()...)
		moveMouseTo(t, machine, target[0], target[1])
		quickDoubleClick(t, machine)
		for i := 0; i < 120; i++ {
			if _, err := machine.StepFrame(); err != nil {
				t.Fatalf("settle frame %d: %v", i, err)
			}
		}
		after := machine.FrameBuffer()
		changed := 0
		for i := range before {
			if before[i] != after[i] {
				changed++
			}
		}
		fmt.Printf("probe target=(%d,%d) changed-bytes=%d\n", target[0], target[1], changed)
	}
}

func TestDebugPDATS321AltAOpensDrive(t *testing.T) {
	boot := func(withDisk bool) *Machine {
		t.Helper()
		machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
		if err != nil {
			t.Fatalf("create machine: %v", err)
		}
		if withDisk {
			disk, err := LoadDiskImage("/Users/jens/projects/gost/downloads/atari-st/PDATS321.msa")
			if err != nil {
				t.Fatalf("load floppy image: %v", err)
			}
			if err := machine.InsertFloppy(0, disk); err != nil {
				t.Fatalf("insert floppy: %v", err)
			}
		}
		for frame := 0; frame < 400; frame++ {
			if _, err := machine.StepFrame(); err != nil {
				t.Fatalf("step frame %d: %v", frame, err)
			}
		}
		return machine
	}

	sendAltA := func(machine *Machine) {
		t.Helper()
		machine.PushKey(0x38, true)
		machine.PushKey(0x1E, true)
		machine.PushKey(0x1E, false)
		machine.PushKey(0x38, false)
		for i := 0; i < 120; i++ {
			if _, err := machine.StepFrame(); err != nil {
				t.Fatalf("post Alt+A frame %d: %v", i, err)
			}
		}
	}

	base := boot(false)
	withDisk := boot(true)
	sendAltA(base)
	sendAltA(withDisk)
	if err := base.DumpFramePNG("/tmp/gost-alt-a-no-disk.png"); err != nil {
		t.Fatalf("dump no-disk frame: %v", err)
	}
	if err := withDisk.DumpFramePNG("/tmp/gost-alt-a-with-disk.png"); err != nil {
		t.Fatalf("dump with-disk frame: %v", err)
	}

	baseFrame := append([]byte(nil), base.FrameBuffer()...)
	withDiskFrame := withDisk.FrameBuffer()
	changed := 0
	for i := range baseFrame {
		if baseFrame[i] != withDiskFrame[i] {
			changed++
		}
	}
	fmt.Printf("alt-a changed-bytes=%d\n", changed)
}

func TestDebugPDATS321AltAFDCTraffic(t *testing.T) {
	disk, err := LoadDiskImage("/Users/jens/projects/gost/downloads/atari-st/PDATS321.msa")
	if err != nil {
		t.Fatalf("load floppy image: %v", err)
	}

	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if err := machine.InsertFloppy(0, disk); err != nil {
		t.Fatalf("insert floppy: %v", err)
	}
	for frame := 0; frame < 400; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}

	logControl := func(info cpu.BusAccessInfo) bool {
		addr := info.Address & 0xFFFFFF
		return (addr >= 0xFF8604 && addr <= 0xFF860F) || addr == 0xFFFA01
	}

	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		if info.InstructionFetch || !logControl(info) {
			return
		}
		regs := machine.Registers()
		kind := "read"
		if info.Write {
			kind = "write"
		}
		fmt.Printf("%s addr=%06x size=%d value=%08x pc=%06x sr=%04x\n",
			kind, info.Address&0xFFFFFF, info.Size, info.Value, regs.PC&0xFFFFFF, regs.SR)
	})
	defer machine.cpu.SetBusTracer(nil)

	machine.PushKey(0x38, true)
	machine.PushKey(0x1E, true)
	machine.PushKey(0x1E, false)
	machine.PushKey(0x38, false)
	for frame := 0; frame < 40; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post Alt+A frame %d: %v", frame, err)
		}
	}
}

func TestDebugACSITrafficAtBoot(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	var lastControl uint16
	var acsiWrites int
	var acsiStatusReads int
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr < 0xFF8600 || addr >= 0xFF8610 {
			return
		}
		if info.Write && addr == 0xFF8606 {
			lastControl = uint16(info.Value)
			fmt.Printf("dmactl=%04x pc=%06x\n", lastControl, machine.Registers().PC&0xFFFFFF)
			return
		}
		if info.Write && addr == 0xFF8604 && lastControl&0x0008 != 0 {
			acsiWrites++
			fmt.Printf("acsi cmd raw=%08x hi=%02x lo=%02x size=%d ctl=%04x pc=%06x\n",
				info.Value, byte(info.Value>>8), byte(info.Value), info.Size, lastControl, machine.Registers().PC&0xFFFFFF)
			return
		}
		if !info.Write && addr == 0xFF8604 && lastControl&0x0008 != 0 {
			acsiStatusReads++
			fmt.Printf("acsi status read raw=%08x hi=%02x lo=%02x size=%d ctl=%04x pc=%06x\n",
				info.Value, byte(info.Value>>8), byte(info.Value), info.Size, lastControl, machine.Registers().PC&0xFFFFFF)
			return
		}
	})
	defer machine.cpu.SetBusTracer(nil)

	for frame := 0; frame < 600; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("step frame %d: %v", frame, err)
		}
	}
	fmt.Printf("acsi writes=%d status-reads=%d hd-bytes=%d\n", acsiWrites, acsiStatusReads, machine.HardDiskSizeBytes())
}

func moveMouseTo(t *testing.T, machine *Machine, targetX, targetY int) {
	t.Helper()
	for step := 0; step < 40; step++ {
		x, y, ok := machine.MousePosition()
		if !ok {
			t.Fatalf("mouse position unavailable")
		}
		dx := targetX - x
		dy := targetY - y
		if dx == 0 && dy == 0 {
			return
		}
		machine.PushMouse(dx, dy, 0)
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("move mouse frame %d: %v", step, err)
		}
	}
}

func clickMouse(t *testing.T, machine *Machine) {
	t.Helper()
	machine.PushMouse(0, 0, 0x02)
	if _, err := machine.StepFrame(); err != nil {
		t.Fatalf("mouse down: %v", err)
	}
	machine.PushMouse(0, 0, 0x00)
	if _, err := machine.StepFrame(); err != nil {
		t.Fatalf("mouse up: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-click frame %d: %v", i, err)
		}
	}
}

func quickDoubleClick(t *testing.T, machine *Machine) {
	t.Helper()
	for n := 0; n < 2; n++ {
		machine.PushMouse(0, 0, 0x02)
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("mouse down %d: %v", n, err)
		}
		machine.PushMouse(0, 0, 0x00)
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("mouse up %d: %v", n, err)
		}
	}
	for i := 0; i < 8; i++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-double-click frame %d: %v", i, err)
		}
	}
}

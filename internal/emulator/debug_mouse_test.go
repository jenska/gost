//go:build debugtests
// +build debugtests

package emulator

import (
	"fmt"
	"testing"

	"github.com/jenska/gost/internal/assets"
	cpu "github.com/jenska/m68kemu"
)

func TestDebugIKBDTrafficAfterDesktop(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	fmt.Printf("desktop reached pc=%06x sr=%04x\n", machine.Registers().PC&0xFFFFFF, machine.Registers().SR)

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		if addr < 0xFFFC00 || addr >= 0xFFFC04 || info.InstructionFetch {
			return
		}
		regs := machine.cpu.Registers()
		fmt.Printf("acia %s addr=%06x size=%d value=%08x pc=%06x sr=%04x\n",
			traceAccessKind(info.Write),
			addr,
			info.Size,
			info.Value,
			regs.PC&0xFFFFFF,
			regs.SR,
		)
	})

	fmt.Printf("inject mouse move\n")
	before := append([]byte(nil), machine.FrameBuffer()...)
	machine.PushMouse(12, 8, 0)

	for i := range 20 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}

	after := machine.FrameBuffer()
	changed := 0
	for idx := range before {
		if before[idx] != after[idx] {
			changed++
		}
	}
	fmt.Printf("framebuffer changed bytes=%d\n", changed)
}

func TestDebugIKBDMouseHandlerState(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	dumpDisassemblyRange(t, machine, 0xE00DD8, 0x60, "ikbd-packet-parser")

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		addr := info.Address & 0xFFFFFF
		pc := machine.cpu.Registers().PC & 0xFFFFFF
		if pc < 0xE00D80 || pc > 0xE00EC0 {
			return
		}
		if info.InstructionFetch {
			return
		}
		fmt.Printf("mouse-handler %s addr=%06x size=%d value=%08x pc=%06x a7=%08x\n",
			traceAccessKind(info.Write),
			addr,
			info.Size,
			info.Value,
			pc,
			machine.cpu.Registers().A[7],
		)
	})

	machine.PushMouse(12, 8, 0)
	for i := range 10 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}
}

func TestDebugIKBDMouseCallbackState(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	dumpDisassemblyRange(t, machine, 0xE16878, 0x80, "ikbd-mouse-callback")

	before := append([]byte(nil), machine.FrameBuffer()...)

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		pc := machine.cpu.Registers().PC & 0xFFFFFF
		if pc < 0xE16878 || pc > 0xE16920 {
			return
		}
		if info.InstructionFetch {
			return
		}
		addr := info.Address & 0xFFFFFF
		if info.Write || addr < 0x010000 {
			fmt.Printf("mouse-callback %s addr=%06x size=%d value=%08x pc=%06x a0=%08x a1=%08x d0=%08x d1=%08x\n",
				traceAccessKind(info.Write),
				addr,
				info.Size,
				info.Value,
				pc,
				machine.cpu.Registers().A[0],
				machine.cpu.Registers().A[1],
				uint32(machine.cpu.Registers().D[0]),
				uint32(machine.cpu.Registers().D[1]),
			)
		}
	})

	machine.PushMouse(12, 8, 0)
	for i := range 10 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}

	after := machine.FrameBuffer()
	changed := 0
	for idx := range before {
		if before[idx] != after[idx] {
			changed++
		}
	}
	fmt.Printf("callback-framebuffer changed bytes=%d\n", changed)
}

func TestDebugIKBDMouseDrawRoutine(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	dumpDisassemblyRange(t, machine, 0xE1ED90, 0x80, "ikbd-mouse-draw")

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		pc := machine.cpu.Registers().PC & 0xFFFFFF
		if pc < 0xE1ED90 || pc > 0xE1EE40 {
			return
		}
		if info.InstructionFetch {
			return
		}
		addr := info.Address & 0xFFFFFF
		if info.Write || addr < 0x10000 || (addr >= 0x0F0000 && addr < 0x100000) {
			fmt.Printf("mouse-draw %s addr=%06x size=%d value=%08x pc=%06x a0=%08x a1=%08x d0=%08x d1=%08x\n",
				traceAccessKind(info.Write),
				addr,
				info.Size,
				info.Value,
				pc,
				machine.cpu.Registers().A[0],
				machine.cpu.Registers().A[1],
				uint32(machine.cpu.Registers().D[0]),
				uint32(machine.cpu.Registers().D[1]),
			)
		}
	})

	before := append([]byte(nil), machine.FrameBuffer()...)
	machine.PushMouse(12, 8, 0)
	for i := range 10 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}
	queueCount, err := machine.bus.Read(cpu.Word, 0x0000C796)
	if err != nil {
		t.Fatalf("read draw queue count: %v", err)
	}
	fmt.Printf("draw-queue count=%d\n", queueCount)
	after := machine.FrameBuffer()
	changed := 0
	for idx := range before {
		if before[idx] != after[idx] {
			changed++
		}
	}
	fmt.Printf("draw-framebuffer changed bytes=%d\n", changed)
}

func TestDebugIKBDMouseDrawPipeline(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	dumpDisassemblyRange(t, machine, 0xE1FA24, 0x80, "ikbd-mouse-draw-helper")
	dumpDisassemblyRange(t, machine, 0xE24662, 0x80, "ikbd-mouse-plotter")

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		pc := machine.cpu.Registers().PC & 0xFFFFFF
		inRange := (pc >= 0xE1FA24 && pc <= 0xE1FB20) || (pc >= 0xE24662 && pc <= 0xE24740)
		if !inRange || info.InstructionFetch {
			return
		}
		addr := info.Address & 0xFFFFFF
		if info.Write || addr < 0x10000 || (addr >= 0x0F0000 && addr < 0x100000) {
			fmt.Printf("mouse-pipeline %s addr=%06x size=%d value=%08x pc=%06x a0=%08x a1=%08x a6=%08x d0=%08x d1=%08x\n",
				traceAccessKind(info.Write),
				addr,
				info.Size,
				info.Value,
				pc,
				machine.cpu.Registers().A[0],
				machine.cpu.Registers().A[1],
				machine.cpu.Registers().A[6],
				uint32(machine.cpu.Registers().D[0]),
				uint32(machine.cpu.Registers().D[1]),
			)
		}
	})

	before := append([]byte(nil), machine.FrameBuffer()...)
	machine.PushMouse(12, 8, 0)
	for i := range 10 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}
	after := machine.FrameBuffer()
	changed := 0
	for idx := range before {
		if before[idx] != after[idx] {
			changed++
		}
	}
	fmt.Printf("pipeline-framebuffer changed bytes=%d\n", changed)
}

func TestDebugIKBDMouseDrawQueueConsumer(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		if info.InstructionFetch {
			return
		}
		addr := info.Address & 0xFFFFFF
		if addr != 0x00C796 && (addr < 0x00ACE8 || addr >= 0x00ACF0) {
			return
		}
		regs := machine.cpu.Registers()
		fmt.Printf("draw-queue %s addr=%06x size=%d value=%08x pc=%06x sr=%04x\n",
			traceAccessKind(info.Write),
			addr,
			info.Size,
			info.Value,
			regs.PC&0xFFFFFF,
			regs.SR,
		)
	})

	machine.PushMouse(12, 8, 0)
	for i := range 100 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}

	queueCount, err := machine.bus.Read(cpu.Word, 0x0000C796)
	if err != nil {
		t.Fatalf("read draw queue count: %v", err)
	}
	fmt.Printf("draw-queue-final count=%d\n", queueCount)
}

func TestDebugDesktopVBLCallbackList(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	for _, addr := range []uint32{0x044e, 0x0452, 0x0456, 0x045a, 0x045e} {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read lowmem %06x: %v", addr, err)
		}
		fmt.Printf("%06x=%08x\n", addr, value)
	}
}

func TestDebugMouseVBLExecution(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for i := range 400 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", i, err)
		}
	}

	readWord := func(addr uint32) uint16 {
		value, err := machine.ram.Read(cpu.Word, addr)
		if err != nil {
			t.Fatalf("read word %06x: %v", addr, err)
		}
		return uint16(value)
	}
	readLong := func(addr uint32) uint32 {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read long %06x: %v", addr, err)
		}
		return value
	}

	vblVector, err := machine.bus.Read(cpu.Long, 0x70)
	if err != nil {
		t.Fatalf("read vbl vector: %v", err)
	}
	vblQueueBase := readLong(0x456)
	vblSlot0 := readLong(vblQueueBase)
	fmt.Printf("before mouse pc=%06x sr=%04x vblvec=%06x vblsem=%04x nvbls=%04x vblqueue=%06x slot0=%06x vbclock=%08x frclock=%08x\n",
		machine.Registers().PC&0xFFFFFF,
		machine.Registers().SR,
		vblVector&0xFFFFFF,
		readWord(0x452),
		readWord(0x454),
		vblQueueBase&0xFFFFFF,
		vblSlot0&0xFFFFFF,
		readLong(0x462),
		readLong(0x466),
	)

	var vblHits int
	var slotHits int
	machine.EnableTrace("", nil)
	machine.cpu.SetTracer(func(info cpu.TraceInfo) {
		pc := info.PC & 0xFFFFFF
		switch pc {
		case vblVector & 0xFFFFFF:
			vblHits++
			fmt.Printf("hit vbl-handler pc=%06x sr=%04x vbclock=%08x frclock=%08x vblsem=%04x\n",
				pc, info.SR, readLong(0x462), readLong(0x466), readWord(0x452))
		case vblSlot0 & 0xFFFFFF:
			slotHits++
			fmt.Printf("hit vbl-slot0 pc=%06x sr=%04x draw_flag=%02x hide_cnt=%04x\n",
				pc, info.SR, readWord(0x0AB4)&0xFF, readWord(0x09BE))
		}
	})

	machine.PushMouse(12, 8, 0)
	for i := range 20 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("post-mouse frame %d: %v", i, err)
		}
	}

	fmt.Printf("after mouse pc=%06x sr=%04x hits(vbl=%d slot0=%d) vblsem=%04x vbclock=%08x frclock=%08x queuecount=%04x\n",
		machine.Registers().PC&0xFFFFFF,
		machine.Registers().SR,
		vblHits,
		slotHits,
		readWord(0x452),
		readLong(0x462),
		readLong(0x466),
		readWord(0xC796),
	)
}

func TestDebugVBLStateEventuallyRecovers(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	readWord := func(addr uint32) uint16 {
		value, err := machine.ram.Read(cpu.Word, addr)
		if err != nil {
			t.Fatalf("read word %06x: %v", addr, err)
		}
		return uint16(value)
	}
	readLong := func(addr uint32) uint32 {
		value, err := machine.ram.Read(cpu.Long, addr)
		if err != nil {
			t.Fatalf("read long %06x: %v", addr, err)
		}
		return value
	}

	for frame := range 2000 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("frame %d: %v", frame, err)
		}
		vblQueueBase := readLong(0x456)
		slot0 := readLong(vblQueueBase)
		vblsem := readWord(0x452)
		if vblsem != 0 || slot0 != 0 {
			fmt.Printf("frame=%d pc=%06x sr=%04x vblsem=%04x slot0=%06x vbclock=%08x frclock=%08x\n",
				frame,
				machine.Registers().PC&0xFFFFFF,
				machine.Registers().SR,
				vblsem,
				slot0&0xFFFFFF,
				readLong(0x462),
				readLong(0x466),
			)
			return
		}
	}

	fmt.Printf("never recovered pc=%06x sr=%04x vblsem=%04x slot0=%06x vbclock=%08x frclock=%08x\n",
		machine.Registers().PC&0xFFFFFF,
		machine.Registers().SR,
		readWord(0x452),
		readLong(readLong(0x456))&0xFFFFFF,
		readLong(0x462),
		readLong(0x466),
	)
}

func TestDebugLateVBLStateWrites(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	for frame := range 300 {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("boot frame %d: %v", frame, err)
		}
	}

	machine.EnableTrace("", nil)
	machine.cpu.SetBusTracer(func(info cpu.BusAccessInfo) {
		if !info.Write || info.InstructionFetch {
			return
		}
		addr := info.Address & 0xFFFFFF
		switch addr {
		case 0x000452, 0x000453, 0x000454, 0x000455, 0x000456, 0x000457, 0x000458, 0x000459,
			0x0004CE, 0x0004CF, 0x0004D0, 0x0004D1:
			regs := machine.cpu.Registers()
			fmt.Printf("late-vbl %s addr=%06x size=%d value=%08x pc=%06x sr=%04x\n",
				traceAccessKind(info.Write), addr, info.Size, info.Value, regs.PC&0xFFFFFF, regs.SR)
		}
	})

	for frame := 300; frame < 450; frame++ {
		if _, err := machine.StepFrame(); err != nil {
			t.Fatalf("late frame %d: %v", frame, err)
		}
	}
}

func TestDebugDisassembleLateVBLWriters(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	dumpDisassemblyRange(t, machine, 0xE01290, 0xC0, "late-vbl-writer-a")
	dumpDisassemblyRange(t, machine, 0xE0AD10, 0x40, "late-vbl-writer-b")
	dumpDisassemblyRange(t, machine, 0xE00750, 0xC0, "vbl-handler")
}

func TestDebugMouseQueueDrainCaller(t *testing.T) {
	machine, err := NewMachine(DefaultConfig(), assets.DefaultROM())
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}

	dumpDisassemblyRange(t, machine, 0xE1FDE0, 0x80, "mouse-queue-drain-caller")
}

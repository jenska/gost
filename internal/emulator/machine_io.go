package emulator

import (
	"fmt"
	"io"

	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/devices"
	cpu "github.com/jenska/m68kemu"
)

const (
	// defaultROMHighAlias is the normal ST ROM mirror used for reset vectors.
	defaultROMHighAlias = 0xFC0000
	// secondaryROMAlias is the lower EmuTOS/TOS mirror used by real ST layouts.
	secondaryROMAlias = 0xE00000
	// stepQuantumCycles bounds how far devices can drift between CPU dispatches.
	stepQuantumCycles = 512
	// guestMouseXAddr and guestMouseYAddr are EmuTOS desktop mouse variables used
	// only by debug/test helpers to observe IKBD mouse motion.
	guestMouseXAddr = 0x001E6C
	guestMouseYAddr = 0x001E6E
)

type (
	// AudioSource provides audio samples from the emulated machine.
	AudioSource interface {
		// DrainMonoF32 fills the provided buffer with mono audio samples at the
		// machine's configured sample rate, returning the number of samples written.
		DrainMonoF32([]float32) int
		// OutputSampleRate returns the sample rate in Hz for audio output.
		OutputSampleRate() int
	}

	// Machine owns the complete emulated ST instance: CPU, bus, RAM, display,
	// interrupt sources, and currently attached I/O devices. Most fields remain
	// package-private so tests can inspect hardware state without making those
	// internals part of the public API.
	Machine struct {
		cfg           *config.Config
		bus           *cpu.Bus
		cpu           cpu.CPU
		ram           *devices.RAM
		overlayROM    *devices.OverlayROM
		memoryConfig  *devices.MemoryConfig
		shifter       *devices.Shifter
		mfp           *devices.MFP
		acia          *devices.ACIA
		fdc           *devices.FDC
		psg           *devices.PSG
		printer       *devices.PrinterPort
		clocked       []devices.Clocked
		irqSources    []devices.InterruptSource
		frameCycles   uint64
		cpuCycleCarry uint64
		traceWriter   io.Writer
		frameCounter  uint64
	}
)

func (m *Machine) FrameBuffer() []byte {
	return m.shifter.FrameBuffer()
}

// Dimensions returns the full shifter framebuffer dimensions.
func (m *Machine) Dimensions() (int, int) {
	return m.shifter.Dimensions()
}

func (m *Machine) DisplayFrameBuffer() []byte {
	return m.shifter.DisplayBuffer()
}

// DisplayDimensions returns the visible display-buffer dimensions after
// monitor-mode scaling and cropping have been applied.
func (m *Machine) DisplayDimensions() (int, int) {
	return m.shifter.DisplayDimensions()
}

// DisplayViewport reports the visible display rectangle within the raw
// framebuffer. It is useful for UIs that want to show exactly what the monitor
// would show while retaining access to the full shifter frame.
func (m *Machine) DisplayViewport() (x, y, width, height int) {
	return m.shifter.DisplayViewport()
}

// Registers returns a snapshot of the CPU register file.
func (m *Machine) Registers() cpu.Registers {
	return m.cpu.Registers()
}

// DebugState returns the CPU debugger state used by the frontend debugger.
func (m *Machine) DebugState() cpu.DebugState {
	return m.cpu.DebugState()
}

// ShifterDebugStats exposes the most recent shifter timing/render counters.
func (m *Machine) ShifterDebugStats() devices.ShifterDebugStats {
	return m.shifter.DebugStats()
}

// Cycles returns total CPU cycles executed since reset.
func (m *Machine) Cycles() uint64 {
	return m.cpu.Cycles()
}

// PushKey injects an IKBD keyboard make or break event.
func (m *Machine) PushKey(scancode byte, pressed bool) {
	m.acia.PushKey(scancode, pressed)
}

// PushMouse injects relative IKBD mouse motion and button state.
func (m *Machine) PushMouse(dx, dy int, buttons byte) {
	m.acia.PushMouse(dx, dy, buttons)
}

// MousePosition reads the current EmuTOS desktop mouse position from guest RAM.
// It returns ok=false until the desktop has initialized those variables.
func (m *Machine) MousePosition() (x, y int, ok bool) {
	width, height := m.Dimensions()
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}

	xValue, err := m.ram.Read(cpu.Word, guestMouseXAddr)
	if err != nil {
		return 0, 0, false
	}
	yValue, err := m.ram.Read(cpu.Word, guestMouseYAddr)
	if err != nil {
		return 0, 0, false
	}

	x = int(uint16(xValue))
	y = int(uint16(yValue))
	if x < 0 || x >= width || y < 0 || y >= height {
		return 0, 0, false
	}

	return x, y, true
}

func (m *Machine) AudioSource() AudioSource {
	return m.psg
}

// InsertFloppy inserts a decoded disk image into drive A. Drive B is not
// modeled yet, so side must be 0.
func (m *Machine) InsertFloppy(side int, image *DiskImage) error {
	if side != 0 {
		return fmt.Errorf("only floppy side 0 is supported")
	}
	if image == nil {
		return fmt.Errorf("disk image is required")
	}
	return m.fdc.InsertDiskWithGeometry(
		image.Data,
		image.Geometry.SectorsPerTrack,
		image.Geometry.Sides,
		image.Geometry.Tracks,
	)
}

func (m *Machine) RequestInterrupt(level uint8, vector *uint8) error {
	return m.cpu.RequestInterrupt(level, vector)
}

// HardDiskSizeBytes reports the active ACSI hard disk image size.
func (m *Machine) HardDiskSizeBytes() int {
	return m.fdc.HardDiskSizeBytes()
}

// HardDiskImage returns a copy of the current ACSI hard disk image.
func (m *Machine) HardDiskImage() []byte {
	return m.fdc.HardDiskImage()
}

// SetHardDiskImage replaces the active ACSI hard disk image.
func (m *Machine) SetHardDiskImage(image []byte) error {
	return m.fdc.SetHardDiskImage(image)
}

// PrinterOutput returns bytes captured from the emulated Centronics printer
// port. Data is latched from PSG Port B on the active-low strobe.
func (m *Machine) PrinterOutput() []byte {
	return m.printer.Output()
}

// ClearPrinterOutput discards captured printer bytes.
func (m *Machine) ClearPrinterOutput() {
	m.printer.ClearOutput()
}

// SetPrinterBusy drives the printer BUSY input exposed through MFP GPIP0.
func (m *Machine) SetPrinterBusy(busy bool) {
	m.printer.SetBusy(busy)
	m.mfp.SetPrinterBusy(busy)
}

// LoadIntoRAM copies payload into guest RAM at address. It is intentionally
// small and test-oriented; normal software should reach RAM through the bus.
func (m *Machine) LoadIntoRAM(address uint32, payload []byte) error {
	return m.ram.LoadAt(address, payload)
}

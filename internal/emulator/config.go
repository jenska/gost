package emulator

const (
	DefaultRAMSize = 1024 * 1024
	DefaultClockHz = 8_000_000
	DefaultFrameHz = 50
)

type Config struct {
	// ROMPath points to the ROM image loaded at startup.
	ROMPath string
	// FloppyA points to the disk image inserted into floppy drive A.
	FloppyA string
	// HardDiskSizeMB configures an in-memory virtual ACSI hard disk size in MiB.
	// Set to 0 to disable the virtual hard disk.
	HardDiskSizeMB int
	// HardDiskImagePath points to a persistent host file used as ACSI hard disk.
	// When empty, the hard disk remains in-memory only.
	HardDiskImagePath string
	// Scale multiplies the rendered display size in windowed mode.
	Scale float64
	// Fullscreen starts the emulator in fullscreen mode.
	Fullscreen bool
	// Headless disables video output and window creation.
	Headless bool
	// Trace selects the trace mode: "" disables tracing, "cpu" logs basic CPU
	// state, "cpu-verbose" adds decoded instructions and more context, "boot"
	// traces boot-related activity in the configured PC range, and
	// "boot-verbose" adds verbose CPU state to boot tracing.
	// "shifter" and "shifter-verbose" emit per-frame shifter instrumentation.
	Trace string
	// TraceStart is the first PC address included by the boot trace modes.
	TraceStart uint32
	// TraceEnd is the last PC address included by the boot trace modes.
	TraceEnd uint32
	// RAMSize sets the amount of emulated RAM in bytes.
	RAMSize uint32
	// ClockHz sets the emulated CPU clock frequency in hertz.
	// This is also the base machine clock used by time-sensitive hardware.
	ClockHz uint64
	// CPUClockHz sets the CPU execution frequency in hertz.
	// When this differs from ClockHz, only CPU throughput changes while
	// hardware timing stays tied to ClockHz.
	CPUClockHz uint64
	// FrameHz sets the target display refresh rate in hertz.
	FrameHz uint64
	// ColorMonitor reports a color monitor on the ST monitor-detect line.
	// When false, the machine behaves like a monochrome monitor is attached.
	ColorMonitor bool
	// MidResYScale scales the displayed height of 640x200 medium resolution.
	// This only affects host presentation, not guest-visible coordinates.
	MidResYScale int
}

func DefaultConfig() Config {
	return Config{
		Scale:          2.0,
		TraceStart:     bootTraceStart,
		TraceEnd:       bootTraceEnd,
		RAMSize:        DefaultRAMSize,
		ClockHz:        DefaultClockHz,
		CPUClockHz:     20 * DefaultClockHz,
		FrameHz:        DefaultFrameHz,
		HardDiskSizeMB: 30,
		ColorMonitor:   true,
		MidResYScale:   2,
	}
}

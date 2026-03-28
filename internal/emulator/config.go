package emulator

const (
	DefaultRAMSize = 1024 * 1024
	DefaultClockHz = 8_000_000
	DefaultFrameHz = 50
)

type Config struct {
	ROMPath    string
	FloppyA    string
	Scale      int
	Fullscreen bool
	Headless   bool
	Trace      string
	TraceStart uint32
	TraceEnd   uint32
	RAMSize    uint32
	ClockHz    uint64
	FrameHz    uint64
}

func DefaultConfig() Config {
	return Config{
		Scale:      1,
		TraceStart: bootTraceStart,
		TraceEnd:   bootTraceEnd,
		RAMSize:    DefaultRAMSize,
		ClockHz:    DefaultClockHz,
		FrameHz:    DefaultFrameHz,
	}
}

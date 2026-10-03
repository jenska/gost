package config

import (
	"fmt"
	"strings"
)

const (
	DefaultRAMSize        = 1024 * 1024
	DefaultClockHz        = 8_000_000
	DefaultFrameHz        = 50
	DefaultHardDiskSizeMB = 30
	DefaultHeadlessFrames = 1000
)

type MachineModel string

const (
	MachineModelST  MachineModel = "st"
	MachineModelSTE MachineModel = "ste"

	bootTraceStart = 0xE00000
	bootTraceEnd   = 0xE01000
)

const (
	KeyConfig         = "config"
	KeyPreset         = "preset"
	KeyROM            = "rom"
	KeyCartridge      = "cartridge"
	KeyFloppyA        = "floppy-a"
	KeyFloppyB        = "floppy-b"
	KeyHardDiskSizeMB = "hd-size-mb"
	KeyHardDiskImage  = "hd-image"
	KeyScale          = "scale"
	KeyFullscreen     = "fullscreen"
	KeyHeadless       = "headless"
	KeyFrames         = "frames"
	KeyDumpFrame      = "dump-frame"
	KeyTrace          = "trace"
	KeyTraceStart     = "trace-start"
	KeyTraceEnd       = "trace-end"
	KeyRAMSize        = "ram-size"
	KeyClockHz        = "clock-hz"
	KeyCPUMHz         = "cpu-mhz"
	KeyCPUClockHz     = "cpu-clock-hz"
	KeyFrameHz        = "frame-hz"
	KeyColorMonitor   = "color-monitor"
	KeyRTC            = "rtc"
	KeyMegaRTC        = "mega-rtc"
	KeyMidResYScale   = "midres-y-scale"
	KeyModel          = "model"
	KeyLauncher       = "launcher"
)

// Config holds all configuration parameters for the Atari ST emulation.
type Config struct {
	// ROMPath is the path to a custom TOS ROM file; if empty, uses bundled EmuTOS.
	ROMPath string
	// CartridgePath is the path to an optional cartridge ROM image.
	CartridgePath string
	// FloppyA is the path to a disk image to insert into floppy drive A.
	FloppyA string
	// FloppyB is the path to a disk image to insert into floppy drive B.
	FloppyB string
	// HardDiskSizeMB is the virtual hard disk size in megabytes (0 disables hard disk).
	HardDiskSizeMB uint32
	// HardDiskImagePath is the file path for persistent virtual hard disk storage.
	HardDiskImagePath string
	// Scale is the UI scaling factor for the display window.
	Scale float64
	// Fullscreen enables fullscreen display mode.
	Fullscreen bool
	// Headless enables headless execution mode (no UI window).
	Headless bool
	// Frames specifies how many frames to run in headless mode before exit.
	Frames int
	// DumpFramePath is the output path for frame dumps (PNG format).
	DumpFramePath string
	// Trace specifies the trace mode: "cpu", "cpu-verbose", "boot", "boot-verbose", "shifter", "shifter-verbose".
	Trace string
	// TraceStart is the CPU address where detailed tracing begins.
	TraceStart uint32
	// TraceEnd is the CPU address where detailed tracing ends.
	TraceEnd uint32
	// RAMSize is the emulated machine RAM size in bytes.
	RAMSize uint32
	// ClockHz is the emulated system clock frequency in Hz.
	ClockHz uint64
	// CPUClockHz is the emulated CPU clock frequency in Hz.
	CPUClockHz uint64
	// FrameHz is the emulated display refresh rate in Hz (typically 50 or 60).
	FrameHz uint64
	// ColorMonitor enables color monitor mode; false uses monochrome mode.
	ColorMonitor bool
	// ICDRTC enables the ICD-compatible ACSI real-time clock, an add-on clock
	// reached through the hard disk interface.
	ICDRTC bool
	// MegaRTC enables the RP5C15 real-time clock built into the Mega ST and
	// Mega STE.
	MegaRTC bool
	// MidResYScale applies Y-axis pixel doubling for medium resolution mode.
	MidResYScale int
	// Model specifies the machine type: st or ste.
	Model MachineModel
	// Launcher forces the desktop configuration launcher to open before boot.
	Launcher bool
}

// DefaultConfig returns the configuration used when no preset is named: an
// Atari 1040 STF on a monochrome monitor.
func DefaultConfig() *Config {
	return &Config{
		Scale:          1.0,
		Frames:         DefaultHeadlessFrames,
		TraceStart:     bootTraceStart,
		TraceEnd:       bootTraceEnd,
		RAMSize:        DefaultRAMSize,
		ClockHz:        DefaultClockHz,
		CPUClockHz:     DefaultClockHz,
		FrameHz:        DefaultFrameHz,
		HardDiskSizeMB: DefaultHardDiskSizeMB,
		MidResYScale:   2,
		Model:          MachineModelST,
	}
}

func (cfg *Config) FrameCycles() uint64 {
	if cfg == nil || cfg.ClockHz == 0 || cfg.FrameHz == 0 {
		return 0
	}
	return cfg.ClockHz / cfg.FrameHz
}

func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	model, err := normalizeModel(cfg.Model)
	if err != nil {
		return err
	}
	cfg.Model = model

	if cfg.Scale <= 0 {
		return fmt.Errorf("invalid scale %.3f: must be > 0", cfg.Scale)
	}
	if cfg.RAMSize == 0 {
		return fmt.Errorf("invalid ram-size %d: must be > 0", cfg.RAMSize)
	}
	if cfg.ClockHz == 0 {
		return fmt.Errorf("invalid clock-hz %d: must be > 0", cfg.ClockHz)
	}
	if cfg.FrameHz == 0 {
		return fmt.Errorf("invalid frame-hz %d: must be > 0", cfg.FrameHz)
	}
	if cfg.CPUClockHz == 0 {
		return fmt.Errorf("invalid %s %d: must be > 0", KeyCPUClockHz, cfg.CPUClockHz)
	}
	if cfg.MidResYScale < 1 {
		return fmt.Errorf("invalid midres-y-scale %d: must be >= 1", cfg.MidResYScale)
	}
	if cfg.Frames < 0 {
		return fmt.Errorf("invalid frames %d: must be >= 0", cfg.Frames)
	}
	if cfg.FrameCycles() == 0 {
		return fmt.Errorf("invalid frame timing: clock-hz %d / frame-hz %d yields 0 frame cycles", cfg.ClockHz, cfg.FrameHz)
	}
	return nil
}

func normalizeModel(model MachineModel) (MachineModel, error) {
	switch normalized := MachineModel(strings.ToLower(strings.TrimSpace(string(model)))); normalized {
	case "", MachineModelST:
		return MachineModelST, nil
	case MachineModelSTE:
		return MachineModelSTE, nil
	default:
		return "", fmt.Errorf("unsupported machine model %q", model)
	}
}

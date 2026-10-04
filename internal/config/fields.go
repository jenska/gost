package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// field describes one configuration key. The same flag.Value parses the key as
// a CLI flag and as a JSON config entry, and its Get result is what a saved
// profile stores.
type field struct {
	key   string
	usage string
	// persist marks the user-facing settings written to profiles and the
	// last-used config.
	persist bool
	value   func(*Config) flag.Getter
}

// fields lists every key except config and preset, which select where values
// come from rather than setting a value themselves.
var fields = []field{
	{KeyROM, "path to Atari ST TOS ROM", true,
		func(c *Config) flag.Getter { return (*stringValue)(&c.ROMPath) }},
	{KeyCartridge, "path to optional Atari ST cartridge ROM image", true,
		func(c *Config) flag.Getter { return (*stringValue)(&c.CartridgePath) }},
	{KeyFloppyA, "path to drive A disk image (.st, .msa, .dim, or compatible .adi)", true,
		func(c *Config) flag.Getter { return (*stringValue)(&c.FloppyA) }},
	{KeyFloppyB, "path to drive B disk image (.st, .msa, .dim, or compatible .adi)", true,
		func(c *Config) flag.Getter { return (*stringValue)(&c.FloppyB) }},
	{KeyHardDiskSizeMB, "virtual ACSI hard disk size in MiB (0 disables)", true,
		func(c *Config) flag.Getter { return (*uint32Value)(&c.HardDiskSizeMB) }},
	{KeyHardDiskImage, "path to persistent virtual hard disk image file", true,
		func(c *Config) flag.Getter { return (*stringValue)(&c.HardDiskImagePath) }},
	{KeyScale, "display scale factor", true,
		func(c *Config) flag.Getter { return (*float64Value)(&c.Scale) }},
	{KeyFullscreen, "run in fullscreen mode", true,
		func(c *Config) flag.Getter { return (*boolValue)(&c.Fullscreen) }},
	{KeyHeadless, "disable video output and window creation", false,
		func(c *Config) flag.Getter { return (*boolValue)(&c.Headless) }},
	{KeyFrames, "frames to run in headless mode", false,
		func(c *Config) flag.Getter { return (*intValue)(&c.Frames) }},
	{KeyDumpFrame, "write the last rendered framebuffer to a PNG file", false,
		func(c *Config) flag.Getter { return (*stringValue)(&c.DumpFramePath) }},
	{KeyTrace, "enable tracing: cpu|cpu-verbose|boot|boot-verbose|shifter|shifter-verbose", false,
		func(c *Config) flag.Getter { return (*stringValue)(&c.Trace) }},
	{KeyTraceStart, "first PC included in boot traces", false,
		func(c *Config) flag.Getter { return (*addressValue)(&c.TraceStart) }},
	{KeyTraceEnd, "last PC included in boot traces", false,
		func(c *Config) flag.Getter { return (*addressValue)(&c.TraceEnd) }},
	{KeyRAMSize, "amount of emulated RAM in bytes", true,
		func(c *Config) flag.Getter { return (*uint32Value)(&c.RAMSize) }},
	{KeyClockHz, "base machine clock frequency in Hz", false,
		func(c *Config) flag.Getter { return (*uint64Value)(&c.ClockHz) }},
	{KeyCPUMHz, "CPU frequency in MHz (hardware timing remains unchanged)", false,
		func(c *Config) flag.Getter { return (*mhzValue)(&c.CPUClockHz) }},
	{KeyCPUClockHz, "CPU frequency in Hz (hardware timing remains unchanged)", true,
		func(c *Config) flag.Getter { return (*uint64Value)(&c.CPUClockHz) }},
	{KeyFrameHz, "frames per second for display and VBL timing", false,
		func(c *Config) flag.Getter { return (*uint64Value)(&c.FrameHz) }},
	{KeyColorMonitor, "emulate an Atari color monitor instead of monochrome", true,
		func(c *Config) flag.Getter { return (*boolValue)(&c.ColorMonitor) }},
	{KeyRTC, "enable the ICD-compatible ACSI real-time clock", true,
		func(c *Config) flag.Getter { return (*boolValue)(&c.ICDRTC) }},
	{KeyMegaRTC, "enable the Mega ST/STE built-in RP5C15 real-time clock", true,
		func(c *Config) flag.Getter { return (*boolValue)(&c.MegaRTC) }},
	{KeyFastFloppy, "complete floppy commands instantly instead of at real drive speed", true,
		func(c *Config) flag.Getter { return (*boolValue)(&c.FastFloppy) }},
	{KeyMidResYScale, "vertical host scaling for medium resolution (>=1)", false,
		func(c *Config) flag.Getter { return (*intValue)(&c.MidResYScale) }},
	{KeyModel, "machine model: st|ste", true,
		func(c *Config) flag.Getter { return (*modelValue)(&c.Model) }},
	{KeyLauncher, "open the desktop configuration launcher before boot", false,
		func(c *Config) flag.Getter { return (*boolValue)(&c.Launcher) }},
}

func lookupField(key string) (field, bool) {
	for _, f := range fields {
		if f.key == key {
			return f, true
		}
	}
	return field{}, false
}

// The value types below follow the standard library's flag values, so a
// zero value prints as the empty default in -help output.

type stringValue string

func (v *stringValue) Set(s string) error { *v = stringValue(s); return nil }
func (v *stringValue) Get() any           { return string(*v) }
func (v *stringValue) String() string     { return string(*v) }

type boolValue bool

func (v *boolValue) Set(s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	*v = boolValue(b)
	return nil
}
func (v *boolValue) Get() any         { return bool(*v) }
func (v *boolValue) String() string   { return strconv.FormatBool(bool(*v)) }
func (v *boolValue) IsBoolFlag() bool { return true }

type intValue int

func (v *intValue) Set(s string) error {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 0, strconv.IntSize)
	if err != nil {
		return err
	}
	*v = intValue(n)
	return nil
}
func (v *intValue) Get() any       { return int(*v) }
func (v *intValue) String() string { return strconv.Itoa(int(*v)) }

type uint32Value uint32

func (v *uint32Value) Set(s string) error {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 0, 32)
	if err != nil {
		return err
	}
	*v = uint32Value(n)
	return nil
}
func (v *uint32Value) Get() any       { return uint32(*v) }
func (v *uint32Value) String() string { return strconv.FormatUint(uint64(*v), 10) }

type uint64Value uint64

func (v *uint64Value) Set(s string) error {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 0, 64)
	if err != nil {
		return err
	}
	*v = uint64Value(n)
	return nil
}
func (v *uint64Value) Get() any       { return uint64(*v) }
func (v *uint64Value) String() string { return strconv.FormatUint(uint64(*v), 10) }

type float64Value float64

func (v *float64Value) Set(s string) error {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return err
	}
	*v = float64Value(f)
	return nil
}
func (v *float64Value) Get() any       { return float64(*v) }
func (v *float64Value) String() string { return strconv.FormatFloat(float64(*v), 'g', -1, 64) }

// addressValue is a CPU address, accepted in any Go integer notation and
// printed as hex.
type addressValue uint32

func (v *addressValue) Set(s string) error {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 0, 32)
	if err != nil {
		return err
	}
	*v = addressValue(n)
	return nil
}
func (v *addressValue) Get() any       { return uint32(*v) }
func (v *addressValue) String() string { return fmt.Sprintf("0x%06x", uint32(*v)) }

// mhzValue sets a Hz field from a (possibly fractional) MHz value.
type mhzValue uint64

func (v *mhzValue) Set(s string) error {
	mhz, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return err
	}
	if mhz <= 0 {
		return fmt.Errorf("invalid %s %.3f: must be > 0", KeyCPUMHz, mhz)
	}
	hz := uint64(mhz * 1_000_000.0)
	if hz == 0 {
		return fmt.Errorf("invalid %s %.6f: effective CPU clock rounded to 0 Hz", KeyCPUMHz, mhz)
	}
	*v = mhzValue(hz)
	return nil
}
func (v *mhzValue) Get() any { return uint64(*v) }
func (v *mhzValue) String() string {
	if *v == 0 {
		return ""
	}
	return strconv.FormatFloat(float64(*v)/1_000_000.0, 'f', -1, 64)
}

type modelValue MachineModel

func (v *modelValue) Set(s string) error {
	model, err := normalizeModel(MachineModel(s))
	if err != nil {
		return err
	}
	*v = modelValue(model)
	return nil
}
func (v *modelValue) Get() any       { return string(*v) }
func (v *modelValue) String() string { return string(*v) }

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/emulator"
	gostebiten "github.com/jenska/gost/internal/platform/ebiten"
)

func main() {
	cfg := emulator.DefaultConfig()
	var frames int
	var dumpFramePath string
	traceStart := fmt.Sprintf("0x%06x", cfg.TraceStart)
	traceEnd := fmt.Sprintf("0x%06x", cfg.TraceEnd)

	flag.StringVar(&cfg.ROMPath, "rom", "", "path to Atari ST TOS ROM")
	flag.StringVar(&cfg.FloppyA, "floppy-a", "", "path to drive A disk image (.st or .msa)")
	flag.IntVar(&cfg.HardDiskSizeMB, "hd-size-mb", cfg.HardDiskSizeMB, "virtual ACSI hard disk size in MiB (0 disables)")
	flag.StringVar(&cfg.HardDiskImagePath, "hd-image", "", "path to persistent virtual hard disk image file")
	flag.Float64Var(&cfg.Scale, "scale", cfg.Scale, "display scale factor")
	flag.BoolVar(&cfg.Fullscreen, "fullscreen", false, "run in fullscreen mode")
	flag.BoolVar(&cfg.Headless, "headless", false, "run without a window")
	flag.BoolVar(&cfg.ColorMonitor, "color-monitor", false, "emulate an Atari color monitor instead of monochrome")
	flag.StringVar(&cfg.Trace, "trace", "", "enable tracing: cpu|cpu-verbose|boot|boot-verbose")
	flag.StringVar(&traceStart, "trace-start", traceStart, "first PC included in boot traces (hex or decimal)")
	flag.StringVar(&traceEnd, "trace-end", traceEnd, "last PC included in boot traces (hex or decimal)")
	flag.StringVar(&dumpFramePath, "dump-frame", "", "write the last rendered framebuffer to a PNG file")
	flag.IntVar(&frames, "frames", 500, "frames to run in headless mode")
	flag.Parse()

	var err error
	cfg.TraceStart, err = parseTraceAddress(traceStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse trace-start: %v\n", err)
		os.Exit(1)
	}
	cfg.TraceEnd, err = parseTraceAddress(traceEnd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse trace-end: %v\n", err)
		os.Exit(1)
	}

	var romImage []byte
	if cfg.ROMPath == "" {
		romImage = assets.DefaultROM()
		fmt.Fprintf(os.Stderr, "using bundled default OS: %s\n", assets.DefaultOSName)
	} else {
		romImage, err = emulator.LoadROM(cfg.ROMPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load ROM: %v\n", err)
			os.Exit(1)
		}
	}

	machine, err := emulator.NewMachine(cfg, romImage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create machine: %v\n", err)
		os.Exit(1)
	}
	if cfg.Trace != "" {
		machine.EnableTrace(cfg.Trace, os.Stdout)
	}
	if cfg.HardDiskImagePath != "" {
		image, created, err := emulator.EnsureHardDiskImageFile(cfg.HardDiskImagePath, machine.HardDiskImage())
		if err != nil {
			fmt.Fprintf(os.Stderr, "prepare hard disk image: %v\n", err)
			os.Exit(1)
		}
		if err := machine.SetHardDiskImage(image); err != nil {
			fmt.Fprintf(os.Stderr, "attach hard disk image: %v\n", err)
			os.Exit(1)
		}
		if created {
			fmt.Fprintf(os.Stderr, "created virtual hard disk image: %s (%d MiB)\n", cfg.HardDiskImagePath, len(image)/(1024*1024))
		} else {
			fmt.Fprintf(os.Stderr, "using virtual hard disk image: %s (%d MiB)\n", cfg.HardDiskImagePath, len(image)/(1024*1024))
		}
	}

	if cfg.FloppyA != "" {
		disk, err := emulator.LoadDiskImage(cfg.FloppyA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load disk: %v\n", err)
			os.Exit(1)
		}
		if err := machine.InsertFloppy(0, disk); err != nil {
			fmt.Fprintf(os.Stderr, "insert disk: %v\n", err)
			os.Exit(1)
		}
	}

	persistHardDisk := func() error {
		if cfg.HardDiskImagePath == "" {
			return nil
		}
		return emulator.SaveHardDiskImageFile(cfg.HardDiskImagePath, machine.HardDiskImage())
	}

	if cfg.Headless {
		for i := 0; i < frames; i++ {
			if _, err := machine.StepFrame(); err != nil {
				if saveErr := persistHardDisk(); saveErr != nil {
					fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", saveErr)
				}
				fmt.Fprintf(os.Stderr, "headless frame %d: %v\n", i, err)
				os.Exit(1)
			}
		}
		regs := machine.Registers()
		if dumpFramePath != "" {
			if err := machine.DumpFramePNG(dumpFramePath); err != nil {
				fmt.Fprintf(os.Stderr, "dump frame: %v\n", err)
				os.Exit(1)
			}
		}
		if err := persistHardDisk(); err != nil {
			fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("frames=%d cycles=%d pc=%06x sr=%04x\n", frames, machine.Cycles(), regs.PC, regs.SR)
		return
	}

	if err := gostebiten.Run(machine, cfg); err != nil {
		if saveErr := persistHardDisk(); saveErr != nil {
			fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", saveErr)
		}
		fmt.Fprintf(os.Stderr, "run emulator: %v\n", err)
		os.Exit(1)
	}

	if dumpFramePath != "" {
		if err := machine.DumpFramePNG(dumpFramePath); err != nil {
			fmt.Fprintf(os.Stderr, "dump frame: %v\n", err)
			os.Exit(1)
		}
	}
	if err := persistHardDisk(); err != nil {
		fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", err)
		os.Exit(1)
	}
}

func parseTraceAddress(raw string) (uint32, error) {
	value, err := strconv.ParseUint(raw, 0, 32)
	if err != nil {
		return 0, err
	}
	return uint32(value), nil
}

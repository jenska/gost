package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/emulator"
	gostebiten "github.com/jenska/gost/internal/platform/ebiten"
)

func main() {
	cfg := emulator.DefaultConfig()
	var frames int

	flag.StringVar(&cfg.ROMPath, "rom", "", "path to Atari ST TOS ROM")
	flag.StringVar(&cfg.ROMPath, "os", "", "path to operating system ROM image")
	flag.StringVar(&cfg.FloppyA, "floppy-a", "", "path to drive A disk image (.st)")
	flag.IntVar(&cfg.Scale, "scale", cfg.Scale, "display scale factor")
	flag.BoolVar(&cfg.Fullscreen, "fullscreen", false, "run in fullscreen mode")
	flag.BoolVar(&cfg.Headless, "headless", false, "run without a window")
	flag.StringVar(&cfg.Trace, "trace", "", "enable tracing: cpu|video|fdc")
	flag.IntVar(&frames, "frames", 300, "frames to run in headless mode")
	flag.Parse()

	var romImage []byte
	if cfg.ROMPath == "" {
		romImage = assets.DefaultROM()
		fmt.Fprintf(os.Stderr, "using bundled default OS: %s\n", assets.DefaultOSName)
	} else {
		var err error
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
	if cfg.Trace != "" && cfg.Trace != "cpu" {
		machine.EnableTrace(cfg.Trace, os.Stdout)
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

	if cfg.Headless {
		for i := 0; i < frames; i++ {
			if _, err := machine.StepFrame(); err != nil {
				fmt.Fprintf(os.Stderr, "headless frame %d: %v\n", i, err)
				os.Exit(1)
			}
		}
		regs := machine.Registers()
		fmt.Printf("frames=%d cycles=%d pc=%06x sr=%04x\n", frames, machine.Cycles(), regs.PC, regs.SR)
		return
	}

	if err := gostebiten.Run(machine, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "run emulator: %v\n", err)
		os.Exit(1)
	}
}

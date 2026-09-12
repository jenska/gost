package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/emulator"
	gostebiten "github.com/jenska/gost/internal/platform/ebiten"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	showLauncher := !cfg.Headless && (cfg.Launcher || config.ShouldShowLauncher(os.Args[1:]))
	if showLauncher {
		if last, ok := config.LoadLastConfig(); ok {
			cfg = last
		}
	}

	session, err := emulator.BuildMachine(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	machine := session.Machine

	if cfg.ROMPath == "" {
		fmt.Fprintf(os.Stderr, "using bundled default OS: %s\n", session.ROMName)
	}
	if cfg.CartridgePath != "" {
		fmt.Fprintf(os.Stderr, "using cartridge ROM: %s\n", cfg.CartridgePath)
	}
	if cfg.HardDiskImagePath != "" {
		verb := "using"
		if session.HardDiskCreated {
			verb = "created"
		}
		fmt.Fprintf(os.Stderr, "%s virtual hard disk image: %s\n", verb, cfg.HardDiskImagePath)
	}
	if cfg.Trace != "" {
		machine.EnableTrace(cfg.Trace, os.Stdout)
	}

	if cfg.Headless {
		runHeadless(cfg, session)
		return
	}

	// Run owns the machine lifecycle from here: reboots on "Apply", hard-disk
	// persistence, frame dumps, and recording the last-used config.
	if err := gostebiten.Run(session, *cfg, showLauncher); err != nil {
		fmt.Fprintf(os.Stderr, "run emulator: %v\n", err)
		os.Exit(1)
	}
}

func runHeadless(cfg *config.Config, session *emulator.Session) {
	machine := session.Machine
	for i := range cfg.Frames {
		if _, err := machine.StepFrame(); err != nil {
			if saveErr := session.PersistHardDisk(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", saveErr)
			}
			fmt.Fprintf(os.Stderr, "headless frame %d: %v\n", i, err)
			os.Exit(1)
		}
	}
	regs := machine.Registers()
	if cfg.DumpFramePath != "" {
		if err := machine.DumpFramePNG(cfg.DumpFramePath); err != nil {
			fmt.Fprintf(os.Stderr, "dump frame: %v\n", err)
			os.Exit(1)
		}
	}
	if err := session.PersistHardDisk(); err != nil {
		fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("frames=%d cycles=%d pc=%06x sr=%04x\n", cfg.Frames, machine.Cycles(), regs.PC, regs.SR)
}

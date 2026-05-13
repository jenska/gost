package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/jenska/gost/internal/assets"
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

	var romImage []byte
	if cfg.ROMPath == "" {
		romImage = assets.DefaultROM()
		fmt.Fprintf(os.Stderr, "using bundled default OS: %s\n", assets.DefaultOSName)
	} else {
		romImage, err = config.LoadROM(cfg.ROMPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load ROM: %v\n", err)
			os.Exit(1)
		}
	}

	var cartridgeImage []byte
	if cfg.CartridgePath != "" {
		cartridgeImage, err = config.LoadROM(cfg.CartridgePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load cartridge ROM: %v\n", err)
			os.Exit(1)
		}
	}

	machine, err := emulator.NewMachineWithCartridge(cfg, romImage, cartridgeImage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create machine: %v\n", err)
		os.Exit(1)
	}
	if cfg.CartridgePath != "" {
		fmt.Fprintf(os.Stderr, "using cartridge ROM: %s (%d bytes)\n", cfg.CartridgePath, len(cartridgeImage))
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

	insertFloppy := func(drive int, path string) {
		if path == "" {
			return
		}
		disk, err := emulator.LoadDiskImage(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load drive %c disk: %v\n", 'A'+drive, err)
			os.Exit(1)
		}
		if err := machine.InsertFloppy(drive, disk); err != nil {
			fmt.Fprintf(os.Stderr, "insert drive %c disk: %v\n", 'A'+drive, err)
			os.Exit(1)
		}
	}
	insertFloppy(0, cfg.FloppyA)
	insertFloppy(1, cfg.FloppyB)

	persistHardDisk := func() error {
		if cfg.HardDiskImagePath == "" {
			return nil
		}
		return emulator.SaveHardDiskImageFile(cfg.HardDiskImagePath, machine.HardDiskImage())
	}

	if cfg.Headless {
		for i := range cfg.Frames {
			if _, err := machine.StepFrame(); err != nil {
				if saveErr := persistHardDisk(); saveErr != nil {
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
		if err := persistHardDisk(); err != nil {
			fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("frames=%d cycles=%d pc=%06x sr=%04x\n", cfg.Frames, machine.Cycles(), regs.PC, regs.SR)
		return
	}

	if err := gostebiten.Run(machine, *cfg); err != nil {
		if saveErr := persistHardDisk(); saveErr != nil {
			fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", saveErr)
		}
		fmt.Fprintf(os.Stderr, "run emulator: %v\n", err)
		os.Exit(1)
	}

	if cfg.DumpFramePath != "" {
		if err := machine.DumpFramePNG(cfg.DumpFramePath); err != nil {
			fmt.Fprintf(os.Stderr, "dump frame: %v\n", err)
			os.Exit(1)
		}
	}
	if err := persistHardDisk(); err != nil {
		fmt.Fprintf(os.Stderr, "save hard disk image: %v\n", err)
		os.Exit(1)
	}
}

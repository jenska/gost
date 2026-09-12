package emulator

import (
	"fmt"

	"github.com/jenska/gost/internal/assets"
	"github.com/jenska/gost/internal/config"
)

// Session bundles a freshly built Machine with the host-side bookkeeping that
// both the CLI entrypoint and the in-app "Apply & Reboot" action need: which ROM
// was used, and how to flush the persistent hard-disk image back to disk.
type Session struct {
	Machine *Machine
	// ROMName is a human-readable label for the loaded TOS image.
	ROMName string
	// HardDiskCreated reports whether BuildMachine created a new hard-disk image
	// file (as opposed to loading an existing one).
	HardDiskCreated bool

	hardDiskPath string
}

// BuildMachine assembles a fully wired Machine from cfg. It loads the TOS ROM
// (bundled EmuTOS when cfg.ROMPath is empty), an optional cartridge, ensures and
// attaches a persistent hard-disk image when cfg.HardDiskImagePath is set, and
// inserts the configured floppy images. It is the single machine-assembly path
// shared by cmd/gost and the desktop UI reboot flow.
func BuildMachine(cfg *config.Config) (*Session, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	romImage, romName, err := loadTOSROM(cfg.ROMPath)
	if err != nil {
		return nil, err
	}

	var cartridgeImage []byte
	if cfg.CartridgePath != "" {
		cartridgeImage, err = config.LoadROM(cfg.CartridgePath)
		if err != nil {
			return nil, fmt.Errorf("load cartridge ROM: %w", err)
		}
	}

	machine, err := NewMachineWithCartridge(cfg, romImage, cartridgeImage)
	if err != nil {
		return nil, fmt.Errorf("create machine: %w", err)
	}

	session := &Session{
		Machine:      machine,
		ROMName:      romName,
		hardDiskPath: cfg.HardDiskImagePath,
	}

	if cfg.HardDiskImagePath != "" {
		image, created, err := EnsureHardDiskImageFile(cfg.HardDiskImagePath, machine.HardDiskImage())
		if err != nil {
			return nil, fmt.Errorf("prepare hard disk image: %w", err)
		}
		if err := machine.SetHardDiskImage(image); err != nil {
			return nil, fmt.Errorf("attach hard disk image: %w", err)
		}
		session.HardDiskCreated = created
	}

	for drive, path := range []string{cfg.FloppyA, cfg.FloppyB} {
		if path == "" {
			continue
		}
		disk, err := LoadDiskImage(path)
		if err != nil {
			return nil, fmt.Errorf("load drive %c disk: %w", 'A'+drive, err)
		}
		if err := machine.InsertFloppy(drive, disk); err != nil {
			return nil, fmt.Errorf("insert drive %c disk: %w", 'A'+drive, err)
		}
	}

	return session, nil
}

// PersistHardDisk flushes the current hard-disk image back to the file it was
// loaded from. It is a no-op when the session has no persistent hard-disk path.
func (s *Session) PersistHardDisk() error {
	if s == nil || s.hardDiskPath == "" {
		return nil
	}
	return SaveHardDiskImageFile(s.hardDiskPath, s.Machine.HardDiskImage())
}

// HardDiskImagePath is the file backing the persistent hard disk, or "" when the
// hard disk is memory-only.
func (s *Session) HardDiskImagePath() string {
	if s == nil {
		return ""
	}
	return s.hardDiskPath
}

func loadTOSROM(path string) (image []byte, name string, err error) {
	if path == "" {
		return assets.DefaultROM(), assets.DefaultOSName, nil
	}
	image, err = config.LoadROM(path)
	if err != nil {
		return nil, "", fmt.Errorf("load ROM: %w", err)
	}
	return image, path, nil
}

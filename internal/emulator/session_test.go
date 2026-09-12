package emulator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jenska/gost/internal/config"
)

func TestBuildMachineWithBundledROM(t *testing.T) {
	cfg := config.DefaultConfig()

	session, err := BuildMachine(cfg)
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}
	if session.Machine == nil {
		t.Fatalf("session has no machine")
	}
	if session.ROMName == "" {
		t.Fatalf("session ROM name is empty")
	}
	if got := session.Machine.HardDiskSizeBytes(); got != int(cfg.HardDiskSizeMB)*1024*1024 {
		t.Fatalf("hard disk size = %d, want %d", got, int(cfg.HardDiskSizeMB)*1024*1024)
	}
}

func TestBuildMachineCreatesAndPersistsHardDiskImage(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "hd.img")
	cfg := config.DefaultConfig()
	cfg.HardDiskSizeMB = 10
	cfg.HardDiskImagePath = imagePath

	session, err := BuildMachine(cfg)
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}
	if !session.HardDiskCreated {
		t.Fatalf("expected BuildMachine to create a new hard disk image")
	}
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("hard disk image not written: %v", err)
	}
	if err := session.PersistHardDisk(); err != nil {
		t.Fatalf("persist hard disk: %v", err)
	}
}

func TestBuildMachineInsertsFloppies(t *testing.T) {
	diskPath := filepath.Join(t.TempDir(), "disk.st")
	if err := os.WriteFile(diskPath, make([]byte, 512*1024), 0o644); err != nil {
		t.Fatalf("write disk: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.FloppyA = diskPath

	if _, err := BuildMachine(cfg); err != nil {
		t.Fatalf("build machine with floppy: %v", err)
	}
}

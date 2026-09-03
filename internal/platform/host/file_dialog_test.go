package host

import (
	"errors"
	"testing"
)

func TestValidateFloppyDiskImagePathAcceptsSupportedExtensions(t *testing.T) {
	for _, path := range []string{
		"/tmp/disk.st",
		"/tmp/disk.STX",
		"/tmp/disk.msa",
		"/tmp/disk.dim",
		"/tmp/disk.adi",
	} {
		if err := ValidateFloppyDiskImagePath(path); err != nil {
			t.Fatalf("ValidateFloppyDiskImagePath(%q): %v", path, err)
		}
	}
}

func TestValidateFloppyDiskImagePathRejectsUnsupportedExtensions(t *testing.T) {
	if err := ValidateFloppyDiskImagePath("/tmp/disk.zip"); err == nil {
		t.Fatalf("expected unsupported extension error")
	}
}

func TestValidateFloppyDiskImagePathRejectsEmptySelection(t *testing.T) {
	if err := ValidateFloppyDiskImagePath(""); !errors.Is(err, ErrFileDialogCanceled) {
		t.Fatalf("ValidateFloppyDiskImagePath(\"\") = %v, want ErrFileDialogCanceled", err)
	}
}

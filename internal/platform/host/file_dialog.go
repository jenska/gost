package host

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrFileDialogCanceled    = errors.New("file dialog canceled")
	ErrFileDialogUnsupported = errors.New("file dialogs are not supported on this platform")
)

var floppyDiskImageExtensions = map[string]struct{}{
	".adi": {},
	".dim": {},
	".msa": {},
	".st":  {},
	".stx": {},
}

func IsSupportedFloppyDiskImagePath(path string) bool {
	_, ok := floppyDiskImageExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func ValidateFloppyDiskImagePath(path string) error {
	if path == "" {
		return ErrFileDialogCanceled
	}
	if !IsSupportedFloppyDiskImagePath(path) {
		return fmt.Errorf("unsupported floppy image extension %q", filepath.Ext(path))
	}
	return nil
}

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

// FileDialogSpec describes a native file-selection dialog.
type FileDialogSpec struct {
	// Title is shown in the dialog chrome.
	Title string
	// Extensions restricts selectable files to these extensions (without a
	// leading dot). An empty slice allows any file.
	Extensions []string
	// ForSave requests a save-style dialog that permits naming a file that does
	// not exist yet.
	ForSave bool
}

// OpenFile shows a native open dialog and returns the chosen path. It returns
// ErrFileDialogCanceled when the user dismisses the dialog and
// ErrFileDialogUnsupported on platforms without a native picker.
func OpenFile(spec FileDialogSpec) (string, error) {
	spec.ForSave = false
	return selectFile(spec)
}

// SaveFile shows a native save dialog and returns the chosen path, which may not
// exist yet. Errors mirror OpenFile.
func SaveFile(spec FileDialogSpec) (string, error) {
	spec.ForSave = true
	return selectFile(spec)
}

var floppyDiskImageExtensions = map[string]struct{}{
	".adi": {},
	".dim": {},
	".msa": {},
	".st":  {},
	".stx": {},
}

func floppyDiskImageExtensionList() []string {
	return []string{"st", "msa", "stx", "dim", "adi"}
}

// SelectFloppyDiskImage shows an open dialog filtered to floppy disk images and
// validates the result.
func SelectFloppyDiskImage() (string, error) {
	path, err := OpenFile(FileDialogSpec{
		Title:      "Select a floppy disk image",
		Extensions: floppyDiskImageExtensionList(),
	})
	if err != nil {
		return "", err
	}
	if err := ValidateFloppyDiskImagePath(path); err != nil {
		if errors.Is(err, ErrFileDialogCanceled) {
			return "", err
		}
		return "", fmt.Errorf("%w; supported extensions are .st, .msa, .stx, .dim, and .adi", err)
	}
	return path, nil
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

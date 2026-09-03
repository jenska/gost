//go:build darwin && !cgo

package host

func SelectFloppyDiskImage() (string, error) {
	return "", ErrFileDialogUnsupported
}

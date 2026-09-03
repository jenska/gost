//go:build !darwin && !js

package host

func SelectFloppyDiskImage() (string, error) {
	return "", ErrFileDialogUnsupported
}

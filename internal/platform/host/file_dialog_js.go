//go:build js

package host

func SelectFloppyDiskImage() (string, error) {
	return "", ErrFileDialogUnsupported
}

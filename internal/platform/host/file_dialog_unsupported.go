//go:build !darwin && !js

package host

func selectFile(FileDialogSpec) (string, error) {
	return "", ErrFileDialogUnsupported
}

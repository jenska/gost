//go:build js

package host

func selectFile(FileDialogSpec) (string, error) {
	return "", ErrFileDialogUnsupported
}

package emulator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const hardDiskSectorSize = 512

func EnsureHardDiskImageFile(path string, initialImage []byte) ([]byte, bool, error) {
	if path == "" {
		return nil, false, fmt.Errorf("hard disk image path is required")
	}

	image, err := os.ReadFile(path)
	if err == nil {
		if err := validateHardDiskImageData(image); err != nil {
			return nil, false, fmt.Errorf("invalid hard disk image %q: %w", path, err)
		}
		return image, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	if err := validateHardDiskImageData(initialImage); err != nil {
		return nil, false, fmt.Errorf("cannot create hard disk image %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, err
	}
	if err := writeFileAtomic(path, initialImage, 0o644); err != nil {
		return nil, false, err
	}
	return append([]byte(nil), initialImage...), true, nil
}

func SaveHardDiskImageFile(path string, image []byte) error {
	if path == "" {
		return fmt.Errorf("hard disk image path is required")
	}
	if err := validateHardDiskImageData(image); err != nil {
		return fmt.Errorf("invalid hard disk image data: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, image, 0o644)
}

func validateHardDiskImageData(image []byte) error {
	if len(image) == 0 {
		return fmt.Errorf("image data is empty")
	}
	if len(image)%hardDiskSectorSize != 0 {
		return fmt.Errorf("image size %d is not a multiple of %d", len(image), hardDiskSectorSize)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perms os.FileMode) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, perms); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

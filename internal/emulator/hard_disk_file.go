package emulator

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const hardDiskSectorSize = 512
const anex86HDIHeaderSize = 32

func EnsureHardDiskImageFile(path string, initialImage []byte) ([]byte, bool, error) {
	if path == "" {
		return nil, false, fmt.Errorf("hard disk image path is required")
	}

	image, err := os.ReadFile(path)
	if err == nil {
		image, err = decodeHardDiskImageFileData(path, image)
		if err != nil {
			return nil, false, fmt.Errorf("invalid hard disk image %q: %w", path, err)
		}
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
	fileData, err := encodeHardDiskImageFileData(path, initialImage, nil)
	if err != nil {
		return nil, false, fmt.Errorf("cannot create hard disk image %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, err
	}
	if err := writeFileAtomic(path, fileData, 0o644); err != nil {
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
	var existing []byte
	if current, err := os.ReadFile(path); err == nil {
		existing = current
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fileData, err := encodeHardDiskImageFileData(path, image, existing)
	if err != nil {
		return fmt.Errorf("encode hard disk image data: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, fileData, 0o644)
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

func decodeHardDiskImageFileData(path string, data []byte) ([]byte, error) {
	if payload, ok, err := decodeAnex86HDI(data); ok || err != nil {
		return payload, err
	}
	if len(data)%hardDiskSectorSize == 0 {
		return data, nil
	}
	if isHDIPath(path) {
		return nil, fmt.Errorf("HDI image is neither raw sector data nor a valid Anex86 HDI file")
	}
	return data, nil
}

func encodeHardDiskImageFileData(path string, image []byte, existing []byte) ([]byte, error) {
	if len(existing) > 0 {
		if _, ok, err := decodeAnex86HDI(existing); err != nil {
			return nil, err
		} else if ok {
			return encodeAnex86HDIWithHeader(image, existing), nil
		}
		return append([]byte(nil), image...), nil
	}
	if isHDIPath(path) {
		return encodeAnex86HDI(image), nil
	}
	return append([]byte(nil), image...), nil
}

func decodeAnex86HDI(data []byte) ([]byte, bool, error) {
	if len(data) < anex86HDIHeaderSize {
		return nil, false, nil
	}
	if binary.LittleEndian.Uint32(data[0x00:0x04]) != 0 {
		return nil, false, nil
	}

	headerSize := int(binary.LittleEndian.Uint32(data[0x08:0x0C]))
	dataSize := int(binary.LittleEndian.Uint32(data[0x0C:0x10]))
	bytesPerSector := int(binary.LittleEndian.Uint32(data[0x10:0x14]))
	sectors := int(binary.LittleEndian.Uint32(data[0x14:0x18]))
	heads := int(binary.LittleEndian.Uint32(data[0x18:0x1C]))
	cylinders := int(binary.LittleEndian.Uint32(data[0x1C:0x20]))
	if headerSize == 0 && dataSize == 0 && bytesPerSector == 0 && sectors == 0 && heads == 0 && cylinders == 0 {
		return nil, false, nil
	}
	if headerSize < anex86HDIHeaderSize || headerSize > len(data) {
		return nil, true, fmt.Errorf("invalid HDI header size %d", headerSize)
	}
	if bytesPerSector != hardDiskSectorSize {
		return nil, true, fmt.Errorf("unsupported HDI sector size %d", bytesPerSector)
	}
	if sectors <= 0 || heads <= 0 || cylinders <= 0 {
		return nil, true, fmt.Errorf("invalid HDI geometry: sectors=%d heads=%d cylinders=%d", sectors, heads, cylinders)
	}
	if dataSize != sectors*heads*cylinders*bytesPerSector {
		return nil, true, fmt.Errorf("HDI data size %d does not match geometry", dataSize)
	}
	if headerSize+dataSize != len(data) {
		return nil, true, fmt.Errorf("HDI file size %d does not match header size %d plus data size %d", len(data), headerSize, dataSize)
	}
	return append([]byte(nil), data[headerSize:]...), true, nil
}

func encodeAnex86HDI(image []byte) []byte {
	header := make([]byte, anex86HDIHeaderSize)
	return encodeAnex86HDIWithHeader(image, header)
}

func encodeAnex86HDIWithHeader(image []byte, headerTemplate []byte) []byte {
	headerSize := anex86HDIHeaderSize
	if len(headerTemplate) >= anex86HDIHeaderSize {
		if parsed := int(binary.LittleEndian.Uint32(headerTemplate[0x08:0x0C])); parsed >= anex86HDIHeaderSize && parsed <= len(headerTemplate) {
			headerSize = parsed
		}
	}
	header := make([]byte, headerSize)
	copy(header, headerTemplate[:minInt(len(headerTemplate), headerSize)])

	sectors, heads, cylinders := chooseHDIGeometry(len(image) / hardDiskSectorSize)
	binary.LittleEndian.PutUint32(header[0x00:0x04], 0)
	binary.LittleEndian.PutUint32(header[0x04:0x08], uint32(len(image)/(1024*1024)))
	binary.LittleEndian.PutUint32(header[0x08:0x0C], uint32(headerSize))
	binary.LittleEndian.PutUint32(header[0x0C:0x10], uint32(len(image)))
	binary.LittleEndian.PutUint32(header[0x10:0x14], hardDiskSectorSize)
	binary.LittleEndian.PutUint32(header[0x14:0x18], uint32(sectors))
	binary.LittleEndian.PutUint32(header[0x18:0x1C], uint32(heads))
	binary.LittleEndian.PutUint32(header[0x1C:0x20], uint32(cylinders))

	out := make([]byte, 0, len(header)+len(image))
	out = append(out, header...)
	out = append(out, image...)
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func chooseHDIGeometry(totalSectors int) (sectors, heads, cylinders int) {
	for _, candidate := range []struct {
		sectors int
		heads   int
	}{
		{63, 16},
		{32, 16},
		{17, 8},
	} {
		perCylinder := candidate.sectors * candidate.heads
		if totalSectors%perCylinder == 0 {
			return candidate.sectors, candidate.heads, totalSectors / perCylinder
		}
	}
	return 1, 1, totalSectors
}

func isHDIPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".hdi")
}

func writeFileAtomic(path string, data []byte, perms os.FileMode) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, perms); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

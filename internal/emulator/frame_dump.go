package emulator

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
)

func (m *Machine) DumpFramePNG(path string) error {
	width, height := m.Dimensions()
	frame := m.FrameBuffer()
	img := &image.RGBA{
		Pix:    frame,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

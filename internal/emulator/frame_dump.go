package emulator

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type FrameDumpEncoder struct {
	jobs chan frameDumpJob
	wg   sync.WaitGroup
}

type frameDumpJob struct {
	path   string
	width  int
	height int
	pixels []byte
	result chan error
}

func NewFrameDumpEncoder(workers int) *FrameDumpEncoder {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers < 1 {
		workers = 1
	}
	encoder := &FrameDumpEncoder{
		jobs: make(chan frameDumpJob, workers*2),
	}
	for range workers {
		encoder.wg.Add(1)
		go encoder.run()
	}
	return encoder
}

func (e *FrameDumpEncoder) EncodePNG(path string, width, height int, pixels []byte) <-chan error {
	result := make(chan error, 1)
	if e == nil {
		result <- fmt.Errorf("frame dump encoder is nil")
		close(result)
		return result
	}
	if err := validateFrameDump(width, height, pixels); err != nil {
		result <- err
		close(result)
		return result
	}
	snapshot := append([]byte(nil), pixels...)
	e.jobs <- frameDumpJob{
		path:   path,
		width:  width,
		height: height,
		pixels: snapshot,
		result: result,
	}
	return result
}

func (e *FrameDumpEncoder) Close() {
	if e == nil {
		return
	}
	close(e.jobs)
	e.wg.Wait()
}

func (e *FrameDumpEncoder) run() {
	defer e.wg.Done()
	for job := range e.jobs {
		job.result <- writeFramePNG(job.path, job.width, job.height, job.pixels)
		close(job.result)
	}
}

func (m *Machine) QueueFramePNG(encoder *FrameDumpEncoder, path string) (<-chan error, error) {
	width, height := m.Dimensions()
	frame := m.FrameBuffer()
	return queueFramePNG(encoder, path, width, height, frame)
}

func (m *Machine) QueueDisplayPNG(encoder *FrameDumpEncoder, path string) (<-chan error, error) {
	width, height := m.DisplayDimensions()
	frame := m.DisplayFrameBuffer()
	return queueFramePNG(encoder, path, width, height, frame)
}

func (m *Machine) DumpFramePNG(path string) error {
	encoder := NewFrameDumpEncoder(1)
	result, err := m.QueueFramePNG(encoder, path)
	if err != nil {
		encoder.Close()
		return err
	}
	err = <-result
	encoder.Close()
	return err
}

func (m *Machine) DumpDisplayPNG(path string) error {
	encoder := NewFrameDumpEncoder(1)
	result, err := m.QueueDisplayPNG(encoder, path)
	if err != nil {
		encoder.Close()
		return err
	}
	err = <-result
	encoder.Close()
	return err
}

func queueFramePNG(encoder *FrameDumpEncoder, path string, width, height int, frame []byte) (<-chan error, error) {
	if encoder == nil {
		return nil, fmt.Errorf("frame dump encoder is nil")
	}
	if err := validateFrameDump(width, height, frame); err != nil {
		return nil, err
	}
	return encoder.EncodePNG(path, width, height, frame), nil
}

func validateFrameDump(width, height int, pixels []byte) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid frame dimensions %dx%d", width, height)
	}
	if len(pixels) != width*height*4 {
		return fmt.Errorf("invalid frame buffer length %d for %dx%d RGBA frame", len(pixels), width, height)
	}
	return nil
}

func writeFramePNG(path string, width, height int, pixels []byte) error {
	img := &image.RGBA{
		Pix:    pixels,
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

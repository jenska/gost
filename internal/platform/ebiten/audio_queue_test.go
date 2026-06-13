package ebiten

import (
	"slices"
	"testing"
	"time"
)

func TestHostAudioQueuePumpsAndDrainsSamples(t *testing.T) {
	source := &queueTestSource{
		sampleRate: 8_000,
		samples:    []float32{0.1, 0.2, 0.3},
	}
	queue := newHostAudioQueue(source, time.Second)

	queue.Pump()

	out := make([]float32, 5)
	if n := queue.DrainMonoF32(out); n != len(out) {
		t.Fatalf("drain count = %d, want %d", n, len(out))
	}
	want := []float32{0.1, 0.2, 0.3, 0, 0}
	if !slices.Equal(out, want) {
		t.Fatalf("drained samples = %#v, want %#v", out, want)
	}
}

func TestHostAudioQueueDropsOldestSamplesOnOverflow(t *testing.T) {
	source := &queueTestSource{sampleRate: 8_000}
	queue := newHostAudioQueue(source, time.Second)
	queue.buffer = make([]float32, 4)

	queue.push([]float32{1, 2, 3})
	queue.push([]float32{4, 5, 6})

	out := make([]float32, 4)
	queue.DrainMonoF32(out)
	want := []float32{3, 4, 5, 6}
	if !slices.Equal(out, want) {
		t.Fatalf("drained samples = %#v, want %#v", out, want)
	}
}

type queueTestSource struct {
	samples    []float32
	sampleRate int
}

func (s *queueTestSource) DrainMonoF32(dst []float32) int {
	n := copy(dst, s.samples)
	s.samples = s.samples[n:]
	return n
}

func (s *queueTestSource) OutputSampleRate() int {
	return s.sampleRate
}

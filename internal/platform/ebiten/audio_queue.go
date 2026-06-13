package ebiten

import (
	"sync"
	"time"

	"github.com/jenska/gost/internal/emulator"
)

const hostAudioPumpChunk = 2048

type hostAudioQueue struct {
	source emulator.AudioSource

	mu      sync.Mutex
	buffer  []float32
	read    int
	count   int
	scratch []float32
}

func newHostAudioQueue(source emulator.AudioSource, capacity time.Duration) *hostAudioQueue {
	sampleRate := source.OutputSampleRate()
	capacityFrames := max(int(capacity.Seconds()*float64(sampleRate)), hostAudioPumpChunk)
	return &hostAudioQueue{
		source:  source,
		buffer:  make([]float32, capacityFrames),
		scratch: make([]float32, hostAudioPumpChunk),
	}
}

func (q *hostAudioQueue) Pump() {
	for {
		n := q.source.DrainMonoF32(q.scratch)
		if n <= 0 {
			return
		}
		q.push(q.scratch[:n])
		if n < len(q.scratch) {
			return
		}
	}
}

func (q *hostAudioQueue) DrainMonoF32(dst []float32) int {
	q.mu.Lock()
	n := q.popLocked(dst)
	q.mu.Unlock()

	clear(dst[n:])
	return len(dst)
}

func (q *hostAudioQueue) OutputSampleRate() int {
	return q.source.OutputSampleRate()
}

func (q *hostAudioQueue) push(samples []float32) {
	if len(samples) == 0 || len(q.buffer) == 0 {
		return
	}
	if len(samples) > len(q.buffer) {
		samples = samples[len(samples)-len(q.buffer):]
	}

	q.mu.Lock()
	overflow := q.count + len(samples) - len(q.buffer)
	if overflow > 0 {
		q.read = (q.read + overflow) % len(q.buffer)
		q.count -= overflow
	}
	for _, sample := range samples {
		write := (q.read + q.count) % len(q.buffer)
		q.buffer[write] = sample
		q.count++
	}
	q.mu.Unlock()
}

func (q *hostAudioQueue) popLocked(dst []float32) int {
	n := min(len(dst), q.count)
	if n == 0 {
		return 0
	}

	first := min(n, len(q.buffer)-q.read)
	copy(dst, q.buffer[q.read:q.read+first])
	if first < n {
		copy(dst[first:n], q.buffer[:n-first])
	}
	q.read = (q.read + n) % len(q.buffer)
	q.count -= n
	return n
}

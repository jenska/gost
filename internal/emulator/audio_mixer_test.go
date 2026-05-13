package emulator

import "testing"

func TestMixedAudioSourceMixesAndClamps(t *testing.T) {
	primary := &fixedAudioSource{samples: []float32{0.75, -0.75}, sampleRate: 48_000}
	secondary := &fixedAudioSource{samples: []float32{0.50, -0.50, 0.25}, sampleRate: 48_000}
	mixer := newMixedAudioSource(primary, secondary)

	out := make([]float32, 4)
	n := mixer.DrainMonoF32(out)
	if n != 3 {
		t.Fatalf("mixed sample count = %d, want 3", n)
	}
	want := []float32{1.0, -1.0, 0.25}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("sample %d = %.2f, want %.2f", i, out[i], want[i])
		}
	}
	if mixer.OutputSampleRate() != 48_000 {
		t.Fatalf("sample rate = %d, want 48000", mixer.OutputSampleRate())
	}
}

type fixedAudioSource struct {
	samples    []float32
	sampleRate int
}

func (f *fixedAudioSource) DrainMonoF32(dst []float32) int {
	n := copy(dst, f.samples)
	return n
}

func (f *fixedAudioSource) OutputSampleRate() int {
	return f.sampleRate
}

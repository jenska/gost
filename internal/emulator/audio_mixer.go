package emulator

type mixedAudioSource struct {
	primary   AudioSource
	secondary AudioSource
	scratch   []float32
}

func newMixedAudioSource(primary, secondary AudioSource) *mixedAudioSource {
	return &mixedAudioSource{primary: primary, secondary: secondary}
}

func (m *mixedAudioSource) DrainMonoF32(dst []float32) int {
	clear(dst)
	nPrimary := m.primary.DrainMonoF32(dst)

	if cap(m.scratch) < len(dst) {
		m.scratch = make([]float32, len(dst))
	}
	scratch := m.scratch[:len(dst)]
	clear(scratch)
	nSecondary := m.secondary.DrainMonoF32(scratch)

	for i := 0; i < nSecondary; i++ {
		dst[i] = clampAudioSample(dst[i] + scratch[i])
	}
	if nSecondary > nPrimary {
		return nSecondary
	}
	return nPrimary
}

func (m *mixedAudioSource) OutputSampleRate() int {
	return m.primary.OutputSampleRate()
}

func clampAudioSample(value float32) float32 {
	switch {
	case value > 1:
		return 1
	case value < -1:
		return -1
	default:
		return value
	}
}

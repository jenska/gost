package devices

type Number interface {
	~int | ~int64 | ~float32 | ~float64
}

func abs[T Number](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

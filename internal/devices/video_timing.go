package devices

import "github.com/jenska/gost/internal/config"

// ST raster line counts. FrameHz at or above 55 is treated as an NTSC machine.
const (
	VideoPALScanlines  = 313
	VideoNTSCScanlines = 263
	VideoMonoScanlines = 501
	// VideoActiveLines is the number of displayed scanlines per frame in the
	// color modes.
	VideoActiveLines = 200
	// VideoMonoActiveLines is the number of displayed monochrome scanlines.
	VideoMonoActiveLines = 400
)

// VideoTiming is the ST raster timing a device derives from the machine clock
// and refresh rate.
type VideoTiming struct {
	FrameCycles uint64 // machine cycles per display frame (>= 1)
	Scanlines   uint64 // total scanlines per frame
	ActiveLines uint64 // displayed scanlines per frame (<= Scanlines)
}

// VideoTimingFor returns the raster timing for cfg, substituting the config
// defaults for a nil cfg or an unset ClockHz/FrameHz. A monochrome monitor gets
// the fixed 71 Hz, 501-line mono raster.
func VideoTimingFor(cfg *config.Config) VideoTiming {
	clockHz, frameHz := uint64(config.DefaultClockHz), uint64(config.DefaultFrameHz)
	mono := false
	if cfg != nil {
		if cfg.ClockHz != 0 {
			clockHz = cfg.ClockHz
		}
		if cfg.FrameHz != 0 {
			frameHz = cfg.RefreshHz()
		}
		mono = !cfg.ColorMonitor
	}

	t := VideoTiming{FrameCycles: clockHz / frameHz}
	if t.FrameCycles == 0 {
		t.FrameCycles = 1
	}
	switch {
	case mono:
		t.Scanlines = VideoMonoScanlines
		t.ActiveLines = VideoMonoActiveLines
		return t
	case frameHz >= 55:
		t.Scanlines = VideoNTSCScanlines
	default:
		t.Scanlines = VideoPALScanlines
	}
	t.ActiveLines = min(VideoActiveLines, t.Scanlines)
	return t
}

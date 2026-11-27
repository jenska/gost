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
	Mono        bool   // monochrome raster
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
		t.Mono = true
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

// LineStartCycle is the frame cycle at which scanline line begins.
func (t VideoTiming) LineStartCycle(line uint64) uint64 {
	if t.Scanlines == 0 {
		return 0
	}
	return line * t.FrameCycles / t.Scanlines
}

// DisplayWindow is the span of frame cycles in which the shifter fetches and
// shows displayed line line (display enable high). A colour line shows 320 of
// its 512 cycles after a 56-cycle left border; a monochrome line shows the
// first 160 of its 224. The window scales with the configured frame timing.
func (t VideoTiming) DisplayWindow(line uint64) (start, end uint64) {
	lineStart := t.LineStartCycle(line)
	length := t.LineStartCycle(line+1) - lineStart
	if t.Mono {
		return lineStart, lineStart + length*160/224
	}
	return lineStart + length*56/512, lineStart + length*376/512
}

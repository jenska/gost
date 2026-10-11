package ebiten

// Window scale factors offered on the Display page.
const (
	scaleMin = 1
	scaleMax = 4
)

// newDisplayPage is the "Display" topic: the monitor attached to the ST and
// the host window's size and fullscreen mode.
func (p *configPanel) newDisplayPage() *configPage {
	page := newPage("Display", "The monitor attached to the ST and how its picture appears on your screen.")

	page.addSetting("Monitor",
		"Colour for games and demos; monochrome (SM124) for 640×400 high-resolution software.",
		p.newOnOffChoice("Monochrome", "Colour", &p.work.ColorMonitor))

	labels := make([]string, 0, scaleMax-scaleMin+1)
	for s := scaleMin; s <= scaleMax; s++ {
		labels = append(labels, string(rune('0'+s))+"×")
	}
	page.addSetting("Window scale",
		"Enlarges the window by a whole factor. Takes effect without a reboot.",
		p.newChoice(labels, 0, func() int {
			return roundScale(p.work.Scale) - scaleMin
		}, func(i int) {
			p.work.Scale = float64(scaleMin + i)
		}))

	page.addSetting("Window mode",
		"Fullscreen fills the screen with the ST picture. Takes effect without a reboot.",
		p.newOnOffChoice("Windowed", "Fullscreen", &p.work.Fullscreen))

	return page
}

// roundScale rounds a window scale to the nearest factor the page offers.
func roundScale(scale float64) int {
	v := int(scale + 0.5)
	if v < scaleMin {
		v = scaleMin
	}
	if v > scaleMax {
		v = scaleMax
	}
	return v
}

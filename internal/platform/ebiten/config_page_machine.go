package ebiten

import (
	"github.com/jenska/gost/internal/config"
)

// newMachinePage is the "Machine" topic: the machine model and the add-on
// real-time clock. Presets are picked from the combo box in the panel header.
func (p *configPanel) newMachinePage() *configPage {
	page := newPage("Machine", "The ST family member to emulate. Pick a preset at the top to start from a classic Atari model.")

	page.addSetting("Model",
		"The STE adds DMA sound, a blitter, hardware scrolling and a 4096-colour palette.",
		p.newChoice([]string{"ST", "STE"}, 0, func() int {
			if p.work.Model == config.MachineModelSTE {
				return 1
			}
			return 0
		}, func(i int) {
			p.work.Model = []config.MachineModel{config.MachineModelST, config.MachineModelSTE}[i]
		}))

	page.addSetting("ICD real-time clock",
		"An add-on clock on the hard disk port that keeps GEM's date and time.",
		p.newOnOffChoice("Off", "On", &p.work.ICDRTC))

	return page
}

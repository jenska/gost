package ebiten

import "fmt"

// newCPUMemoryPage is the "CPU & Memory" topic: emulated RAM size and the
// 68000 clock speed.
func (p *configPanel) newCPUMemoryPage() *configPage {
	page := newPage("CPU & Memory", "How much RAM the ST has and how fast its 68000 runs.")

	ramLabels := make([]string, len(RAMSizeChoices))
	for i, choice := range RAMSizeChoices {
		ramLabels[i] = choice.Label
	}
	ramHint := page.addSetting("RAM", "", p.newChoice(ramLabels, 0, func() int {
		for i, choice := range RAMSizeChoices {
			if choice.Bytes == p.work.RAMSize {
				return i
			}
		}
		return -1
	}, func(i int) {
		p.work.RAMSize = RAMSizeChoices[i].Bytes
	}))
	p.refreshers = append(p.refreshers, func() {
		ramHint.Label = fmt.Sprintf("%s installed. 512 KB–1 MB suits most games; 4 MB helps GEM applications.", ramLabel(p.work.RAMSize))
	})

	cpuLabels := make([]string, len(CPUClockChoices))
	for i, choice := range CPUClockChoices {
		cpuLabels[i] = choice.Label
	}
	page.addSetting("CPU clock",
		"8 MHz is the original speed. Faster clocks speed up GEM but can break timing-sensitive games.",
		p.newChoice(cpuLabels, 0, func() int {
			for i, choice := range CPUClockChoices {
				if choice.Hz == p.work.CPUClockHz {
					return i
				}
			}
			return -1
		}, func(i int) {
			p.work.CPUClockHz = CPUClockChoices[i].Hz
		}))

	return page
}

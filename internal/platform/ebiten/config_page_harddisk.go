package ebiten

import "fmt"

// newHardDiskPage is the "Hard Disk" topic: the virtual ACSI disk's size and
// its backing image file.
func (p *configPanel) newHardDiskPage() *configPage {
	page := newPage("Hard Disk", "A virtual ACSI hard disk attached to the ST.")

	labels := make([]string, len(hardDiskSizeChoices))
	for i, mb := range hardDiskSizeChoices {
		if mb == 0 {
			labels[i] = "Off"
		} else {
			labels[i] = fmt.Sprintf("%d MB", mb)
		}
	}
	page.addSetting("Size",
		"Capacity of the virtual disk. Choose Off to boot without a hard disk.",
		p.newChoice(labels, 0, func() int {
			for i, mb := range hardDiskSizeChoices {
				if mb == p.work.HardDiskSizeMB {
					return i
				}
			}
			return -1
		}, func(i int) {
			p.work.HardDiskSizeMB = hardDiskSizeChoices[i]
		}))

	page.addSetting("Image file",
		"Blank keeps the disk in memory only; its contents are lost when GoST quits.",
		p.newPathInput("(none — memory only)", targetHardDiskOpen, p.work.HardDiskImagePath, 236, "Detach",
			newPanelButton("New…", func() { p.app.QueueBrowse(targetHardDiskNew) })))

	return page
}

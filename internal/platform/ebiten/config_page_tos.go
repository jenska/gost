package ebiten

// newTOSPage is the "TOS ROM" topic: the operating system image the ST boots.
func (p *configPanel) newTOSPage() *configPage {
	page := newPage("TOS ROM", "The operating system the ST boots from ROM.")

	page.addSetting("ROM image",
		"Leave blank to boot the bundled EmuTOS. Use an original TOS image for best compatibility.",
		p.newPathInput("(none — bundled EmuTOS)", targetROM, p.work.ROMPath, 300, "Clear"))

	return page
}

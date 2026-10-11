package ebiten

// newFloppyPage is the "Floppy Drives" topic: the disks in drives A and B and
// the drive speed.
func (p *configPanel) newFloppyPage() *configPage {
	page := newPage("Floppy Drives", "Insert disk images into the ST's two floppy drives.")

	const imageHint = "Supports .st, .msa, .stx and .dim images."
	page.addSetting("Drive A", imageHint,
		p.newPathInput("(no disk)", targetFloppyA, p.work.FloppyA, 300, "Eject"))
	page.addSetting("Drive B", imageHint,
		p.newPathInput("(no disk)", targetFloppyB, p.work.FloppyB, 300, "Eject"))

	page.addSetting("Drive speed",
		"Fast loads instantly; real speed emulates the motor and seek times some copy protections need.",
		p.newOnOffChoice("Real speed", "Fast", &p.work.FastFloppy))

	return page
}

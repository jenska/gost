package ebiten

import (
	"fmt"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/jenska/gost/internal/config"
)

type configMode int

const (
	configModeLauncher configMode = iota
	configModeOverlay
)

// hardDiskSizeChoices are the ACSI hard-disk sizes (in MiB) the panel cycles
// through; 0 disables the virtual hard disk.
var hardDiskSizeChoices = []uint32{0, 10, 20, 30, 60, 120}

// configPanel is the shared Atari ST configuration UI. It is shown full-window
// by the launcher and inside the F12 overlay during a session.
type configPanel struct {
	app  *App
	mode configMode

	container *widget.Container
	status    *widget.Text

	work config.Config

	cycles []*cycleField

	pathInputs   map[fileTarget]*widget.TextInput
	lastPathText map[fileTarget]string

	profileName *widget.TextInput
	profileList *widget.Text

	fullscreenBox *widget.Checkbox
	scaleSlider   *widget.Slider
	scaleValue    *widget.Text

	ramIndex int
	cpuIndex int
	hdIndex  int

	// transientStatus is set while an explicit message (profile saved, error)
	// should stay on the status line instead of the idle hint.
	transientStatus bool
}

type cycleField struct {
	button *widget.Button
	label  string
	value  func() string
}

func (c *cycleField) refresh() {
	c.button.SetText(c.label + ":  " + c.value())
}

func newConfigPanel(app *App, mode configMode, initial config.Config) *configPanel {
	p := &configPanel{
		app:          app,
		mode:         mode,
		work:         initial,
		pathInputs:   map[fileTarget]*widget.TextInput{},
		lastPathText: map[fileTarget]string{},
	}
	p.syncIndices()

	p.container = widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
			widget.WidgetOpts.MinSize(panelContentWidth, 0),
		),
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(panelBackgroundColor)),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 8, Bottom: 8, Left: 12, Right: 12}),
			widget.RowLayoutOpts.Spacing(4),
		)),
	)

	title := "GoST Configuration"
	if mode == configModeOverlay {
		title = "GoST Configuration  ·  Apply reboots the ST"
	}
	p.container.AddChild(newPanelText(title, 13, panelTitleColor))

	p.container.AddChild(newPanelText("MACHINE", 10, panelAccentColor))
	grid := p.newGrid()
	p.container.AddChild(grid)
	p.addCycle(grid, "Preset", func() string { return p.presetLabel() }, p.cyclePreset)
	p.addCycle(grid, "Machine", func() string { return strings.ToUpper(string(p.work.Model)) }, p.cycleModel)
	p.addCycle(grid, "RAM", func() string { return ramLabel(p.work.RAMSize) }, p.cycleRAM)
	p.addCycle(grid, "CPU", func() string { return cpuLabel(p.work.CPUClockHz) }, p.cycleCPU)
	p.addCycle(grid, "Monitor", func() string {
		if p.work.ColorMonitor {
			return "Colour"
		}
		return "Monochrome"
	}, p.cycleMonitor)
	p.addCycle(grid, "RTC", func() string {
		if p.work.RTC {
			return "On"
		}
		return "Off"
	}, p.cycleRTC)
	p.addCycle(grid, "Hard disk", func() string {
		if p.work.HardDiskSizeMB == 0 {
			return "Off"
		}
		return fmt.Sprintf("%d MB", p.work.HardDiskSizeMB)
	}, p.cycleHardDiskSize)

	p.container.AddChild(p.newPathRow("", "TOS ROM — blank uses bundled EmuTOS", targetROM, p.work.ROMPath, 384, "Clear"))

	p.container.AddChild(newPanelText("DRIVES", 10, panelAccentColor))
	p.container.AddChild(p.newPathRow("Floppy A", ".st / .msa / .stx / .dim image", targetFloppyA, p.work.FloppyA, 300, "Eject"))
	p.container.AddChild(p.newPathRow("Floppy B", ".st / .msa / .stx / .dim image", targetFloppyB, p.work.FloppyB, 300, "Eject"))
	p.container.AddChild(p.newHardDiskRow())

	p.container.AddChild(newPanelText("DISPLAY", 10, panelAccentColor))
	p.container.AddChild(p.newDisplayRow())

	p.container.AddChild(newPanelText("PROFILES", 10, panelAccentColor))
	p.container.AddChild(p.newProfileRow())

	p.status = newPanelText("", 11, panelMutedTextColor)
	p.container.AddChild(p.status)
	p.container.AddChild(p.newFooter())

	p.reloadProfiles()
	p.Refresh()
	return p
}

// panelContentWidth is the fixed inner width of the configuration panel; the
// hosting window (launcher or F12 overlay) is sized to comfortably contain it.
const panelContentWidth = 640

func (p *configPanel) newGrid() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(panelContentWidth, 0)),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Spacing(6, 4),
			widget.GridLayoutOpts.DefaultStretch(true, false),
		)),
	)
}

func (p *configPanel) addCycle(parent *widget.Container, label string, value func() string, onCycle func()) {
	c := &cycleField{label: label, value: value}
	c.button = newPanelButtonW(label+":  "+value(), 300, func() {
		onCycle()
		p.transientStatus = false
		p.Refresh()
	})
	p.cycles = append(p.cycles, c)
	parent.AddChild(c.button)
}

func (p *configPanel) newRowContainer() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 3, Bottom: 3, Left: 6, Right: 6}),
		)),
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(panelRowBackgroundColor)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(panelContentWidth, 0)),
	)
}

func (p *configPanel) newPathRow(label, placeholder string, target fileTarget, initial string, inputWidth int, clearLabel string) *widget.Container {
	row := p.newRowContainer()
	if label != "" {
		l := newPanelText(label, 12, panelTextColor)
		l.GetWidget().MinWidth = 58
		row.AddChild(l)
	}
	input := newPanelTextInput(placeholder, inputWidth)
	input.SetText(initial)
	p.pathInputs[target] = input
	p.lastPathText[target] = initial
	row.AddChild(input)
	row.AddChild(newPanelButton("Browse", func() { p.app.QueueBrowse(target) }))
	row.AddChild(newPanelButton(clearLabel, func() {
		input.SetText("")
		p.lastPathText[target] = ""
		p.app.ClearSelectedPath(target)
	}))
	return row
}

func (p *configPanel) newHardDiskRow() *widget.Container {
	row := p.newRowContainer()
	l := newPanelText("Hard disk", 12, panelTextColor)
	l.GetWidget().MinWidth = 58
	row.AddChild(l)
	input := newPanelTextInput("hard disk image — blank keeps it memory-only", 232)
	input.SetText(p.work.HardDiskImagePath)
	p.pathInputs[targetHardDiskOpen] = input
	p.lastPathText[targetHardDiskOpen] = p.work.HardDiskImagePath
	row.AddChild(input)
	row.AddChild(newPanelButton("Browse", func() { p.app.QueueBrowse(targetHardDiskOpen) }))
	row.AddChild(newPanelButton("New", func() { p.app.QueueBrowse(targetHardDiskNew) }))
	row.AddChild(newPanelButton("Detach", func() {
		input.SetText("")
		p.lastPathText[targetHardDiskOpen] = ""
		p.app.ClearSelectedPath(targetHardDiskOpen)
		p.app.ClearSelectedPath(targetHardDiskNew)
	}))
	return row
}

const (
	scaleSliderMin = 1
	scaleSliderMax = 4
)

func scaleToSlider(scale float64) int {
	v := int(scale + 0.5)
	if v < scaleSliderMin {
		v = scaleSliderMin
	}
	if v > scaleSliderMax {
		v = scaleSliderMax
	}
	return v
}

// fullscreenBoxWidth reserves enough row space for the fullscreen checkbox.
// widget.Checkbox.PreferredSize does not count its own check-box glyph towards
// the widget's width (only the label is measured), so a RowLayout placed a
// sibling widget right on top of it; wrapping the checkbox in a container that
// does honor WidgetOpts.MinSize keeps the next control clear of it.
const fullscreenBoxWidth = 112

func (p *configPanel) newDisplayRow() *widget.Container {
	row := p.newRowContainer()

	p.fullscreenBox = newPanelCheckbox("Fullscreen", p.work.Fullscreen, func(checked bool) {
		p.work.Fullscreen = checked
		p.transientStatus = false
	})
	fullscreenWrap := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(fullscreenBoxWidth, 0)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	fullscreenWrap.AddChild(p.fullscreenBox)
	row.AddChild(fullscreenWrap)

	scaleLabel := newPanelText("Scale", 12, panelTextColor)
	scaleLabel.GetWidget().MinWidth = 46
	row.AddChild(scaleLabel)

	p.scaleSlider = newPanelSlider(scaleSliderMin, scaleSliderMax, scaleToSlider(p.work.Scale), func(v int) {
		p.work.Scale = float64(v)
		p.transientStatus = false
		p.refreshScaleValue()
	})
	row.AddChild(p.scaleSlider)

	p.scaleValue = newPanelText("", 12, panelMutedTextColor)
	p.scaleValue.GetWidget().MinWidth = 34
	row.AddChild(p.scaleValue)
	p.refreshScaleValue()
	return row
}

func (p *configPanel) refreshScaleValue() {
	if p.scaleValue != nil {
		p.scaleValue.Label = fmt.Sprintf("%d×", scaleToSlider(p.work.Scale))
	}
}

func (p *configPanel) newProfileRow() *widget.Container {
	row := p.newRowContainer()
	p.profileList = newPanelText("", 11, panelMutedTextColor)
	p.profileName = newPanelTextInput("Profile name", 200)
	row.AddChild(p.profileName)
	row.AddChild(newPanelButton("Save", p.saveProfile))
	row.AddChild(newPanelButton("Load", p.loadProfile))
	row.AddChild(newPanelButton("Delete", p.deleteProfile))

	wrap := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(panelContentWidth, 0)),
	)
	wrap.AddChild(p.profileList)
	wrap.AddChild(row)
	return wrap
}

func (p *configPanel) newFooter() *widget.Container {
	footer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 4}),
		)),
	)
	if p.mode == configModeLauncher {
		footer.AddChild(newPanelButtonW("Start GoST", 140, p.apply))
	} else {
		footer.AddChild(newPanelButtonW("Apply & Reboot", 140, p.apply))
		footer.AddChild(newPanelButtonW("Close", 90, func() { p.app.setOverlayVisible(false) }))
	}
	return footer
}

// --- cycle handlers -------------------------------------------------------

func (p *configPanel) cyclePreset() {
	presets := config.MachinePresets
	if len(presets) == 0 {
		return
	}
	current := config.MatchPreset(&p.work)
	next := 0
	for i, preset := range presets {
		if preset.ID == current {
			next = (i + 1) % len(presets)
			break
		}
	}
	presets[next].Apply(&p.work)
	p.syncIndices()
}

func (p *configPanel) cycleModel() {
	if p.work.Model == config.MachineModelSTE {
		p.work.Model = config.MachineModelST
	} else {
		p.work.Model = config.MachineModelSTE
	}
}

func (p *configPanel) cycleRAM() {
	p.ramIndex = (p.ramIndex + 1) % len(config.RAMSizeChoices)
	p.work.RAMSize = config.RAMSizeChoices[p.ramIndex].Bytes
}

func (p *configPanel) cycleCPU() {
	p.cpuIndex = (p.cpuIndex + 1) % len(config.CPUClockChoices)
	p.work.CPUClockHz = config.CPUClockChoices[p.cpuIndex].Hz
}

func (p *configPanel) cycleMonitor() {
	p.work.ColorMonitor = !p.work.ColorMonitor
}

func (p *configPanel) cycleRTC() {
	p.work.RTC = !p.work.RTC
}

func (p *configPanel) cycleHardDiskSize() {
	p.hdIndex = (p.hdIndex + 1) % len(hardDiskSizeChoices)
	p.work.HardDiskSizeMB = hardDiskSizeChoices[p.hdIndex]
}

func (p *configPanel) syncIndices() {
	p.ramIndex = 0
	for i, choice := range config.RAMSizeChoices {
		if choice.Bytes == p.work.RAMSize {
			p.ramIndex = i
		}
	}
	p.cpuIndex = 0
	for i, choice := range config.CPUClockChoices {
		if choice.Hz == p.work.CPUClockHz {
			p.cpuIndex = i
		}
	}
	p.hdIndex = 0
	for i, mb := range hardDiskSizeChoices {
		if mb == p.work.HardDiskSizeMB {
			p.hdIndex = i
		}
	}
}

// --- profiles -----------------------------------------------------------

func (p *configPanel) reloadProfiles() {
	names, err := config.ListProfiles()
	if err != nil {
		p.setStatus("list profiles: " + err.Error())
		return
	}
	if p.profileList == nil {
		return
	}
	if len(names) == 0 {
		p.profileList.Label = "No saved profiles yet."
	} else {
		p.profileList.Label = "Saved: " + strings.Join(names, ", ")
	}
}

func (p *configPanel) saveProfile() {
	name := strings.TrimSpace(p.profileName.GetText())
	p.readPathInputs()
	if err := p.work.Validate(); err != nil {
		p.setStatus("invalid configuration: " + err.Error())
		return
	}
	if err := config.SaveProfile(name, &p.work); err != nil {
		p.setStatus("save profile: " + err.Error())
		return
	}
	p.setStatus("Saved profile " + name)
	p.reloadProfiles()
}

func (p *configPanel) loadProfile() {
	name := strings.TrimSpace(p.profileName.GetText())
	loaded, err := config.LoadProfile(name)
	if err != nil {
		p.setStatus("load profile: " + err.Error())
		return
	}
	// Preserve host-only fields that profiles do not carry.
	loaded.FrameHz = p.work.FrameHz
	loaded.ClockHz = p.work.ClockHz
	p.work = *loaded
	p.syncIndices()
	p.pushPathInputs()
	p.syncDisplayWidgets()
	p.setStatus("Loaded profile " + name)
	p.Refresh()
}

func (p *configPanel) syncDisplayWidgets() {
	if p.fullscreenBox != nil {
		state := widget.WidgetUnchecked
		if p.work.Fullscreen {
			state = widget.WidgetChecked
		}
		p.fullscreenBox.SetState(state)
	}
	if p.scaleSlider != nil {
		p.scaleSlider.Current = scaleToSlider(p.work.Scale)
	}
	p.refreshScaleValue()
}

func (p *configPanel) deleteProfile() {
	name := strings.TrimSpace(p.profileName.GetText())
	if err := config.DeleteProfile(name); err != nil {
		p.setStatus("delete profile: " + err.Error())
		return
	}
	p.setStatus("Deleted profile " + name)
	p.reloadProfiles()
}

// --- apply / refresh --------------------------------------------------------

func (p *configPanel) readPathInputs() {
	if in := p.pathInputs[targetROM]; in != nil {
		p.work.ROMPath = strings.TrimSpace(in.GetText())
	}
	if in := p.pathInputs[targetFloppyA]; in != nil {
		p.work.FloppyA = strings.TrimSpace(in.GetText())
	}
	if in := p.pathInputs[targetFloppyB]; in != nil {
		p.work.FloppyB = strings.TrimSpace(in.GetText())
	}
	if in := p.pathInputs[targetHardDiskOpen]; in != nil {
		p.work.HardDiskImagePath = strings.TrimSpace(in.GetText())
	}
}

func (p *configPanel) pushPathInputs() {
	for target, value := range map[fileTarget]string{
		targetROM:          p.work.ROMPath,
		targetFloppyA:      p.work.FloppyA,
		targetFloppyB:      p.work.FloppyB,
		targetHardDiskOpen: p.work.HardDiskImagePath,
	} {
		if in := p.pathInputs[target]; in != nil {
			in.SetText(value)
			p.lastPathText[target] = value
		}
	}
}

func (p *configPanel) apply() {
	p.readPathInputs()
	if err := p.work.Validate(); err != nil {
		p.setStatus("invalid configuration: " + err.Error())
		return
	}
	p.app.applyConfig(p.work)
}

// Refresh reconciles widget labels and browsed paths with the working config.
// It is called every frame by the hosting UI.
func (p *configPanel) Refresh() {
	// A path chosen through a native dialog lands in the App; pull it in unless
	// the user has since edited the field.
	for _, target := range []fileTarget{targetROM, targetFloppyA, targetFloppyB, targetHardDiskOpen, targetHardDiskNew} {
		selected := p.app.SelectedPath(target)
		if selected == "" {
			continue
		}
		dst := target
		if target == targetHardDiskNew {
			dst = targetHardDiskOpen
		}
		in := p.pathInputs[dst]
		if in == nil {
			continue
		}
		if selected != p.lastPathText[dst] {
			in.SetText(selected)
			p.lastPathText[dst] = selected
		}
	}

	for _, c := range p.cycles {
		c.refresh()
	}

	if p.status == nil {
		return
	}
	switch {
	case p.app.LastHostCommandError() != "":
		p.status.Label = p.app.LastHostCommandError()
	case p.transientStatus:
		// Keep the last explicit message (e.g. "Saved profile ...") in place.
	case p.mode == configModeOverlay:
		p.status.Label = "Apply & Reboot restarts the ST · F12 closes this panel"
	default:
		p.status.Label = "Choose a machine, mount images, then Start GoST"
	}
}

// setStatus shows an explicit message that persists until the next cycle change
// or Apply.
func (p *configPanel) setStatus(text string) {
	if p.status != nil {
		p.status.Label = text
	}
	p.transientStatus = true
}

// --- labels ---------------------------------------------------------------

func (p *configPanel) presetLabel() string {
	if id := config.MatchPreset(&p.work); id != "" {
		if preset, ok := config.PresetByID(id); ok {
			return preset.Label
		}
	}
	return "Custom"
}

func ramLabel(bytes uint32) string {
	for _, choice := range config.RAMSizeChoices {
		if choice.Bytes == bytes {
			return choice.Label
		}
	}
	return fmt.Sprintf("%d KB", bytes/1024)
}

func cpuLabel(hz uint64) string {
	for _, choice := range config.CPUClockChoices {
		if choice.Hz == hz {
			return choice.Label
		}
	}
	return fmt.Sprintf("%.2f MHz", float64(hz)/1_000_000)
}

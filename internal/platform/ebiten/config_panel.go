package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	ebitenlib "github.com/hajimehoshi/ebiten/v2"
	"github.com/jenska/gost/internal/config"
)

type configMode int

const (
	configModeLauncher configMode = iota
	configModeOverlay
)

// hardDiskSizeChoices are the ACSI hard-disk sizes (in MiB) the panel offers;
// 0 disables the virtual hard disk.
var hardDiskSizeChoices = []uint32{0, 10, 20, 30, 60, 120}

// RAMSizeChoice is one selectable RAM size for the launcher.
type RAMSizeChoice struct {
	Label string
	Bytes uint32
}

// RAMSizeChoices are the RAM sizes offered by the launcher.
var RAMSizeChoices = []RAMSizeChoice{
	{Label: "256 KB", Bytes: 256 * 1024},
	{Label: "512 KB", Bytes: 512 * 1024},
	{Label: "1 MB", Bytes: 1024 * 1024},
	{Label: "2 MB", Bytes: 2 * 1024 * 1024},
	{Label: "2.5 MB", Bytes: 2560 * 1024},
	{Label: "4 MB", Bytes: 4 * 1024 * 1024},
}

// CPUClockChoice is one selectable CPU speed for the launcher.
type CPUClockChoice struct {
	Label string
	Hz    uint64
}

// CPUClockChoices are the CPU speeds offered by the launcher.
var CPUClockChoices = []CPUClockChoice{
	{Label: "8 MHz (stock)", Hz: 8_000_000},
	{Label: "16 MHz", Hz: 16_000_000},
	{Label: "32 MHz", Hz: 32_000_000},
	{Label: "64 MHz", Hz: 64_000_000},
}

// Panel geometry. The panel is a topic list on the left and the selected
// topic's page on the right; both share a fixed body height so switching
// topics never resizes the panel. The hosting window (launcher or F12 overlay)
// is sized to comfortably contain it.
const (
	panelContentWidth = 680
	topicListWidth    = 150
	panelBodyHeight   = 400
	panelBodySpacing  = 10
	panelPadding      = 8
	pageContentWidth  = panelContentWidth - 2*panelPadding - topicListWidth - panelBodySpacing
	pageInnerWidth    = pageContentWidth - 2*pagePadding
	pagePadding       = 12
)

// configPage is one topic of the configuration panel: an entry in the topic
// list and the settings shown beside it. Each topic is built in its own
// config_page_*.go file.
type configPage struct {
	title       string
	description string
	content     *widget.Container
}

// configPanel is the shared Atari ST configuration UI. It is shown full-window
// by the launcher and inside the F12 overlay during a session.
type configPanel struct {
	app  *App
	mode configMode

	container *widget.Container
	status    *widget.Text

	header      *widget.Container
	presetCombo *widget.ListComboButton
	presetNote  *widget.Text

	topics    *widget.List
	pages     []*configPage
	pageTitle *widget.Text
	pageDesc  *widget.Text
	flipBook  *widget.FlipBook
	current   *configPage

	work config.Config

	// refreshers re-sync widgets that mirror p.work (selected options,
	// dynamic hints) and run on every Refresh.
	refreshers []func()

	pathInputs   map[fileTarget]*widget.TextInput
	lastPathText map[fileTarget]string

	profileName  *widget.TextInput
	profileGrid  *widget.Container
	profileEmpty *widget.Text

	// transientStatus is set while an explicit message (profile saved, error)
	// should stay on the status line instead of the idle hint.
	transientStatus bool
}

func newConfigPanel(app *App, mode configMode, initial config.Config) *configPanel {
	p := &configPanel{
		app:          app,
		mode:         mode,
		work:         initial,
		pathInputs:   map[fileTarget]*widget.TextInput{},
		lastPathText: map[fileTarget]string{},
	}

	p.container = widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
			widget.WidgetOpts.MinSize(panelContentWidth, 0),
		),
		widget.ContainerOpts.BackgroundImage(newPanelFrameImage()),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(panelPadding)),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	p.container.AddChild(p.newHeader("GoST Configuration"))

	p.pages = []*configPage{
		p.newMachinePage(),
		p.newCPUMemoryPage(),
		p.newTOSPage(),
		p.newFloppyPage(),
		p.newHardDiskPage(),
		p.newDisplayPage(),
		p.newProfilesPage(),
	}
	p.container.AddChild(p.newBody())

	p.status = newPanelText("", 11, panelMutedTextColor)
	p.container.AddChild(p.status)
	p.container.AddChild(p.newFooter())

	p.selectPage(p.pages[app.configTopic%len(p.pages)])
	p.reloadProfiles()
	p.Refresh()
	return p
}

// newBody lays out the topic list beside the page area.
func (p *configPanel) newBody() *widget.Container {
	body := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{false, true}, []bool{true}),
			widget.GridLayoutOpts.Spacing(panelBodySpacing, 0),
		)),
	)

	entries := make([]any, len(p.pages))
	for i, page := range p.pages {
		entries[i] = page
	}
	p.topics = widget.NewList(
		widget.ListOpts.ContainerOpts(widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(topicListWidth, panelBodyHeight),
		)),
		widget.ListOpts.Entries(entries),
		widget.ListOpts.EntryLabelFunc(func(e any) string { return e.(*configPage).title }),
		widget.ListOpts.EntryFontFace(panelFontFace(13)),
		widget.ListOpts.EntryColor(listEntryColor),
		widget.ListOpts.EntryTextPadding(&widget.Insets{Top: 6, Bottom: 6, Left: 12, Right: 8}),
		widget.ListOpts.EntryTextPosition(widget.TextPositionStart, widget.TextPositionCenter),
		widget.ListOpts.ScrollContainerImage(&widget.ScrollContainerImage{
			Idle: euiimage.NewNineSliceColor(panelRowBackgroundColor),
			// The mask clips the entries; it must be opaque to show them.
			Mask: euiimage.NewNineSliceColor(color.Black),
		}),
		widget.ListOpts.ScrollContainerPadding(&widget.Insets{Top: 6, Bottom: 6}),
		widget.ListOpts.HideHorizontalSlider(),
		widget.ListOpts.HideVerticalSlider(),
		widget.ListOpts.EntrySelectedHandler(func(args *widget.ListEntrySelectedEventArgs) {
			if page, ok := args.Entry.(*configPage); ok {
				p.showPage(page)
			}
		}),
	)
	body.AddChild(p.topics)

	pageArea := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(pageContentWidth, panelBodyHeight)),
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(panelRowBackgroundColor)),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(pagePadding)),
			widget.RowLayoutOpts.Spacing(4),
		)),
	)
	p.pageTitle = newPanelText("", 16, panelTitleColor)
	p.pageDesc = newPanelText("", 11, panelMutedTextColor)
	p.pageDesc.GetWidget().MinWidth = pageInnerWidth
	pageArea.AddChild(p.pageTitle)
	pageArea.AddChild(p.pageDesc)
	pageArea.AddChild(newSeparator(pageInnerWidth))
	p.flipBook = widget.NewFlipBook(
		widget.FlipBookOpts.ContainerOpts(widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
		)),
		widget.FlipBookOpts.Padding(&widget.Insets{Top: 6}),
	)
	pageArea.AddChild(p.flipBook)
	body.AddChild(pageArea)
	return body
}

// drawShadow is an ebitenui PreRenderHook that draws the panel's shadow
// beneath it.
func (p *configPanel) drawShadow(screen *ebitenlib.Image) {
	drawPanelShadow(screen, p.container.GetWidget().Rect)
}

// selectPage switches to page and highlights it in the topic list.
func (p *configPanel) selectPage(page *configPage) {
	p.showPage(page)
	p.topics.SetSelectedEntry(page)
}

func (p *configPanel) showPage(page *configPage) {
	if page == p.current {
		return
	}
	p.current = page
	p.pageTitle.Label = page.title
	p.pageDesc.Label = page.description
	p.flipBook.SetPage(page.content)
	p.flipBook.RequestRelayout()
	for i, candidate := range p.pages {
		if candidate == page {
			// Remember the topic so a rebuilt panel (each F12 opens a fresh
			// overlay) comes back to it.
			p.app.configTopic = i
		}
	}
}

func (p *configPanel) newFooter() *widget.Container {
	footer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	if p.mode == configModeLauncher {
		footer.AddChild(newPanelButtonW("Start GoST", p.apply))
	} else {
		footer.AddChild(newPanelButtonW("Apply & Reboot", p.apply))
		footer.AddChild(newPanelButtonW("Close", func() { p.app.setOverlayVisible(false) }))
	}
	return footer
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

// Refresh reconciles widgets and browsed paths with the working config.
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

	for _, refresh := range p.refreshers {
		refresh()
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
		p.status.Label = "Pick a topic on the left, adjust it, then Start GoST"
	}
}

// setStatus shows an explicit message that persists until the next option
// change or Apply.
func (p *configPanel) setStatus(text string) {
	if p.status != nil {
		p.status.Label = text
	}
	p.transientStatus = true
}

// changed is called after the user picks an option; it drops any explicit
// status message in favour of the idle hint.
func (p *configPanel) changed() {
	p.transientStatus = false
}

// --- labels ---------------------------------------------------------------

func ramLabel(bytes uint32) string {
	for _, choice := range RAMSizeChoices {
		if choice.Bytes == bytes {
			return choice.Label
		}
	}
	return fmt.Sprintf("%d KB", bytes/1024)
}

func cpuLabel(hz uint64) string {
	for _, choice := range CPUClockChoices {
		if choice.Hz == hz {
			return choice.Label
		}
	}
	return fmt.Sprintf("%.2f MHz", float64(hz)/1_000_000)
}

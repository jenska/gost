package ebiten

import (
	"strings"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/jenska/gost/internal/config"
)

// newProfilesPage is the "Profiles" topic: saving the whole configuration
// under a name and restoring it later.
func (p *configPanel) newProfilesPage() *configPage {
	page := newPage("Profiles", "Save the complete setup under a name and switch between setups later.")

	p.profileGrid = widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(pageInnerWidth, 0)),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(3),
			widget.GridLayoutOpts.Spacing(4, 4),
			widget.GridLayoutOpts.DefaultStretch(true, false),
		)),
	)
	p.profileEmpty = newPanelText("No saved profiles yet.", 12, panelTextColor)
	saved := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewRowLayout(
		widget.RowLayoutOpts.Direction(widget.DirectionVertical),
	)))
	saved.AddChild(p.profileEmpty)
	saved.AddChild(p.profileGrid)
	page.addSetting("Saved profiles", "Click a profile to pick its name, then Load or Delete it.", saved)

	row := newHorizontalRow()
	p.profileName = newPanelTextInput("(profile name)", 220)
	row.AddChild(p.profileName)
	row.AddChild(newPanelButton("Save", p.saveProfile))
	row.AddChild(newPanelButton("Load", p.loadProfile))
	row.AddChild(newPanelButton("Delete", p.deleteProfile))
	page.addSetting("Profile", "Save stores the settings from every page, including mounted disk images.", row)

	return page
}

func (p *configPanel) reloadProfiles() {
	names, err := config.ListProfiles()
	if err != nil {
		p.setStatus("list profiles: " + err.Error())
		return
	}
	if p.profileGrid == nil {
		return
	}
	p.profileGrid.RemoveChildren()
	for _, name := range names {
		p.profileGrid.AddChild(newPanelButtonW(name, func() {
			p.profileName.SetText(name)
		}))
	}
	p.profileGrid.RequestRelayout()
	if len(names) == 0 {
		p.profileEmpty.Label = "No saved profiles yet."
	} else {
		p.profileEmpty.Label = ""
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
	p.pushPathInputs()
	p.setStatus("Loaded profile " + name)
	p.Refresh()
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

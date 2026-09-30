package emulator

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenska/gost/internal/config"
)

// Real TOS images cannot ship with the repository. These tests boot every
// supported TOS image found in $GOST_TOS_DIR (default: the git-ignored TOS/
// folder at the repository root) and are skipped when none is present.

type tosTarget struct {
	name  string
	model config.MachineModel
	// knownFailure, when set, skips the image with this reason.
	knownFailure string
}

// tosTargets maps the version word in the ROM header to the machine it runs on.
var tosTargets = map[uint16]tosTarget{
	0x0100: {name: "TOS 1.00", model: config.MachineModelST},
	0x0102: {name: "TOS 1.02", model: config.MachineModelST},
	0x0104: {name: "TOS 1.04", model: config.MachineModelST},
	0x0106: {name: "TOS 1.06", model: config.MachineModelSTE},
	0x0162: {name: "TOS 1.62", model: config.MachineModelSTE},
	0x0205: {name: "TOS 2.05", model: config.MachineModelSTE, knownFailure: "TOS 2.05 does not reach the desktop yet"},
	0x0206: {name: "TOS 2.06", model: config.MachineModelSTE},
}

type tosImage struct {
	path   string
	target tosTarget
	rom    []byte
}

func findTOSImages(t *testing.T) []tosImage {
	t.Helper()
	dir := os.Getenv("GOST_TOS_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "TOS")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no TOS images: %v", err)
	}
	var images []tosImage
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		rom, err := os.ReadFile(path)
		if err != nil || len(rom) < 4 || rom[0] != 0x60 {
			continue
		}
		target, ok := tosTargets[binary.BigEndian.Uint16(rom[2:4])]
		if !ok {
			continue
		}
		images = append(images, tosImage{path: path, target: target, rom: rom})
	}
	if len(images) == 0 {
		t.Skipf("no supported TOS images in %s", dir)
	}
	return images
}

// screenRegion is a box on the display, given as fractions of its size.
type screenRegion struct{ x0, y0, x1, y1 float64 }

var (
	menuBar    = screenRegion{0, 0, 1, 0.04}
	desktopBox = screenRegion{0.25, 0.3, 0.75, 0.9}
	centerBox  = screenRegion{0.35, 0.35, 0.65, 0.65}
)

// darkShare is the fraction of non-white pixels in r. A drawn desktop is
// ~0.5 (mono dither) to ~1.0 (color); a blank or memory-test screen is ~0.
func darkShare(m *Machine, r screenRegion) float64 {
	w, h := m.Dimensions()
	fb := m.FrameBuffer()
	x0, x1 := int(r.x0*float64(w)), int(r.x1*float64(w))
	y0, y1 := int(r.y0*float64(h)), int(r.y1*float64(h))
	dark, total := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := (y*w + x) * 4
			if fb[i] < 0xC0 || fb[i+1] < 0xC0 || fb[i+2] < 0xC0 {
				dark++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(dark) / float64(total)
}

// desktopDrawn wants a filled desktop under a menu bar of dark text on white;
// the upper bound on the menu bar rejects the black screen before first render.
func desktopDrawn(m *Machine) bool {
	menu := darkShare(m, menuBar)
	return darkShare(m, desktopBox) > 0.4 && menu > 0.03 && menu < 0.5
}

// TestRealTOSBootsToDesktop boots each image in color and mono, waits for the
// GEM desktop, then opens the About dialog. Together these exercise the MMU
// RAM detection, STE MICROWIRE, ACSI DMA, blitter ROM/odd-address blits and
// the CPU instructions TOS's AES and VDI depend on. Subtests run serially:
// m68kemu keeps effective-address state in package-level tables, so CPUs on
// separate goroutines corrupt each other.
func TestRealTOSBootsToDesktop(t *testing.T) {
	if testing.Short() {
		t.Skip("boots real TOS images; skipped in -short mode")
	}
	for _, img := range findTOSImages(t) {
		for _, color := range []bool{true, false} {
			mode := "mono"
			if color {
				mode = "color"
			}
			t.Run(fmt.Sprintf("%s/%s/%s", img.target.name, filepath.Base(img.path), mode), func(t *testing.T) {
				if img.target.knownFailure != "" {
					t.Skip(img.target.knownFailure)
				}
				withBlitter := bootToDesktopAndOpenAbout(t, img, color, machineOptions{})
				software := bootToDesktopAndOpenAbout(t, img, color, machineOptions{noBlitter: true})
				if diff := frameDiff(withBlitter, software); diff != 0 {
					saveFrame(t, withBlitter, img, color, "blitter")
					saveFrame(t, software, img, color, "software")
					t.Fatalf("blitter and software rendering differ in %d pixels", diff)
				}
			})
		}
	}
}

// bootToDesktopAndOpenAbout boots img to the desktop and opens the About box.
func bootToDesktopAndOpenAbout(t *testing.T, img tosImage, color bool, opts machineOptions) *Machine {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Model = img.target.model
	cfg.ColorMonitor = color
	cfg.RAMSize = 1024 * 1024
	m, err := newMachine(cfg, img.rom, nil, opts)
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	step := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, err := m.StepFrame(); err != nil {
				t.Fatalf("step: %v", err)
			}
		}
	}
	variant := "blitter"
	if opts.noBlitter {
		variant = "software"
	}
	fail := func(format string, args ...any) {
		t.Helper()
		saveFrame(t, m, img, color, variant)
		t.Fatalf("%s rendering: "+format, append([]any{variant}, args...)...)
	}

	const maxFrames = 4000
	frame := 0
	for ; frame < maxFrames && !desktopDrawn(m); frame += 50 {
		if frame == 500 {
			// TOS 2.x holds its cold-boot memory test until a key is pressed.
			m.PushKey(0x39, true)
			step(3)
			m.PushKey(0x39, false)
		}
		step(50)
	}
	if !desktopDrawn(m) {
		fail("no desktop after %d frames: menu bar %.3f, desktop %.3f dark", maxFrames,
			darkShare(m, menuBar), darkShare(m, desktopBox))
	}
	t.Logf("desktop after %d frames", frame)

	// Open the first entry of the leftmost menu (the About/Desktop Info box).
	// Mouse packets move twice as far on the 640x400 mono screen.
	k := 1
	if !color {
		k = 2
	}
	for i := 0; i < 40; i++ {
		m.PushMouse(-20, -20, 0)
		step(1)
	}
	m.PushMouse(20*k, 2*k, 0)
	step(20)
	m.PushMouse(0, 12*k, 0)
	step(10)
	m.PushMouse(0, 0, 2)
	step(5)
	m.PushMouse(0, 0, 0)
	step(150)
	if c := darkShare(m, centerBox); c > 0.3 || c < 0.03 {
		fail("About dialog not shown: screen centre %.3f dark, want a white box with text", c)
	}
	return m
}

// frameDiff counts the pixels that differ between two machines' displays.
func frameDiff(a, b *Machine) int {
	fa, fb := a.FrameBuffer(), b.FrameBuffer()
	if len(fa) != len(fb) {
		return len(fa) / 4
	}
	diff := 0
	for i := 0; i < len(fa); i += 4 {
		if fa[i] != fb[i] || fa[i+1] != fb[i+1] || fa[i+2] != fb[i+2] {
			diff++
		}
	}
	return diff
}

// saveFrame keeps a failing screen outside the test's temp dir for inspection.
func saveFrame(t *testing.T, m *Machine, img tosImage, color bool, variant string) {
	t.Helper()
	mode := "mono"
	if color {
		mode = "color"
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("gost-%s-%s-%s.png", filepath.Base(img.path), mode, variant))
	if err := m.DumpFramePNG(path); err == nil {
		t.Logf("screen saved to %s", path)
	}
}

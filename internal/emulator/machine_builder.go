package emulator

import (
	"fmt"
	"io"
	"os"

	"github.com/jenska/gost/internal/config"
	"github.com/jenska/gost/internal/devices"
	cpu "github.com/jenska/m68kemu"
)

func NewMachine(cfg *config.Config, romImage []byte) (*Machine, error) {
	return NewMachineWithCartridge(cfg, romImage, nil)
}

func NewMachineWithCartridge(cfg *config.Config, romImage []byte, cartridgeImage []byte) (*Machine, error) {
	if len(romImage) == 0 {
		return nil, fmt.Errorf("ROM image is required")
	}
	frameCycles := cfg.FrameCycles()
	if frameCycles == 0 {
		return nil, fmt.Errorf("invalid frame timing: clock-hz %d / frame-hz %d yields 0 frame cycles", cfg.ClockHz, cfg.FrameHz)
	}

	// Core memory devices are created first because several later devices
	// perform DMA-style reads/writes directly against ST RAM.
	ram := devices.NewRAM(0x000000, cfg.RAMSize)
	rom := devices.NewROM(romImage, defaultROMHighAlias, secondaryROMAlias)
	cartridge, err := newCartridgeROM(cartridgeImage)
	if err != nil {
		return nil, err
	}
	overlayROM := devices.NewOverlayROM(rom, ram)
	memoryConfig := devices.NewMemoryConfig(overlayROM, cfg.RAMSize)
	ram.SetMemoryConfig(memoryConfig)

	// Model-specific video hardware is the first real fork in machine setup.
	shifter, err := newModelShifter(cfg, ram)
	if err != nil {
		return nil, err
	}

	glue := devices.NewGLUE(cfg)
	blitter := devices.NewBlitter(ram)
	mfp := devices.NewMFP(cfg)
	acia := devices.NewACIA(mfp.SetACIAInterrupt)
	fdc := devices.NewFDC(ram, mfp.SetFDCInterrupt)
	psg := devices.NewPSG(cfg.ClockHz)
	printer := devices.NewPrinterPort()
	rs232 := devices.NewRS232()
	midi := devices.NewMIDI()
	steSound := devices.NewSTESound()

	mfp.AttachRS232(rs232)
	acia.AttachMIDI(midi)
	if err := configureStorageAndClock(cfg, fdc, mfp); err != nil {
		return nil, err
	}
	wirePSGPorts(psg, fdc, mfp, printer)

	busDevices := []cpu.Device{
		overlayROM,
		ram,
		memoryConfig,
		glue,
		shifter,
		blitter,
		mfp,
		acia,
		fdc,
		psg,
		steSound,
		newMonsterProbeRegion(),
	}
	if cartridge != nil {
		busDevices = append(busDevices, cartridge)
	} else {
		busDevices = append(busDevices, newAbsentCartridgeProbeRegion())
	}
	busDevices = append(busDevices,
		newOpenBusRegion(romImage),
		rom,
	)
	bus := cpu.NewBus(busDevices...)
	bus.SetWaitStates(4)

	processor, err := cpu.NewCPU(bus)
	if err != nil {
		return nil, err
	}

	machine := &Machine{
		cfg:          cfg,
		bus:          bus,
		cpu:          processor,
		ram:          ram,
		rom:          rom,
		cartridge:    cartridge,
		overlayROM:   overlayROM,
		memoryConfig: memoryConfig,
		shifter:      shifter,
		glue:         glue,
		mfp:          mfp,
		acia:         acia,
		fdc:          fdc,
		psg:          psg,
		printer:      printer,
		rs232:        rs232,
		midi:         midi,
		clocked:      []devices.Clocked{glue, mfp, acia, fdc, psg},
		irqSources:   []devices.InterruptSource{glue, mfp, acia, fdc},
		frameCycles:  frameCycles,
		traceWriter:  io.Discard,
	}

	if cfg.Trace == string(TraceModeSimple) {
		machine.EnableTrace(string(TraceModeSimple), os.Stdout)
	}

	return machine, nil
}

func newCartridgeROM(image []byte) (*devices.CartridgeROM, error) {
	if len(image) == 0 {
		return nil, nil
	}
	cartridge, err := devices.NewCartridgeROM(image)
	if err != nil {
		return nil, fmt.Errorf("create cartridge ROM: %w", err)
	}
	return cartridge, nil
}

func newModelShifter(cfg *config.Config, ram *devices.RAM) (*devices.Shifter, error) {
	switch cfg.Model {
	case config.MachineModelST:
		return devices.NewSTShifter(cfg, ram), nil
	case config.MachineModelSTE:
		return devices.NewSTEShifter(cfg, ram), nil
	default:
		return nil, fmt.Errorf("unsupported machine model %q", cfg.Model)
	}
}

func configureStorageAndClock(cfg *config.Config, fdc *devices.FDC, mfp *devices.MFP) error {
	if cfg.RTC {
		// The ICD RTC is reached through ACSI command bytes and reports its
		// detect/session line through MFP GPIP5.
		rtc := devices.NewICDRTC()
		mfp.AttachICDRTC(rtc)
		fdc.AttachICDRTC(rtc)
	}
	if cfg.HardDiskSizeMB == 0 {
		return nil
	}
	sizeBytes := cfg.HardDiskSizeMB * 1024 * 1024
	if err := fdc.CreateVirtualHardDisk(sizeBytes); err != nil {
		return fmt.Errorf("create virtual hard disk: %w", err)
	}
	return nil
}

func wirePSGPorts(psg *devices.PSG, fdc *devices.FDC, mfp *devices.MFP, printer *devices.PrinterPort) {
	mfp.SetPrinterBusy(printer.Busy())
	psg.SetPortAObserver(func(value byte) {
		// PSG Port A is a shared ST latch: drive select/side for floppy logic
		// plus Centronics strobe on bit 5.
		fdc.SetDriveControl(value)
		printer.SetStrobe(value&0x20 != 0)
	})
	// PSG Port B is the Centronics printer data byte.
	psg.SetPortBObserver(printer.SetData)
}

func newMonsterProbeRegion() *devices.BusErrorRegion {
	return devices.NewBusErrorRegion(
		devices.AddressRange{Start: 0xFFFE00, End: 0xFFFE10},
	)
}

func newAbsentCartridgeProbeRegion() *devices.FixedValueRegion {
	return devices.NewFixedValueRegion(
		0xFFFFFFFF,
		devices.AddressRange{Start: 0xFA0000, End: 0xFA0010},
	)
}

func newOpenBusRegion(romImage []byte) *devices.OpenBus {
	return devices.NewOpenBus(
		devices.AddressRange{Start: secondaryROMAlias + uint32(len(romImage)), End: defaultROMHighAlias},
		devices.AddressRange{Start: 0xFF8000, End: 0x1000000},
	)
}

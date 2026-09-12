# Changelog

## Unreleased

### Added

- Desktop configuration launcher: a bare `go run ./cmd/gost` (or `--launcher`)
  now opens a full configuration screen before boot. It offers Atari ST model
  presets (520 ST, 1040 STF, 1040 STE, Mega ST 2/4, Mega STE), colour/monochrome
  monitor, RAM size, CPU speed, TOS ROM, floppy A/B, and a hard-disk image with
  native file pickers (including a save dialog to create a new image), plus a
  fullscreen checkbox and a window-scale slider. The same panel is reachable
  in-session via `F12`; applying machine changes there cold-reboots the emulated
  ST while floppy mount/eject and the display settings apply live. `F12` renders
  as a HUD over the running screen rather than resizing it: the ST picture never
  rescales or repositions when the panel opens, and the panel scrolls (mouse
  wheel) if it is taller or wider than the current ST resolution.
- Named configuration profiles saved locally as JSON under
  `<user config dir>/gost/profiles`, with Save / Load / Delete in the panel. The
  last configuration used is remembered and pre-selected the next time the
  launcher opens (`GOST_CONFIG_DIR` overrides the storage location).
- `config.MachinePresets` catalogue and `emulator.BuildMachine` / `emulator.Session`,
  the single machine-assembly path now shared by `cmd/gost` and the UI reboot.
- Generic `host.OpenFile` / `host.SaveFile` native file dialogs (macOS), replacing
  the floppy-only selector. They are driven through AppleScript (`osascript`)
  rather than a CGo `NSOpenPanel`, so the picker can be opened safely from the
  Ebiten game loop without a main-thread AppKit call.

### Changed

- Fetches and TOS-vector reads from the boot ROM now bypass the bus via a
  read-only flat memory window (`m68kemu` `SetFastMemory`), cutting an EmuTOS
  boot benchmark by ~8%. Wait-state accounting is unchanged, so cycle counts,
  register state, and the debug-trace suite are byte-for-byte identical to the
  bus path; `machine.disableFastMemory` forces every access back through the bus.
- `EnableTrace` installs CPU observation callbacks through the single
  `m68kemu` `SetHooks` call instead of separate setters.
- `Machine.RequestInterrupt` and `devices.Interrupt.Vector` now take a plain
  `uint8` vector; pass `cpu.AutoVector` / `devices.AutoVector` (zero) to
  auto-vector, replacing the previous `*uint8`.
- The single-range peripherals (ACIA, GLUE, MFP, PSG, FDC, Blitter, STE sound,
  cartridge ROM) expose `AddressRange()` and drop their `Contains` boilerplate;
  the bus page-maps them instead of scanning.
- Machine construction now builds the CPU with `m68kemu` `WithDeferredReset` and
  runs `Machine.Reset` once before returning, making it the single reset path.
- `EnableTrace` disassembly comes from `m68kemu`'s `TraceInfo.Mnemonic` instead
  of a direct `m68kdasm` decode; output is unchanged.
- Device interrupts now reach the CPU as a single level-sensitive `m68kemu`
  `IRQSource` (`machineIRQ` over GLUE/MFP/FDC) that the core samples each
  instruction, replacing the machine's hand-rolled `dispatchInterrupts` drain
  loop and `maskedAutovectorPulse` SR inspection. GLUE now drives a held HBL/VBL
  autovector line instead of queuing pulses, so a blank interrupt the CPU is
  masking is taken shortly after the mask clears rather than dropped outright.
- Internal cleanup, no behaviour change: PAL/NTSC raster constants moved to
  `config.Config.Video()`; GLUE dropped its unused system-control register (now
  a shared `devices.ScratchRegion` at `$FF8006`) and is no longer bus-mapped;
  `DrainInterrupts` removed in favour of `PendingIRQ`/`AckIRQ`; the JSON config
  loader is table-driven; the unused `Machine.cartridge` field is gone.

### Fixed

- The boot-ROM fast-memory window is now installed on the normal construction
  path. It was only wired into `Machine.Reset`, which the desktop and headless
  runners never call, so the shipped binary never actually used it.

### Dependencies

- `github.com/jenska/m68kemu` updated to v1.5.0 for the `WithDeferredReset`,
  `SetHooks`/`Hooks`, and `SetFastMemory` APIs, a polished public surface
  (plain-value `RequestInterrupt`, `*Bus` on `NewCPU`, fewer exported
  internals), and optional `Device.Contains`.
- `github.com/jenska/m68kdasm` is no longer a direct dependency; `m68kemu`
  still pulls it in for disassembly.

## v0.4.0 - 2026-09-04

### Added

- Added the first GUI support for runtime floppy mounting: press `F12` during desktop execution to open an overlay with drive A/B path fields plus `Browse`, `Mount`, and `Eject` controls. `Browse` opens the native file selector on macOS and falls back to manual path entry on every other build; mounting accepts the same disk image formats as `--floppy-a` / `--floppy-b`.
- Added `FDC` / `Machine` disk-eject APIs (`EjectDiskFromDrive`, `EjectFloppy`) and an Ebiten host-command queue that applies mount/eject/browse requests on the game-loop thread, reporting failures on a status line instead of aborting.
- Documented 1st Word Plus 2.02 as working from local Atarimania `.stx` floppy images on the 1040STE monochrome profile.
- Added an optional local smoke test for booting with the 1st Word Plus 2.02 `.stx` disk pair mounted.

### Changed

- Raised the minimum Go version to 1.27.
- Updated the CPU dependency to `github.com/jenska/m68kemu v1.4.0` (tagged release) and refreshed `m68kdasm`, `ym2149`, Ebitengine, and `golang.org/x/*` to their current versions.
- Simplified the new floppy-mount code: removed dead code and redundant guards, collapsed the panel colour indirection into `color.NRGBA`, and merged the duplicated path-refresh paths.

### Notes

- This remains a development release. Real TOS compatibility, exact MMU behavior, copy-protected disk formats, and full IKBD/MIDI coverage are still incomplete.

## v0.3.0 - 2026-06-13

### Added

- Added basic STE DMA sound support for STE model mode.
- Added basic printer port support.
- Added optional ICD-compatible ACSI real-time clock support.
- Added persistent ACSI hard-disk image support with raw-sector and Anex86-compatible `.hdi` handling.
- Added asynchronous PNG frame dumping from immutable framebuffer snapshots.
- Added browser demo layout and input handling updates after moving browser assets to `wasm/`.

### Changed

- Parallelized Shifter scanline rendering while keeping emulation state ownership single-threaded.
- Optimized Blitter hot paths and audio mixing/host audio queue behavior.
- Improved MFP UART timing, GLUE/VBL/HBL interrupt timing, and Shifter contention behavior.
- Updated the CPU dependency to `github.com/jenska/m68kemu v1.3.0`.
- Modernized the codebase for Go 1.26 language features, including range-over-int and built-in `min`/`max`.
- Condensed and updated README documentation for CLI usage, device coverage, and concurrency boundaries.

### Fixed

- Fixed real-time clock behavior and TOS boot/runtime issues exposed during desktop bring-up.
- Improved mouse handling in the Ebitengine and WebAssembly frontends.
- Cleaned up loop and bounds-check patterns across emulator and device code.

### Notes

- This remains a development release focused on the usable GEM desktop baseline, host-side performance, and broader device coverage.
- Real TOS compatibility, exact MMU behavior, copy-protected disk formats, and full IKBD/MIDI coverage are still incomplete.

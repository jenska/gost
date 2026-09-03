# Changelog

## Unreleased

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

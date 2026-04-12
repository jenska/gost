# GoST - Atari ST Emulator in Go

<p align="center">
  <img src="assets/media/gost.png" alt="GoST logo" width="240">
</p>

GoST is an Atari ST emulator in Go built around [`github.com/jenska/m68kemu`](https://github.com/jenska/m68kemu) for Motorola 68000 CPU emulation.

Browser build target: [GoST WebAssembly demo](https://jenska.github.io/gost/)

## Status

Major milestone:

GoST has moved beyond early bring-up and now provides a usable Atari ST desktop baseline:

- The bundled EmuTOS image boots to the GEM desktop in both monochrome and color-monitor modes.
- The desktop frontend runs in an Ebitengine window with working keyboard, mouse, and audio paths.
- Headless execution, PNG frame dumping, CPU/boot tracing, and browser builds are available for development and debugging.
- The machine model now includes RAM, ROM, Shifter, Blitter, MFP, IKBD/ACIA, floppy DMA/FDC, and YM2149-backed PSG audio.

This is still not a complete Atari ST emulator for broad real-software compatibility yet. The current focus is cleanup, stabilization, and expanding compatibility from the working desktop baseline.

Latest 400-frame color desktop boot:

![Current emulation status](assets/media/gost-status.png)

Current focus:

- Stabilize longer interactive GEM desktop sessions.
- Improve compatibility with more real Atari ST applications and disk images.
- Continue filling hardware behavior gaps where real software exposes them.

## Features

- Motorola 68000 emulation via [`github.com/jenska/m68kemu`](https://github.com/jenska/m68kemu)
- Atari ST machine model with a 24-bit bus, ROM overlay boot, and 1 MiB RAM default profile
- GEM desktop boot with the bundled EmuTOS ROM
- Monochrome and color-monitor boot modes
- Low, medium, and high resolution Shifter framebuffer rendering
- Working desktop input path for keyboard and mouse through IKBD/ACIA
- YM2149-backed PSG sound with live audio playback in the desktop frontend
- Atari ST Blitter register model exercised by live GEM/VDI boot
- MFP timer and interrupt delivery
- Floppy DMA/FDC path with `.st` and `.msa` image support
- Virtual ACSI hard disk (30 MiB default) for hard-disk aware guest software
- Desktop frontend via Ebitengine
- Headless execution with PNG framebuffer dumping
- CPU, boot, and verbose tracing for bring-up and debugging
- WebAssembly build target for browser-based experiments
- Automated Go test coverage for devices, emulator behavior, and frontend integration

## Project Layout

```text
cmd/gost                CLI entrypoint
internal/config         Presets, JSON config loading, and CLI flag parsing
internal/emulator       Machine orchestration and ST bus wiring
internal/devices        Atari ST hardware device models
internal/platform       Host frontend integrations
```

## Requirements

- Go 1.26+

The repository includes a bundled default ROM:

- EmuTOS 1.4 US 256K image
- Source: [official EmuTOS 1.4 release](https://sourceforge.net/projects/emutos/files/emutos/1.4/)
- Upstream license and release readme are mirrored in `internal/assets/EMUTOS-LICENSE.txt` and `internal/assets/EMUTOS-README.txt`

## Running

Desktop mode:

```bash
make run
```

```bash
go run ./cmd/gost
```

If `downloads/atari-st/PDATS321.msa` exists locally, `make run` and `make headless` automatically mount it as drive A.

Color monitor mode:

```bash
go run ./cmd/gost --color-monitor
```

Atari STF preset:

```bash
go run ./cmd/gost --preset stf
```

Atari ST preset:

```bash
go run ./cmd/gost --preset st
```

Atari Mega ST preset:

```bash
go run ./cmd/gost --preset mega-st
```

Headless mode:

```bash
make headless
```

```bash
go run ./cmd/gost --headless --frames 300
```

Headless color desktop boot:

```bash
go run ./cmd/gost --headless --color-monitor --frames 400 --dump-frame /tmp/gost-color-desktop.png
```

Headless boot inspection with a PNG dump:

```bash
go run ./cmd/gost --headless --frames 60 --trace boot --dump-frame /tmp/gost-boot.png
```

Verbose headless boot inspection:

```bash
go run ./cmd/gost --headless --frames 20 --trace boot-verbose
```

Late-boot trace inspection in a custom PC range:

```bash
go run ./cmd/gost --headless --frames 20 --trace boot-verbose --trace-start 0xE16780 --trace-end 0xE16820
```

With a floppy image:

```bash
make run ARGS="--floppy-a /path/to/disk.msa"
```

```bash
go run ./cmd/gost --floppy-a /path/to/disk.msa
```

Override the bundled OS:

```bash
go run ./cmd/gost --rom /path/to/tos.rom
```

Example JSON config:

```json
{
  "preset": "mega-st",
  "floppy-a": "/path/to/disk.msa",
  "hd-size-mb": 0,
  "cpu-mhz": 8,
  "color-monitor": false,
  "trace-start": "0xE00000",
  "trace-end": "0xE01000"
}
```

Run with the config file and optionally override individual settings on the CLI:

```bash
go run ./cmd/gost --config /path/to/gost.json --headless --frames 400
```

Load order is: preset defaults, then JSON config file, then CLI flags.

JSON config keys use the same names as the CLI flags, just without the leading `--`.

## WebAssembly

Yes: this project already compiles to `GOOS=js GOARCH=wasm`, and the bundled EmuTOS image makes a browser build practical without adding ROM download steps.

Build the browser demo assets into `docs/`:

```bash
make wasm
```

Serve the generated files locally:

```bash
python3 -m http.server --directory docs 8000
```

Then open [http://localhost:8000](http://localhost:8000).

The repository also includes a GitHub Pages workflow at [`./.github/workflows/pages.yml`](./.github/workflows/pages.yml).

Current browser-build limitations:

- The browser build always boots the bundled EmuTOS image.
- CLI paths such as `--rom`, `--floppy-a`, `--hd-size-mb`, `--hd-image`, and `--dump-frame` remain desktop/headless features unless a browser-side file picker is added later.
- The generated `.wasm` binary must be served over HTTP; opening `docs/index.html` directly from disk will not work.

### CLI Flags

- `--config <path>`: optional JSON config file loaded before CLI overrides
- `--preset <name>`: machine preset, currently `default`, `stf`, `st`, or `mega-st`
- `--rom <path>`: path to the TOS ROM image
- `--floppy-a <path>`: optional floppy disk image for drive A (`.st` or `.msa`)
- `--ram-size <bytes>`: emulated RAM size in bytes
- `--clock-hz <n>`: base machine clock frequency in Hz
- `--hd-size-mb <n>`: virtual ACSI hard disk size in MiB (default `30`, set `0` to disable)
- `--hd-image <path>`: optional persistent ACSI hard disk image file; loads if present, otherwise creates from `--hd-size-mb`
- `--cpu-mhz <n>`: CPU frequency in MHz (default `8`); increases/decreases CPU speed without changing other hardware timing
- `--frame-hz <n>`: display and VBL refresh rate in Hz; frame timing is derived from `clock-hz / frame-hz`
- `--scale <n>`: window scale factor, default `1`
- `--fullscreen`: start fullscreen
- `--headless`: run without opening a window
- `--color-monitor`: emulate an Atari color monitor instead of monochrome
- `--midres-y-scale <n>`: scale medium-resolution display height on host output (`1` = off)
- `--frames <n>`: number of frames to run in headless mode, default `500`
- `--trace <mode>`: enable tracing, currently `cpu`, `cpu-verbose`, `boot`, `boot-verbose`, `shifter`, or `shifter-verbose`
- `--trace-start <addr>`: first PC included in `boot` and `boot-verbose` traces, default `0xE00000`
- `--trace-end <addr>`: last PC included in `boot` and `boot-verbose` traces, default `0xE01000`
- `--dump-frame <path>`: write the last rendered framebuffer to a PNG file

## Development

Run tests:

```bash
make test
```

```bash
go test ./...
```

Debug-oriented emulator probes are kept behind a build tag so the default suite stays fast:

```bash
go test -tags debugtests ./internal/emulator
```

Build everything:

```bash
make build
```

```bash
go build ./...
```

See available targets:

```bash
make help
```

## Concurrency Notes

At this stage, the emulator core is intentionally single-threaded. The CPU, bus, memory map, device advancement, and interrupt dispatch currently run in a deterministic lockstep loop. That makes bring-up, debugging, and test behavior much easier to reason about while the hardware models are still incomplete.

Using goroutines "wherever possible" is not recommended yet. For an emulator, broad early concurrency tends to introduce races, lock contention, and timing bugs before correctness is established. The current priority is accurate and predictable behavior rather than parallel execution.

### Recommended Near-Term Approach

- Keep the emulation core single-threaded.
- Prefer goroutines only at host-side boundaries such as async file loading, trace/log streaming, debugger tooling, or future audio buffering.
- If concurrency is introduced in the machine layer, prefer a single emulation goroutine that owns all `Machine` state and accepts input/events through channels.

### Shifter Guidance

The shifter is a possible future concurrency boundary, but not in its current form. Today it renders directly from live RAM and live register state, so moving rendering to another goroutine would require synchronization around RAM, palette registers, resolution, base address, and framebuffer ownership.

If shifter work is parallelized later, the safer design is:

- Keep emulation-side shifter state single-threaded.
- At frame boundaries, capture an immutable snapshot of the visible video state.
- Include the screen base, resolution, palette, and the RAM bytes needed for the visible frame.
- Hand that snapshot to a renderer goroutine that converts bitplanes into an RGBA back buffer.
- Present the completed back buffer on a later host frame.

This preserves deterministic emulation while creating room for asynchronous framebuffer conversion and double-buffering.

## Current Implementation Notes

- The CPU core is provided by `m68kemu`; this repo does not implement its own 68000.
- If no ROM path is passed, `cmd/gost` boots the bundled EmuTOS image by default.
- The machine runs with an 8 MHz default clock and 50 Hz frame cadence.
- The video path renders from RAM-backed bitplanes into an RGBA framebuffer for the host frontend.
- Interrupts are routed into the CPU through the machine layer.
- The floppy controller now covers WD1772 command groups (type I/II/III/IV) over sector images, including seek/step commands, sector and track DMA reads/writes, and read-address support.
- A virtual ACSI hard disk is attached by default with 30 MiB capacity.
- Use `--hd-image` to persist hard-disk contents across emulator restarts.

## Known Gaps

- Real TOS boot coverage beyond the bundled EmuTOS image is not complete yet
- MMU/GLUE behavior is still incomplete
- Shifter timing and register coverage are partial
- IKBD protocol coverage is incomplete
- MIDI and copy-protected disk format support are still missing

## Next Steps

- Improve MMU/GLUE/Shifter behavior for broader TOS compatibility
- Expand MFP coverage and timing accuracy
- Flesh out IKBD and ACIA behavior to match TOS expectations
- Improve ACSI hard-disk command coverage and real-software compatibility
- Build debugger and trace tooling around the existing machine core

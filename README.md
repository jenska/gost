# GoST - Atari ST Emulator in Go

<p align="center">
  <img src="assets/media/gost.png" alt="GoST logo" width="240">
</p>

GoST is an Atari ST emulator in Go built around [`github.com/jenska/m68kemu`](https://github.com/jenska/m68kemu) for Motorola 68000 CPU emulation.

## Status

This repository currently provides a working emulator foundation:

- `m68kemu` is wired in as the CPU core.
- A desktop frontend is available via Ebitengine.
- The project has an ST-oriented bus and device model for RAM, ROM, Shifter, MFP, IKBD/ACIA, FDC, and PSG.
- Headless execution, tracing hooks, and a basic test suite are in place.

This is not yet a complete Atari ST emulator that boots real TOS to the GEM desktop. The hardware models are intentionally simplified and meant as the base for continued bring-up.

Current 400-frame headless boot state:

![Current emulation status](assets/media/gost-status.png)

Current bring-up note:

- EmuTOS reaches video setup and renders the panic screen shown above rather than the GEM desktop.
- The current blocker is in late interrupt/device bring-up, especially mixed VBL and MFP behavior.
- Recent work added better boot tracing, a more accurate ST device map, and reduced false interrupt sources to narrow the remaining failure.

## Features

- Go 1.26 project layout with a runnable `cmd/gost` entrypoint
- `github.com/jenska/m68kemu` pinned as the CPU emulator dependency
- 24-bit address bus via a local ST bus wrapper
- Boot-vector proxy at address `0x000000`
- ROM aliases in high memory
- 1 MiB RAM default machine profile
- Low and medium resolution Shifter framebuffer conversion
- Minimal MFP timer and interrupt support
- Minimal IKBD/ACIA keyboard and mouse event path
- Simplified sector-based floppy controller with `.st` image support
- CPU and boot trace options for bring-up and debugging
- Headless framebuffer dump to PNG for boot inspection

## Project Layout

```text
cmd/gost                CLI entrypoint
internal/emulator       Machine orchestration, config, and ST bus wiring
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

Headless mode:

```bash
make headless
```

```bash
go run ./cmd/gost --headless --frames 300
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
make run ARGS="--floppy-a /path/to/disk.st"
```

```bash
go run ./cmd/gost --floppy-a /path/to/disk.st
```

Override the bundled OS:

```bash
make run ARGS="--os /path/to/tos.rom"
```

```bash
go run ./cmd/gost --os /path/to/tos.rom
```

### CLI Flags

- `--rom <path>`: path to the TOS ROM image
- `--os <path>`: alias for `--rom`
- `--floppy-a <path>`: optional floppy disk image for drive A
- `--scale <n>`: window scale factor, default `2`
- `--fullscreen`: start fullscreen
- `--headless`: run without opening a window
- `--frames <n>`: number of frames to run in headless mode, default `300`
- `--trace <mode>`: enable tracing, currently `cpu`, `cpu-verbose`, `boot`, or `boot-verbose`
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
- The floppy controller is intentionally simplified and currently models sector reads rather than full WD1772 behavior.

## Known Gaps

- Real TOS boot to GEM is not complete yet
- MMU/GLUE behavior is still incomplete
- Shifter timing and register coverage are partial
- MFP support is minimal and only suitable for early bring-up
- IKBD protocol coverage is incomplete
- Audio generation is not implemented yet
- No STE features, blitter, hard disk, MIDI, or copy-protected disk format support yet

## Next Steps

- Improve MMU/GLUE/Shifter behavior for real TOS bring-up
- Expand MFP coverage and timing accuracy
- Flesh out IKBD and ACIA behavior to match TOS expectations
- Deepen WD1772 emulation beyond simple sector access
- Add real YM2149 audio output
- Build debugger and trace tooling around the existing machine core

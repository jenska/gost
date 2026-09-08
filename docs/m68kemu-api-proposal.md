# m68kemu API changes to simplify gost integration

Status: partially implemented
Author: gost maintainers
Target: `github.com/jenska/m68kemu` (currently v1.4.0)
Scope: additive where possible; one semantic change (item 2) behind an opt-in

## 0. Progress

Landed in `m68kemu` (branch `api-additive-hooks-fastram`) and wired into `gost`:

- **3.6 `WithDeferredReset`** — implemented in `m68kemu`; not yet adopted by
  `gost` (the current construction order already leaves devices in a valid
  post-cold-reset state, so it buys little until the scheduler work).
- **3.5 `SetHooks`** — implemented and adopted; `EnableTrace` now installs the
  whole callback set in one call.
- **3.1 fast memory** — implemented as `SetFastMemory(...FastRegion)` (multi
  region, with a `ReadOnly` flag and bus-matching wait-state accounting).
  `gost` maps a read-only window over each isolated ROM mirror only. A low-RAM
  window was prototyped and dropped: the shifter's variable video-DMA
  contention penalty cannot be reproduced on a flat region, so it made cycle
  counts drift from the bus model. ROM has no such penalty, so the ROM window
  is cycle-exact (verified by a 45-frame boot-parity test) and still removes
  ~8% of an EmuTOS-boot benchmark, since EmuTOS executes entirely from ROM.

Also landed, as a separate surface-polish pass (`m68kemu` gost is the sole
consumer): `RequestInterrupt` takes a plain `uint8` vector with an `AutoVector`
constant instead of `*uint8` (the minimum of **3.3**); `NewCPU` takes `*Bus`;
`SetPreTracer` / `SetInterruptTracer` folded into `SetHooks`; and
`InterruptController`, `MappedDevice`, `ScheduledEvent`, `WaitHook`,
`Bus.SetWaitHook` unexported.

Primitives landed in `m68kemu` (branch `api-scheduler-irq`) but **not adopted by
`gost`**:

- **3.2 `CycleScheduler.SetClockRatio`** and **3.3 `CPU.SetIRQSource`** — both
  implemented, tested, opt-in (the queue and the 1:1 scheduler are unchanged
  when unused).
- Wiring `gost` onto the scheduler was tried and reverted. It is behaviourally
  correct — the debug-trace suite and the fast/bus cycle-parity tests were
  unchanged — but ~2× slower: the current devices advance in an "advance me by
  N cycles" style, so as `CycleListener`s they do their per-quantum work (GLUE
  scanline loop, MFP GPIP-edge scan, interrupt drain) on *every instruction*
  instead of every ~512-cycle quantum. The scheduler pays off only after the
  device layer is rewritten event-driven (each device schedules its next state
  change), which is its own project. Until then `gost` keeps the quantum loop
  and `cpuCyclesForHardwareCycles`.

- **3.4 optional `Contains`** — landed. `m68kemu`'s `Device` no longer requires
  `Contains`; a device is located by `AddressRangeDevice` (page-mapped) and/or
  `ContainsDevice`. `gost`'s eight single-range peripherals now expose
  `AddressRange()` and drop `Contains`; the shifter, ROM, RAM, overlay,
  memory-config, and multi-range regions keep `Contains`. The bus resolves each
  check once, so the multi-device lookup is slightly faster. Debug-trace suite
  unchanged.

Not started: the device-layer event-driven rewrite that 3.2/3.3 adoption
depends on.

## 1. Motivation

`gost` drives `m68kemu` as the CPU core of an Atari ST. Three subsystems that
`m68kemu` already ships are reimplemented inside `gost` because the library
versions do not fit the ST's timing and interrupt model, and a fourth
(the interpreter fast path) is unreachable from a multi-device bus.

| gost implements | m68kemu already has | why the library version is unused |
|---|---|---|
| `clocked []Clocked`, `advanceDevices`, `EventPredictor`, `nextStepQuantum`, `cpuCyclesForHardwareCycles`, `cpuCycleCarry` | `CycleScheduler`, `CycleListener`, `Schedule`/`ScheduleAfter` | no CPU-clock ↔ device-clock ratio; no "advance to next scheduled event" from inside `RunCycles` |
| `irqSources`, `InterruptSource.DrainInterrupts`, `dispatchInterrupts`, `maskedAutovectorPulse` | `InterruptController`, `CPU.RequestInterrupt` | controller queues every request indefinitely and never coalesces; a masked autovector pulse is delivered late instead of dropped, so `gost` inspects `cpu.Registers().SR` by hand to discard it |
| `Machine.RunUntil` — a per-instruction loop that re-aggregates `RunResult` | `CPU.RunUntil` | cannot advance devices or sample interrupts between instructions, so `gost` forces `MaxInstructions = 1` and defeats the internal loop |
| — | `Bus.fastRAM`, `cpu.fastFetchMem` single-RAM fast path | only enabled when the bus holds exactly one device and it is a `*m68kemu.RAM` (`bus.go`, `refreshTopology`); `gost`'s bus has ~15 devices and its own `devices.RAM` |

Relevant `gost` files: `internal/emulator/machine_runtime.go`,
`internal/emulator/machine_builder.go`, `internal/emulator/machine_io.go`,
`internal/devices/device.go`, `internal/devices/glue.go`,
`internal/devices/mfp.go`.

## 2. Current data flow

### 2.1 Frame stepping (`machine_runtime.go`)

```
StepFrame:
  shifter.BeginFrame()
  remaining = frameCycles                       // 8 MHz "hardware" cycles
  while remaining > 0:
    dispatchInterrupts()                        // drain every irqSource -> cpu.RequestInterrupt
    quantum   = min(remaining, 512, nextDeviceEventCycles())
    cpuQuant  = cpuCyclesForHardwareCycles(quantum)   // rational scale + carry
    cpu.RunCycles(cpuQuant)
    advanceDevices(quantum)                     // every Clocked.Advance(quantum)
    shifter.AdvanceFrame(quantum)
    dispatchInterrupts()
    remaining -= quantum
  shifter.EndFrame()
```

`nextDeviceEventCycles()` polls every `EventPredictor` for the soonest state
change so the quantum never steps over an HBL boundary. `cpuCycleCarry` holds the
sub-cycle remainder of the `CPUClockHz / ClockHz` conversion.

### 2.2 Interrupts

`GLUE` (`glue.go`) appends a level-2 pulse per scanline and a level-4 pulse at
frame end into a slice. `MFP` does the same for its timers. `dispatchInterrupts`
calls `DrainInterrupts()` on each source, then for every returned `Interrupt`:

```go
func (m *Machine) maskedAutovectorPulse(irq devices.Interrupt) bool {
    if irq.Vector != nil {            // vectored (MFP) interrupts are never dropped
        return false
    }
    mask := uint8((m.cpu.Registers().SR >> 8) & 0x7)
    return irq.Level <= mask          // autovector pulse masked right now -> drop it
}
```

This hand-rolled masking exists only because `InterruptController.Request`
enqueues the pulse and delivers it whenever the mask next drops, which is wrong
for the ST's edge-triggered autovector lines.

### 2.3 `RunUntil` (debugger / test stepping)

`Machine.RunUntil` copies the caller's `RunUntilOptions`, sets
`MaxInstructions = 1`, calls `cpu.RunUntil` in a loop, and after each instruction
runs `advanceDevices(advanced)` + `dispatchInterrupts()`, then merges
`result.Instructions/Cycles/PC/Exception/BusAccess/Interrupt` back into a running
`RunResult` — about 50 lines that duplicate the library's own loop.

## 3. Proposals

Ordered by payoff. Items 1, 3, 4, 5, 6 are backward compatible. Item 2 changes
`CycleListener` timing only for callers that opt in with `SetClockRatio`.

### 3.1 Caller-supplied fast-memory window

```go
// SetFastRAM installs a flat byte slice that the interpreter reads and writes
// directly for addresses in [base, base+len(mem)), bypassing the bus. The caller
// guarantees the region is plain RAM: no access side effects, no MMU remapping,
// stable for the lifetime of the mapping. Passing a zero-length slice clears it.
// Addresses outside the window fall through to the bus unchanged.
func (c *CPU) SetFastRAM(base uint32, mem []byte)
```

Cleaner alternative — keep the guarantee on the device:

```go
// A device may implement FastMemory to expose a directly-addressable slice.
// The bus picks the widest such region and hands it to the core.
type FastMemory interface {
    FastSlice() (base uint32, mem []byte, ok bool)
}
```

`gost` maps only the always-present low ST RAM (below the MMU bank-switch,
"absent", and high-mirror ranges handled in `devices/ram.go`). TOS executes
almost entirely from low RAM, so this restores most of the throughput lost to the
`deviceForAddress -> page-map binary search -> MappedDevice bounds check ->
interface call -> RAM.translate()` chain that every fetch and RAM word currently
pays.

Interaction: the fast window must be consulted **before** breakpoints and bus
tracing are disabled — i.e. it participates in the same `refreshRunModes` gate
that already governs `fastFetchMem`. When any execute/read/write breakpoint or
bus tracer is active, the core must route through the bus so those hooks still
fire.

### 3.2 Clock ratio on the scheduler

```go
// SetClockRatio makes scheduler time run in device cycles rather than CPU
// cycles. After SetClockRatio(deviceHz, cpuHz), CycleListener.AdvanceCycles
// deltas, Now(), and Schedule() "at" values are all expressed in device cycles,
// and the fractional remainder of the conversion is retained internally.
// The default 1:1 ratio preserves current behaviour.
func (s *CycleScheduler) SetClockRatio(deviceHz, cpuHz uint64)
```

With this, `gost` registers `glue`, `mfp`, `acia`, `fdc`, `psg`, `steSound`, and
`shifter` as `CycleListener`s and deletes: `advanceDevices`, `nextStepQuantum`,
`nextDeviceEventCycles`, `cpuCyclesForHardwareCycles`, `cpuCycleCarry`,
`stepQuantumCycles`, and the `Clocked` and `EventPredictor` interfaces. Devices
that predict events (`GLUE.NextEventCycles`, `MFP.NextEventCycles`) instead call
`scheduler.ScheduleAfter(delta, fn)` when their state last changed; the scheduler
already fires callbacks at the exact cycle and bounds `Advance` to the next
event, which is what `nextDeviceEventCycles` approximated.

Open question: `CycleScheduler.Advance` is currently called from
`cpu.addCycles`, i.e. after each instruction's cycle cost is booked. That is fine
for device advancement. The shifter needs writes to `FF82xx` registers to take
effect at the right raster position; per-instruction granularity (typ. 4-40
cycles) is finer than today's 512-cycle quantum, so this is an improvement, but
the shifter's `BeginFrame`/`EndFrame` bracketing still has to be driven by
`gost` around `RunCycles`.

### 3.3 Pull-based interrupt line

```go
// IRQSource reports the interrupt line state sampled at each instruction
// boundary, before the next opcode is fetched. level 0 means no request.
// When autovector is true, vector is ignored and the CPU uses 24+level.
type IRQSource interface {
    PendingIRQ() (level uint8, vector uint8, autovector bool)
}

func (c *CPU) SetIRQSource(IRQSource)
```

This is the Musashi / UAE model: the line is level-sensitive. A request that is
masked when sampled simply is not taken yet and is re-sampled next boundary; if
the device lowers the line first, it is never taken. ST-style HBL/VBL edge
behaviour becomes the device's decision — `GLUE` lowers its line after the CPU
acknowledges (observed via the existing `InterruptCallback`).

`gost` deletes `DrainInterrupts`, `dispatchInterrupts`, `maskedAutovectorPulse`,
the `irqSources` slice, and the `Interrupt` / `InterruptSource` types. In their
place, one aggregator:

```go
func (m *Machine) PendingIRQ() (uint8, uint8, bool) {
    // highest of GLUE / MFP / ACIA / FDC lines; MFP wins ties with its vector
}
```

Because interrupts are now sampled inside the core, `cpu.RunUntil` advances
devices (via 3.2) and interrupts correctly on its own, so **`Machine.RunUntil`
is deleted** and callers use `m.cpu.RunUntil(options)` directly.

Minimum viable version if the full redesign is too large for one release:

```go
const AutoVector uint8 = 0   // sentinel for RequestInterrupt

func (c *CPU) RequestInterrupt(level, vector uint8) error   // was (level uint8, vector *uint8)
```

plus coalescing in `InterruptController` so a device that re-requests the same
level every boundary does not grow the queue without bound.

### 3.4 Optional `Contains` for range devices

Every `gost` device implements both `Contains(uint32) bool` and the
`AddressRangeDevice` interface `AddressRange() (start, end uint32)`. The
hand-written `Contains` methods are a recurring bug source — see the high-mirror
and MMU-size logic in `devices/ram.go:Contains`.

Proposal: if a device implements `AddressRangeDevice` but not `Device.Contains`,
the bus synthesises containment from the range. Split the interface:

```go
type Device interface {
    Read(Size, uint32) (uint32, error)
    Write(Size, uint32, uint32) error
    Reset()
}

type ContainsDevice interface {   // optional; only for non-contiguous decode
    Contains(address uint32) bool
}
```

Most `gost` devices then shrink to `AddressRange` + `Read` + `Write` + `Reset`.

### 3.5 One hooks struct

```go
type Hooks struct {
    Trace       TraceCallback
    PreTrace    PreTraceCallback
    Exception   ExceptionCallback
    Bus         BusAccessCallback
    Interrupt   InterruptCallback
}

func (c *CPU) SetHooks(Hooks)   // replaces the five SetXxxTracer setters
```

`gost`'s `EnableTrace` currently nils four setters and then re-sets a subset on
every mode change, each call re-running `refreshDebugModes` / `refreshRunModes`.
A single struct swap is atomic and runs the refresh once. Keep the individual
setters as thin wrappers for compatibility.

### 3.6 Deferred reset in `NewCPU`

`NewCPU` calls `c.Reset()` internally, which reads the reset vector from a bus
whose devices `gost` has not finished wiring. `gost` immediately calls
`Machine.Reset` again after construction. Add:

```go
type Option func(*config)
func WithDeferredReset() Option
func NewCPU(bus AddressBus, opts ...Option) (CPU, error)
```

so construction does not touch the bus and the first real reset is the caller's.

## 4. Effect on gost

Deleted (~250-300 lines):

- `machine_runtime.go`: `nextStepQuantum`, `nextDeviceEventCycles`,
  `cpuCyclesForHardwareCycles`, `advanceDevices`, `dispatchInterrupts`,
  `maskedAutovectorPulse`, `Machine.RunUntil`, fields `cpuCycleCarry`
- `device.go`: `Clocked`, `EventPredictor`, `InterruptSource`, `Interrupt`
- `machine_builder.go`: `clockedDevices`, the `irqSources` slice literal
- `glue.go` / `mfp.go`: `DrainInterrupts`, `draining` scratch buffers,
  `NextEventCycles` (replaced by `ScheduleAfter` calls)

`StepFrame` after the change:

```go
func (m *Machine) StepFrame() (bool, error) {
    m.shifter.BeginFrame()
    if err := m.cpu.RunCycles(m.frameCycles); err != nil {
        return false, err
    }
    frame := m.shifter.EndFrame()
    if m.traceShifter() {
        m.traceShifterFrame(frame)
    }
    m.frameCounter++
    return frame, nil
}
```

Setup in `NewMachineWithCartridge`:

```go
sched := cpu.NewCycleScheduler()
sched.SetClockRatio(cfg.ClockHz, cfg.CPUClockHz)
for _, d := range []cpu.CycleListener{glue, mfp, acia, fdc, psg, shifter} {
    sched.AddListener(d)
}
if steSound != nil { sched.AddListener(steSound) }
processor.SetScheduler(sched)
processor.SetIRQSource(machineIRQ{glue, mfp, acia, fdc})
if lo := ram.FlatLowRegion(); len(lo) != 0 {
    processor.SetFastRAM(0x000000, lo)
}
```

## 5. Compatibility and rollout

| Item | Break? | Suggested release |
|---|---|---|
| 3.1 `SetFastRAM` | additive | minor |
| 3.2 `SetClockRatio` | additive; default 1:1 unchanged | minor |
| 3.3 `IRQSource` | additive; old `RequestInterrupt` kept | minor, deprecate queue path |
| 3.3 min. `RequestInterrupt` non-pointer | signature break | major, or new `RequestIRQ` name |
| 3.4 optional `Contains` | additive if `ContainsDevice` is a new optional interface | minor |
| 3.5 `SetHooks` | additive | minor |
| 3.6 `WithDeferredReset` | additive | minor |

Recommended: ship 3.1, 3.2, 3.5, 3.6 in one minor release; land 3.3 and 3.4 in
the next once `gost` has migrated its device layer.

## 6. Risks

- **3.2**: moving device time inside `RunCycles` means a device callback that
  itself touches the bus (DMA) runs mid-instruction-budget rather than at a
  quantum edge. `gost`'s blitter and FDC DMA already assume they can read ST RAM
  at any point, so this should be safe, but needs a regression pass against the
  floppy and blitter test suites.
- **3.3**: the ST relies on HBL firing every scanline even under load. A
  level-sensitive line that the device forgets to lower would stall the guest in
  the level-2 handler. The `GLUE` change must lower the line in the same place it
  currently clears `pending`.
- **3.1**: if the fast window ever overlaps an address the MMU can remap
  (bank switch, `memoryAddressAbsent`), reads would bypass that logic silently.
  `gost` must expose only the region below `MemoryConfig.LogicalSize()`'s
  smallest possible value, and re-call `SetFastRAM` (or clear it) if the machine
  ever gains runtime-reconfigurable low RAM.

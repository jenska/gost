# GoST Emulator Compatibility Roadmap

**Analysis Date:** May 12, 2026  
**Target:** Full Atari 1040STF Compatibility  
**Current Status:** EmuTOS Desktop Boot (Partial Hardware Implementation)

---

## Executive Summary

GoST has reached a milestone with EmuTOS desktop boot capability, but significant hardware and software compatibility work remains for a true 1040STF emulator. Current implementation covers:

✅ **Working:**
- 68000 CPU core (via m68kemu)
- Basic Shifter (ST & STE models)
- MFP timers and interrupt delivery
- IKBD/ACIA keyboard & mouse
- Blitter (immediate execution model)
- PSG/YM2149 audio (YM2149 library)
- Floppy DMA/FDC (sector-based, ST/MSA images)
- Virtual ACSI hard disk (FAT16)
- VBL interrupts
- ROM overlay boot
- EmuTOS 1.4 boot to desktop

⚠️ **Partially Working:**
- STE Shifter (screen base addressing improvements added but incomplete)
- Serial I/O (handshaking stubs, no real UART)
- GPIP input simulation (monitor detect, limited)
- FDC command set (basic subset only)

❌ **Missing/Incomplete:**
- True hardware-cycle accurate bus contention
- Multiple disk formats (ADI/HDI, D64, IMD, raw ST images with bad sectors)
- Real-time clock (RTC) emulation
- Printer port (Parallel / Centronics)
- MIDI port
- Modem port (second ACIA channel)
- STE DMA sound
- STE/Mega ST extended hardware
- Cartridge ROM support
- Network/Ethernet
- Precise timing of various hardware subsystems

---

## 1. Hardware/Device Gap Analysis

### 1.1 Implemented Hardware

| Device | Status | Location | Notes |
|--------|--------|----------|-------|
| M68000 CPU | ✅ Complete | m68kemu (external) | Via github.com/jenska/m68kemu |
| RAM | ✅ Complete | internal/devices/ram.go | 512K-2MB support |
| ROM | ✅ Complete | internal/devices/rom.go | 256K/512K images |
| Shifter (ST) | ⚠️ Partial | internal/devices/shifter_st.go | Low/Medium/High res, palette |
| Shifter (STE) | ⚠️ Partial | internal/devices/shifter_ste.go | Missing line offset, fine scroll details |
| Blitter | ✅ Functional | internal/devices/blitter.go | Immediate execution, no cycle timing |
| MFP 68901 | ⚠️ Partial | internal/devices/mfp.go | Timers, basic GPIP, no serial I/O details |
| ACIA | ✅ Basic | internal/devices/acia.go | IKBD channel only (channel 1 not used) |
| IKBD | ✅ Basic | internal/devices/ikbd.go | Keyboard, mouse, clock queries |
| FDC WD1772 | ⚠️ Partial | internal/devices/fdc.go | Basic commands, sector-based only |
| PSG/YM2149 | ✅ Functional | internal/devices/psg.go | Via ym2149 library, audio output |
| VBL Source | ✅ Complete | internal/devices/vbl.go | 50 Hz autovector |
| GLUE | ⚠️ Stub | internal/devices/glue.go | Minimal register model |
| STE Sound | ✅ Stub | internal/devices/ste_sound.go | Returns bus error (correct for ST) |

### 1.2 Missing Hardware Devices

| Device | Category | Impact | Complexity |
|--------|----------|--------|-----------|
| **Real-Time Clock (RTC)** | System | Medium | HIGH - Requires time tracking, GPIP integration |
| **Parallel/Printer Port** | I/O | Low-Medium | MEDIUM - Device model + interrupt integration |
| **MIDI Port** | I/O | Low | MEDIUM - Basic serial protocol, no timing |
| **Modem Port (ACIA Ch. 1)** | I/O | Low | MEDIUM - Second ACIA channel implementation |
| **STE DMA Sound** | Audio | High (STE) | HIGH - Complex timing, DMA integration |
| **Mega ST Extended Memory** | Memory | Medium (Mega) | MEDIUM - Bank switching, dual MFP |
| **Cartridge ROM** | Memory | Low | LOW - Simple address mapping |
| **Network/Ethernet** | I/O | Very Low | VERY HIGH - Requires network stack |
| **SCSI Controller (ST Compact)** | Storage | Low | MEDIUM - Command set, DMA |
| **Floppy Controller Edge Cases** | Storage | Medium | HIGH - Various disk formats, error conditions |

---

## 2. Incomplete Hardware Implementation Details

### 2.1 Shifter (Both ST & STE)

**Current Implementation:**
```go
// shifter.go lines 1-100
type Shifter struct {
    cfg            *config.Config
    ram            *RAM
    model          shifterModel
    width, height  int
    framebuffer    []byte
    // ... palette, base address, video address tracking
}
```

**Missing/Incomplete:**
- ❌ Bus contention modeling (shifter steals cycles from CPU)
  - Current: Simplified model
  - Required: Accurate DMA slot timing per scanline
  - Impact: Graphics glitches in software with tight timing
  
- ❌ Line-to-line screen base changes
  - Current: Uses base address from start of frame
  - Required: Per-scanline base address updates (STE supports this)
  - Impact: Mid-screen scrolling effects fail
  
- ⚠️ Fine scroll (ST) / Horizontal scroll (STE)
  - Current: STE fine scroll partially implemented
  - Missing: Precise pixel-level shifting in rendering
  - Impact: Smooth scrolling demos show artifacts
  
- ❌ Sync/blank timing precision
  - Current: Coarse 8-way horizontal split
  - Required: Cycle-accurate blank/display boundaries
  - Impact: Border effects in demos (e.g., "Overscan")
  
- ❌ Monochrome mode edge cases
  - Current: Works for basic desktop
  - Missing: Extended monochrome timing, special monitor detection
  
- ⚠️ Screen base addressing on STE
  - Current: 24-bit addressing implemented
  - Missing: Some edge cases with dynamically-updated base

**Test Coverage:**
- ✅ Basic framebuffer generation
- ✅ Palette manipulation
- ✅ Screen base changes per frame
- ❌ Mid-frame base changes
- ❌ Bus contention timing effects

**Files Affected:**
- [internal/devices/shifter.go](internal/devices/shifter.go#L1-L50)
- [internal/devices/shifter_st.go](internal/devices/shifter_st.go)
- [internal/devices/shifter_ste.go](internal/devices/shifter_ste.go)

### 2.2 MFP 68901 (Multi-Function Peripheral)

**Current Implementation:**
```go
// mfp.go - 4 timers, basic GPIP, interrupt routing
type MFP struct {
    registers    [mfpSize]byte
    timers       [4]mfpTimer
    aciaIRQActive bool
    // ... serial stub fields
}
```

**Missing/Incomplete:**
- ❌ **Serial I/O (UART)**
  - Current: Stub registers (UCR, RSR, TSR, UDR) always return "ready"
  - Missing: Actual UART baud rate generation, data framing, parity
  - Impact: Real serial devices won't work; TOS 1.0+ may timeout
  - Complexity: HIGH (requires cycle-accurate UART simulation)

- ❌ **GPIP Interrupt Detection (AER/DDR)**
  - Current: GPIP register readable, DDR/AER stored but not used
  - Missing: Edge detection logic for AER transitions
  - Missing: Proper input/output direction control via DDR
  - Impact: Hardware detection (RTC, cartridge) may fail
  - Complexity: MEDIUM

- ❌ **Timer Output Connections**
  - Current: Timers generate interrupts only
  - Missing: Timer outputs used for other purposes (PSG clock, DMA timing)
  - Impact: Some timing-sensitive software fails
  - Complexity: MEDIUM

- ⚠️ **Timer Mode Details**
  - Current: Basic count-down implemented
  - Missing: Pulse mode, one-shot vs. continuous details
  - Impact: Minor for typical software

- ⚠️ **Interrupt Mask Edge Cases**
  - Current: IERA/IERB/IMRA/IMRB implemented
  - Missing: ISR/IPR level-based priority handling
  - Impact: Rare multi-priority scenarios

**Test Coverage:**
- ✅ Timer counting and interrupts
- ✅ Basic register reads/writes
- ❌ Serial I/O sequences
- ❌ GPIP edge detection
- ❌ Timer output signals

**Files Affected:**
- [internal/devices/mfp.go](internal/devices/mfp.go#L1-L100)
- Tests: [internal/devices/mfp_test.go](internal/devices/mfp_test.go)

### 2.3 FDC WD1772 (Floppy Disk Controller)

**Current Implementation:**
```go
// fdc.go - Sector-based, command subset
type FDC struct {
    diskA        []byte      // Sector-based disk image
    status, track, sector, data byte
    // ... ACSI hard disk, DMA state
}
```

**Missing/Incomplete:**
- ❌ **Track-Level Commands**
  - Current: Type I commands (Restore, Seek, Step) work at track level
  - Missing: Type IV (Read Track, Write Track, Force Interrupt)
  - Missing: Raw track data handling (non-sector-based)
  - Impact: Copy protection schemes, low-level disk tools fail
  - Complexity: HIGH

- ❌ **Disk Format Support**
  - Current: ✅ ST (raw sector) and ✅ MSA (compressed)
  - Missing: ADI (Atari Disk Image), HDI, D64, IMD, raw with bad sectors
  - Impact: Many real disks can't be used
  - Complexity: MEDIUM-HIGH

- ❌ **Error Simulation**
  - Current: Always succeeds or returns "not found"
  - Missing: CRC errors, lost data, disk defects, write protection violations
  - Impact: Error handling code in real software untested
  - Complexity: HIGH

- ❌ **Multi-Sector Operations**
  - Current: Sector count respected, but transfers are instant
  - Missing: Multi-track reads, proper DMA interleaving
  - Impact: Large file I/O might be unreliable
  - Complexity: MEDIUM

- ⚠️ **Motor Control**
  - Current: Motor status bits returned, but no timing simulation
  - Missing: Motor spin-up delay
  - Impact: Minor for typical software

- ❌ **Write Protection**
  - Current: Flag returned but enforcement inconsistent
  - Missing: Proper per-disk write protection enforcement

- ❌ **ACSI Hard Disk Commands**
  - Current: Minimal SCSI command subset
  - Missing: Full SCSI command set (READ, WRITE, etc.)
  - Missing: Sense data, unit attention
  - Impact: Some TOS versions and disk utilities fail
  - Complexity: MEDIUM-HIGH

**Test Coverage:**
- ✅ Basic read/write sectors
- ✅ Seek operations
- ✅ MSA image loading
- ❌ Track-level operations
- ❌ Multi-format support
- ❌ Error conditions
- ❌ SCSI command handling

**Files Affected:**
- [internal/devices/fdc.go](internal/devices/fdc.go#L1-L100)
- [internal/emulator/disk_image.go](internal/emulator/disk_image.go)
- Tests: [internal/devices/fdc_test.go](internal/devices/fdc_test.go)

### 2.4 IKBD & ACIA (Keyboard/Mouse/Serial)

**Current Implementation:**
```go
// ikbd.go - Command queue, mouse tracking
type IKBD struct {
    queue       []byte
    mouseMode   ikbdMouseMode  // Relative or Absolute
    absX, absY  uint16
}

// acia.go - Two channels, IKBD on channel 0
type ACIA struct {
    ikbd        *IKBD
    control, status, data [2]byte
}
```

**Missing/Incomplete:**
- ❌ **ACIA Channel 1 (Modem Port)**
  - Current: Not wired up to any device
  - Missing: Implementation as I/O device
  - Impact: Serial device support, modem software won't work
  - Complexity: MEDIUM

- ❌ **IKBD Timing**
  - Current: Commands processed immediately
  - Missing: Realistic command processing delay
  - Impact: Timing-sensitive input code may fail

- ⚠️ **IKBD Interrogate Commands**
  - Current: Self-test, mouse interrogate partially work
  - Missing: Full response sequences
  - Impact: Some utilities may probe and fail

- ❌ **Clock Setting/Reading**
  - Current: Returns stub time
  - Missing: Integration with system RTC (not implemented)
  - Impact: Date/time functions return wrong values

**Test Coverage:**
- ✅ Keyboard input
- ✅ Mouse motion and buttons
- ✅ Basic IKBD commands
- ❌ Modem port
- ❌ Real timing

**Files Affected:**
- [internal/devices/ikbd.go](internal/devices/ikbd.go)
- [internal/devices/acia.go](internal/devices/acia.go)
- Tests: [internal/devices/ikbd_test.go](internal/devices/ikbd_test.go)

### 2.5 Blitter

**Current Implementation:**
```go
// blitter.go - Immediate register execution
type Blitter struct {
    ram   *RAM
    regs  [blitterSize]byte  // Register window
}
```

**Missing/Incomplete:**
- ⚠️ **Cycle-Accurate Execution**
  - Current: Blitter operations execute immediately when BUSY written
  - Missing: Realistic blit timing based on operation parameters
  - Impact: Race conditions in software relying on blit duration
  - Complexity: MEDIUM

- ❌ **HOG Bit Behavior**
  - Current: HOG status bit written but not enforced
  - Missing: CPU yield while blit in progress
  - Impact: Timing-critical code sees no delay

- ⚠️ **Nbit Operations**
  - Current: Basic binary operations work
  - Missing: Complex operation modes
  - Impact: Rare for typical software

**Test Coverage:**
- ✅ Basic blit operations
- ✅ Blitter register reads/writes
- ❌ Cycle timing
- ❌ Contention with CPU

**Files Affected:**
- [internal/devices/blitter.go](internal/devices/blitter.go)
- Tests: [internal/devices/blitter_test.go](internal/devices/blitter_test.go)

### 2.6 PSG/YM2149

**Current Implementation:**
```go
// psg.go - Wrapper around ym2149 library
type PSG struct {
    clockDomain *ym2149.ClockDomain
    chip        *ym2149.Chip
}
```

**Status:** ✅ Generally Complete (via external library)

**Minor Gaps:**
- ⚠️ Port B control (drive control) partially modeled
- ⚠️ Sound output integration with emulator timing

---

## 3. Software Compatibility Assessment

### 3.1 Known Working Software

| Software | Type | Status | Notes |
|----------|------|--------|-------|
| **EmuTOS 1.4** | OS | ✅ Boots to desktop | Bundled, tested extensively |
| **GEM Desktop** | UI | ✅ Interactive | Keyboard, mouse, menu functions work |
| **GEM VDI** | Graphics | ✅ Partial | Blitter exercised during boot |
| **TOS 1.0x** | OS | ✅ Boots | Via separate ROM images |
| **TOS 1.02** | OS | ✅ Boots | Via separate ROM images |
| **TOS 1.04** | OS | ✅ Boots | Via separate ROM images |
| **EmuTOS 1.4 (Color)** | OS | ✅ Boots | Color desktop mode |

### 3.2 Known Failing / Untested Software

| Category | Examples | Issue |
|----------|----------|-------|
| **Copy-Protected Games** | Mainly 1980s-90s releases | Track-level FDC commands needed |
| **Low-Level Disk Tools** | HDCopy, Kyroflop, FastCopy | Raw track I/O, special FDC modes |
| **3D Graphics** | Falcon 030 features | Not implemented (different CPU) |
| **Network Software** | Spectre GCN, NetBSD | No networking hardware |
| **Real-Time Apps** | MIDI sequencers, audio apps | Timing inaccuracy, no MIDI port |
| **Hard Disk Utilities** | ICD RTC, partition tools | Missing RTC, limited SCSI support |
| **Cartridge Software** | Various | No cartridge ROM support |
| **Second Floppy Drive** | Multi-drive sequences | Single drive (A) only, no B |

### 3.3 Disk Image Format Support

| Format | Read | Write | Notes |
|--------|------|-------|-------|
| **.ST (Raw)** | ✅ | ✅ | Standard sector image |
| **.MSA (Compressed)** | ✅ | ❌ | Decompressed in memory |
| **.ADI** | ❌ | ❌ | Atari Disk Image |
| **.HDI** | ❌ | ❌ | Hard Disk Image |
| **.D64** | ❌ | ❌ | Commodore 64 (rarely used on ST) |
| **.IMD** | ❌ | ❌ | ImageDisk format |
| **Raw + Bad Sectors** | ❌ | ❌ | Special sector markers |

---

## 4. Test Coverage Analysis

### 4.1 Test Files by Component

```
internal/devices/
  ✅ acia_test.go             - IKBD channel, basic reads/writes
  ✅ blitter_benchmark_test.go - Performance testing
  ✅ blitter_test.go          - Basic operations
  ✅ bus_error_region_test.go  - Error handling
  ✅ fixed_value_region_test.go- Fixed value regions
  ✅ glue_test.go             - GLUE register model
  ✅ ikbd_test.go             - Keyboard, mouse, commands
  ✅ mfp_test.go              - Timers, interrupts, GPIP
  ❌ NO: Serial I/O specific tests
  ❌ NO: GPIP edge detection tests
  ✅ psg_test.go              - PSG register reads/writes
  ✅ shifter_benchmark_test.go - Framebuffer generation
  ✅ shifter_test.go          - Low/Medium/High res rendering
  ✅ ste_sound_test.go        - STE sound stub behavior
  ❌ NO: Shifter contention tests
  ❌ NO: Mid-frame base change tests
  ✅ fdc_test.go              - Basic disk operations
  ❌ NO: Track-level FDC tests
  ❌ NO: Multi-format disk tests
  ✅ mmu_test.go              - Memory addressing

internal/emulator/
  ✅ machine_test.go              - Machine creation, basic stepping
  ✅ machine_trace_test.go         - Trace mode functionality
  ✅ disk_image_test.go            - Image loading and parsing
  ✅ hard_disk_file_test.go        - Virtual disk file handling
  ✅ debug_emutos_test.go          - EmuTOS boot (requires debugtests tag)
  ✅ debug_floppy_test.go          - Floppy operations (requires debugtests tag)
  ✅ debug_mouse_test.go           - Mouse handling (requires debugtests tag)
  ✅ debug_panic_test.go           - Panic condition detection
  ✅ debug_fontcopy_test.go        - Font copying operations
  ✅ debug_process_test.go         - Process execution
  ✅ debug_test_helpers_test.go    - Test utility functions
```

### 4.2 Coverage Gaps

| Area | Gap | Impact |
|------|-----|--------|
| **Serial Communication** | No UART tests | Can't verify real serial protocols |
| **Multi-Format Disks** | Only ST/MSA tested | ADI/HDI/etc. unsupported |
| **Track-Level FDC** | No tests | Copy protection unverifiable |
| **Real TOS Images** | Limited testing | TOS 1.0x boot not verified |
| **Graphics Effects** | No demo tests | Scrolling, mid-frame effects untested |
| **Hard Disk Utils** | Basic ACSI only | SCSI commands incomplete |
| **Timing Precision** | No cycle-count tests | Contention not verified |
| **Edge Cases** | Limited error paths | Exception handling partial |

---

## 5. Prioritized Implementation Roadmap

### PHASE 1: SHORT-TERM (1-2 Months) - Stability & Core Completeness

**Goal:** Solidify desktop environment and fix critical bugs

#### 1.1 FDC Enhancements
- **Implement ADI/HDI disk format support**
  - Files: [internal/emulator/disk_image.go](internal/emulator/disk_image.go)
  - Effort: 1 week
  - Impact: Many more real disks become usable
  - Tests needed: Format detection, decompression, geometry inference

- **Add disk error simulation**
  - Files: [internal/devices/fdc.go](internal/devices/fdc.go)
  - Effort: 3-4 days
  - Impact: Real disk utilities can be tested
  - Tests: Error conditions, retry logic

- **Implement Type IV FDC commands (Read/Write Track)**
  - Files: [internal/devices/fdc.go](internal/devices/fdc.go#L200-L400)
  - Effort: 1 week
  - Impact: Copy protection, low-level tools work
  - Complexity: HIGH
  - Tests: Track data validation, sector placement

#### 1.2 Shifter Bus Contention
- **Model DMA cycle stealing**
  - Files: [internal/devices/shifter.go](internal/devices/shifter.go)
  - Effort: 1 week
  - Impact: Proper CPU/shifter timing, graphics stability
  - Tests: Cycle count validation per scanline
  
- **Implement mid-frame screen base changes**
  - Files: [internal/devices/shifter.go](internal/devices/shifter.go)
  - Effort: 3-4 days
  - Impact: Scrolling effects work
  - Tests: Base address tracking, render validation

#### 1.3 GPIP Edge Detection (AER/DDR)
- **Implement data direction register (DDR) enforcement**
  - Files: [internal/devices/mfp.go](internal/devices/mfp.go#L200-L250)
  - Effort: 2-3 days
  - Impact: Hardware detection works properly
  - Tests: Input/output separation, write rejection

- **Implement active edge register (AER) triggering**
  - Files: [internal/devices/mfp.go](internal/devices/mfp.go)
  - Effort: 3-4 days
  - Impact: Interrupt-driven I/O possible
  - Tests: Edge detection, interrupt firing

#### 1.4 Documentation & Testing
- Create test suite for real TOS images (TOS 1.0, 1.02, 1.04)
  - Effort: 2-3 days
  - Impact: Regressions caught early

- Document real-world software compatibility
  - Effort: 1-2 days
  - Impact: Users know what works/doesn't

**Completion Criteria:**
- ✅ 90%+ disk image support (ST, MSA, ADI, HDI)
- ✅ FDC error paths functional
- ✅ Track-level commands working
- ✅ Shifter contention affecting CPU timing correctly
- ✅ GPIP DDR/AER fully functional
- ✅ Test suite passes with 5+ real TOS images

**Estimated Timeline:** 6-8 weeks

---

### PHASE 2: MEDIUM-TERM (2-4 Months) - Real-World Compatibility

**Goal:** Support broader real-world Atari ST software

#### 2.1 Real-Time Clock (RTC)
- **Implement ICD RTC emulation**
  - Impact: Date/time functions work
  - Complexity: MEDIUM-HIGH
  - Effort: 2-3 weeks
  - Files: New `internal/devices/rtc.go`
  - Integration points: GPIP input lines, port memory mapping
  - Tests: Date/time queries, hardware detection

#### 2.2 Parallel/Printer Port
- **Add printer port device model**
  - Impact: Printer drivers can be tested
  - Complexity: MEDIUM
  - Effort: 1-2 weeks
  - Files: New `internal/devices/parallel.go`
  - Registers: Parallel port control (0xFFBF00-0xFFBF10)
  - Tests: Data transmission, handshake signals

#### 2.3 ACIA Channel 1 (Modem Port)
- **Wire up second ACIA channel**
  - Impact: Serial device software works
  - Complexity: MEDIUM
  - Effort: 1 week
  - Files: [internal/devices/acia.go](internal/devices/acia.go), new serial device stub
  - Tests: Serial communication simulation

#### 2.4 MFP Serial I/O (UART)
- **Implement complete UART model**
  - Impact: Real serial devices possible
  - Complexity: HIGH
  - Effort: 3-4 weeks
  - Files: [internal/devices/mfp.go](internal/devices/mfp.go#L600-L700)
  - Sub-tasks:
    - Baud rate timing (async clock generation)
    - Data framing (5/6/7/8 bits, parity, stop bits)
    - Flow control (RTS/CTS, DTR/DSR)
    - Error handling (overrun, framing error, parity error)
  - Tests: Baud rate accuracy, data integrity

#### 2.5 STE Hardware Support
- **Improve STE Shifter (line offset, fine scroll)**
  - Impact: STE boot and demos work
  - Complexity: MEDIUM
  - Effort: 1-2 weeks
  - Files: [internal/devices/shifter_ste.go](internal/devices/shifter_ste.go)

- **Implement STE DMA Sound**
  - Impact: STE audio programs work
  - Complexity: VERY HIGH
  - Effort: 4-6 weeks
  - Files: New `internal/devices/ste_dma_sound.go`
  - Integration: DMA controller, interrupt routing, audio path

**Completion Criteria:**
- ✅ 80%+ real Atari ST games/applications run
- ✅ RTC detection and basic queries work
- ✅ Printer port emulated
- ✅ Serial device I/O functional
- ✅ STE-specific hardware operational
- ✅ Test coverage includes 10+ real software titles

**Estimated Timeline:** 10-14 weeks

---

### PHASE 3: LONG-TERM (4+ Months) - Advanced Features

**Goal:** Achieve comprehensive Atari ST compatibility

#### 3.1 Advanced FDC Features
- **Multi-format disk support expansion**
  - D64 (Commodore, rare on ST)
  - IMD (ImageDisk format)
  - Raw tracks with bad sectors
  - Effort: 2-4 weeks
  - Tests: Format-specific compliance

- **Advanced SCSI command set**
  - Full READ/WRITE commands
  - Sense data reporting
  - Unit attention conditions
  - Effort: 2-3 weeks
  - Impact: Hard disk utilities work

#### 3.2 Hardware Refinement
- **Cycle-accurate bus contention**
  - Precise CPU/shifter/blitter timing
  - DMA slot allocation
  - Wait state insertion
  - Effort: 4-6 weeks
  - Complexity: VERY HIGH
  - Impact: Timing-critical software works

- **Blitter cycle timing**
  - Accurate operation duration
  - HOG bit enforcement
  - Effort: 2-3 weeks

- **MFP timer output connections**
  - Timer outputs to PSG clock, sound, etc.
  - Effort: 1-2 weeks

#### 3.3 System Features
- **Cartridge ROM support**
  - Cartridge detection
  - Address mapping (0xFA0000-0xFBFFFF)
  - Effort: 1 week
  - Impact: Cartridge games/utilities run

- **Mega ST extended features**
  - Dual MFP support
  - Bank switching for 4MB RAM
  - Effort: 2-3 weeks
  - Complexity: MEDIUM-HIGH

#### 3.4 Advanced Testing & Debugging
- **Real-world software test suite**
  - 50+ games, utilities, demos
  - Automated compatibility testing
  - Effort: 4-6 weeks

- **Timing validation harness**
  - Cycle-count verification
  - Bus access tracing
  - Performance profiling
  - Effort: 2-3 weeks

**Completion Criteria:**
- ✅ 95%+ Atari ST software compatibility
- ✅ All standard hardware features operational
- ✅ Cycle accuracy for mainstream operations
- ✅ Comprehensive test coverage
- ✅ Full 1040STF emulation feature-complete

**Estimated Timeline:** 16+ weeks

---

## 6. Technical Debt & Cleanup

### 6.1 Code Quality Items

| Item | Severity | Effort | Impact |
|------|----------|--------|--------|
| Extract shifter contention to separate module | MEDIUM | 1 week | Clarity, testability |
| Refactor FDC command dispatch | MEDIUM | 1 week | Maintainability |
| Add comprehensive error handling to disk I/O | MEDIUM | 2-3 days | Robustness |
| Consolidate duplicate bus access patterns | LOW | 3-4 days | Code duplication |
| Complete MFP register documentation | LOW | 2-3 days | Maintainability |

### 6.2 Documentation
- Hardware register reference guide
- Device implementation guide for contributors
- Real-world software compatibility matrix
- Performance profiling documentation

---

## 7. Risk Factors & Mitigation

### High-Risk Areas

| Area | Risk | Mitigation |
|------|------|-----------|
| **Bus Contention** | Complex timing interactions | Early integration testing with real software |
| **FDC Compatibility** | Many disk formats | Create format test suite first |
| **STE DMA Sound** | Complex DMA + audio timing | Prototype with simple waveforms first |
| **Real RTC** | Requires system integration | Use system clock as base |
| **Serial I/O** | Baud rate precision | Create clock domain test utilities |

---

## 8. Feature Dependency Graph

```
EmuTOS Desktop (✅ Done)
├── Shifter framebuffer
│   └── Bus contention (Phase 1)
├── MFP timers
│   └── GPIP edge detection (Phase 1)
├── IKBD/ACIA
│   └── ACIA Ch1 (Phase 2)
├── FDC
│   ├── Track-level commands (Phase 1)
│   ├── ADI/HDI formats (Phase 1)
│   └── SCSI commands (Phase 2)
└── PSG audio
    └── STE DMA Sound (Phase 2)

Real TOS Compatibility (Phase 2)
├── Serial I/O (Phase 2)
├── RTC support (Phase 2)
├── Printer port (Phase 2)
└── Hardware detection (GPIP, Phase 1)

Advanced Software (Phase 3)
├── Cycle-accurate timing
├── Cartridge ROM
├── Mega ST features
└── 50+ real software titles
```

---

## 9. Success Metrics

### Phase 1 Completion
- [ ] Track-level FDC commands working
- [ ] 5+ disk formats supported
- [ ] GPIP DDR/AER fully functional
- [ ] Shifter contention affecting CPU timing
- [ ] 5 different TOS versions boot successfully
- [ ] Test coverage at 85%+

### Phase 2 Completion
- [ ] RTC emulated and functional
- [ ] Serial I/O with real devices possible
- [ ] 30+ games/utilities tested
- [ ] STE hardware operational
- [ ] 90%+ software compatibility

### Phase 3 Completion
- [ ] 95%+ software compatibility
- [ ] Cycle-accurate operation
- [ ] Full 1040STF feature parity
- [ ] 50+ software titles passing
- [ ] Production-ready emulator

---

## 10. References & Resources

### Hardware Documentation
- Atari ST Hardware Manual (Atari Corporation)
- WD1772 FDC Specification
- 68901 MFP Datasheet
- YM2149 PSG Documentation
- GEM Programmer's Reference

### External Libraries Used
- `github.com/jenska/m68kemu` - 68000 CPU emulation
- `github.com/jenska/ym2149` - YM2149 PSG emulation

### Related Emulator Projects
- Hatari (C implementation, reference)
- Previous (Macintosh, ST emulation)
- Steem (Windows ST emulator)
- ARAnyM (Atari Falcon Linux emulator)

---

## Appendix A: Hardware Feature Checklist

```
[ ] RAM (512K-4MB configurable)
[ ] ROM (256K/512K)
[ ] Shifter (ST mode, STE mode, Mega ST)
    [ ] Low resolution
    [ ] Medium resolution
    [ ] High resolution
    [ ] Palette management
    [ ] Screen base addressing
    [ ] Bus contention
    [ ] Mid-frame base changes
    [ ] Fine/Horizontal scroll
[ ] Blitter
    [ ] All operation modes
    [ ] Cycle-accurate timing
    [ ] Bus HOG functionality
[ ] MFP 68901
    [ ] Timer A, B, C, D
    [ ] Interrupt routing
    [ ] GPIP register
    [ ] GPIP edge detection (AER)
    [ ] Data direction (DDR)
    [ ] Serial I/O (UART)
[ ] ACIA
    [ ] Keyboard channel (Ch 0)
    [ ] Modem channel (Ch 1)
    [ ] Control/Status/Data registers
[ ] IKBD
    [ ] Keyboard input
    [ ] Mouse (relative mode)
    [ ] Mouse (absolute mode)
    [ ] Commands
    [ ] Clock queries
[ ] FDC WD1772
    [ ] Type I commands
    [ ] Type II commands
    [ ] Type III commands
    [ ] Type IV commands
    [ ] DMA integration
    [ ] Error reporting
    [ ] Write protection
    [ ] Multi-sector operations
    [ ] Disk formats (ST, MSA, ADI, HDI, etc.)
[ ] PSG/YM2149
    [ ] Tone generation
    [ ] Envelope control
    [ ] Port A/B
    [ ] Noise generation
[ ] VBL interrupts (50 Hz)
[ ] RTC (Real-Time Clock)
[ ] Parallel/Printer port
[ ] MIDI port
[ ] Cartridge ROM
[ ] Extended memory (Mega ST)
```

---

**Last Updated:** May 12, 2026  
**Document Version:** 1.0

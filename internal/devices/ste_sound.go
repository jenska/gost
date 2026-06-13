package devices

import (
	"fmt"

	cpu "github.com/jenska/m68kemu"
)

const (
	steSoundBase = 0xFF8900
	steSoundSize = 0x40

	steSoundRegControl      = 0xFF8901
	steSoundRegFrameBaseHi  = 0xFF8903
	steSoundRegFrameBaseMid = 0xFF8905
	steSoundRegFrameBaseLow = 0xFF8907
	steSoundRegCounterHi    = 0xFF8909
	steSoundRegCounterMid   = 0xFF890B
	steSoundRegCounterLow   = 0xFF890D
	steSoundRegFrameEndHi   = 0xFF890F
	steSoundRegFrameEndMid  = 0xFF8911
	steSoundRegFrameEndLow  = 0xFF8913
	steSoundRegMode         = 0xFF8921
	steSoundRegMicrowire    = 0xFF8922
	steSoundRegMicrowireEnd = 0xFF8926

	steSoundControlEnable = 0x01
	steSoundControlLoop   = 0x02
	steSoundModeMono      = 0x80

	steSoundOutputSampleRate = 48_000
	steSoundOutputGain       = 0.5
)

var steSoundDMASampleRates = [4]uint64{6258, 12517, 25033, 50066}

// STESound models the STE 8-bit PCM DMA sound registers and a mono host audio
// stream. It intentionally keeps MICROWIRE as register storage only; DAC volume
// and tone-control effects can be layered on later without changing callers.
type STESound struct {
	ram     *RAM
	clockHz uint64

	control byte
	mode    byte

	frameStart uint32
	frameEnd   uint32
	current    uint32

	microwireData uint16
	microwireMask uint16

	outputPhase uint64
	dmaPhase    uint64
	lastSample  float32
	samples     []float32
}

func NewSTESound(ram *RAM, clockHz uint64) *STESound {
	if clockHz == 0 {
		clockHz = defaultShifterClockHz
	}
	s := &STESound{ram: ram, clockHz: clockHz}
	s.Reset()
	return s
}

func NewAbsentSTESound() *BusErrorRegion {
	return NewBusErrorRegion(AddressRange{Start: steSoundBase, End: steSoundBase + steSoundSize})
}

func (s *STESound) Contains(address uint32) bool {
	return address >= steSoundBase && address < steSoundBase+steSoundSize
}

func (s *STESound) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (s *STESound) Read(size cpu.Size, address uint32) (uint32, error) {
	count, err := steSoundAccessSize(size)
	if err != nil {
		return 0, err
	}
	if !s.accessInRange(address, count) {
		return 0, cpu.BusError(address)
	}

	var value uint32
	for i := range count {
		value = value<<8 | uint32(s.readByte(address+uint32(i)))
	}
	return value, nil
}

func (s *STESound) Peek(size cpu.Size, address uint32) (uint32, error) {
	return s.Read(size, address)
}

func (s *STESound) Write(size cpu.Size, address uint32, value uint32) error {
	count, err := steSoundAccessSize(size)
	if err != nil {
		return err
	}
	if !s.accessInRange(address, count) {
		return cpu.BusError(address)
	}

	for i := range count {
		shift := uint((count - 1 - i) * 8)
		s.writeByte(address+uint32(i), byte(value>>shift))
	}
	return nil
}

func (s *STESound) Reset() {
	s.control = 0
	s.mode = 0
	s.frameStart = 0
	s.frameEnd = 0
	s.current = 0
	s.microwireData = 0
	s.microwireMask = 0
	s.outputPhase = 0
	s.dmaPhase = 0
	s.lastSample = 0
	s.samples = s.samples[:0]
}

func (s *STESound) Advance(cycles uint64) {
	if cycles == 0 || s.clockHz == 0 {
		return
	}
	s.outputPhase += cycles * steSoundOutputSampleRate
	for s.outputPhase >= s.clockHz {
		s.outputPhase -= s.clockHz
		if s.playing() {
			s.advanceDMASample()
		} else {
			s.lastSample = 0
		}
		s.samples = append(s.samples, s.lastSample)
	}
}

func (s *STESound) DrainMonoF32(dst []float32) int {
	n := copy(dst, s.samples)
	copy(s.samples, s.samples[n:])
	s.samples = s.samples[:len(s.samples)-n]
	return n
}

func (s *STESound) OutputSampleRate() int {
	return steSoundOutputSampleRate
}

func (s *STESound) readByte(address uint32) byte {
	switch address {
	case steSoundRegControl:
		return s.control & (steSoundControlEnable | steSoundControlLoop)
	case steSoundRegFrameBaseHi:
		return byte(s.frameStart >> 16 & 0x3F)
	case steSoundRegFrameBaseMid:
		return byte(s.frameStart >> 8)
	case steSoundRegFrameBaseLow:
		return byte(s.frameStart) & 0xFE
	case steSoundRegCounterHi:
		return byte(s.current >> 16 & 0x3F)
	case steSoundRegCounterMid:
		return byte(s.current >> 8)
	case steSoundRegCounterLow:
		return byte(s.current) & 0xFE
	case steSoundRegFrameEndHi:
		return byte(s.frameEnd >> 16 & 0x3F)
	case steSoundRegFrameEndMid:
		return byte(s.frameEnd >> 8)
	case steSoundRegFrameEndLow:
		return byte(s.frameEnd) & 0xFE
	case steSoundRegMode:
		return s.mode & (steSoundModeMono | 0x03)
	default:
		if address >= steSoundRegMicrowire && address < steSoundRegMicrowireEnd {
			return s.readMicrowireByte(address)
		}
		return 0
	}
}

func (s *STESound) writeByte(address uint32, value byte) {
	switch address {
	case steSoundRegControl:
		s.writeControl(value)
	case steSoundRegFrameBaseHi:
		s.frameStart = (s.frameStart & 0x00FFFF) | (uint32(value&0x3F) << 16)
		s.syncCurrentToStartWhenIdle()
	case steSoundRegFrameBaseMid:
		s.frameStart = (s.frameStart & 0x3F00FF) | (uint32(value) << 8)
		s.syncCurrentToStartWhenIdle()
	case steSoundRegFrameBaseLow:
		s.frameStart = (s.frameStart & 0x3FFF00) | uint32(value&0xFE)
		s.syncCurrentToStartWhenIdle()
	case steSoundRegFrameEndHi:
		s.frameEnd = (s.frameEnd & 0x00FFFF) | (uint32(value&0x3F) << 16)
	case steSoundRegFrameEndMid:
		s.frameEnd = (s.frameEnd & 0x3F00FF) | (uint32(value) << 8)
	case steSoundRegFrameEndLow:
		s.frameEnd = (s.frameEnd & 0x3FFF00) | uint32(value&0xFE)
	case steSoundRegMode:
		s.mode = value & (steSoundModeMono | 0x03)
	default:
		if address >= steSoundRegMicrowire && address < steSoundRegMicrowireEnd {
			s.writeMicrowireByte(address, value)
		}
	}
}

func (s *STESound) writeControl(value byte) {
	wasPlaying := s.playing()
	s.control = value & (steSoundControlEnable | steSoundControlLoop)
	if !s.playing() {
		s.lastSample = 0
		return
	}
	if !wasPlaying {
		s.current = s.frameStart
		s.dmaPhase = 0
		s.lastSample = 0
	}
	if s.frameEnd <= s.frameStart {
		s.stopPlayback()
	}
}

func (s *STESound) syncCurrentToStartWhenIdle() {
	if !s.playing() {
		s.current = s.frameStart
	}
}

func (s *STESound) readMicrowireByte(address uint32) byte {
	switch address {
	case 0xFF8922:
		return byte(s.microwireData >> 8)
	case 0xFF8923:
		return byte(s.microwireData)
	case 0xFF8924:
		return byte(s.microwireMask >> 8)
	case 0xFF8925:
		return byte(s.microwireMask)
	default:
		return 0
	}
}

func (s *STESound) writeMicrowireByte(address uint32, value byte) {
	switch address {
	case 0xFF8922:
		s.microwireData = (s.microwireData & 0x00FF) | uint16(value)<<8
	case 0xFF8923:
		s.microwireData = (s.microwireData & 0xFF00) | uint16(value)
	case 0xFF8924:
		s.microwireMask = (s.microwireMask & 0x00FF) | uint16(value)<<8
	case 0xFF8925:
		s.microwireMask = (s.microwireMask & 0xFF00) | uint16(value)
	}
}

func (s *STESound) advanceDMASample() {
	rate := steSoundDMASampleRates[s.mode&0x03]
	s.dmaPhase += rate
	for s.dmaPhase >= steSoundOutputSampleRate && s.playing() {
		s.dmaPhase -= steSoundOutputSampleRate
		s.lastSample = s.nextSample()
	}
}

func (s *STESound) nextSample() float32 {
	if s.mode&steSoundModeMono != 0 {
		value, ok := s.readDMAByte()
		if !ok {
			return 0
		}
		return stePCMByteToFloat(value)
	}

	left, ok := s.readDMAByte()
	if !ok {
		return 0
	}
	right, ok := s.readDMAByte()
	if !ok {
		return stePCMByteToFloat(left) * 0.5
	}
	return (stePCMByteToFloat(left) + stePCMByteToFloat(right)) * 0.5
}

func (s *STESound) readDMAByte() (byte, bool) {
	if !s.playing() || s.ram == nil {
		s.stopPlayback()
		return 0, false
	}
	if s.current >= s.frameEnd {
		if !s.finishFrame() {
			return 0, false
		}
	}

	value, err := s.ram.Read(cpu.Byte, s.current)
	if err != nil {
		s.stopPlayback()
		return 0, false
	}
	s.current++
	if s.current >= s.frameEnd {
		s.finishFrame()
	}
	return byte(value), true
}

func (s *STESound) finishFrame() bool {
	if s.control&steSoundControlLoop == 0 {
		s.stopPlayback()
		return false
	}
	s.current = s.frameStart
	if s.frameEnd <= s.frameStart {
		s.stopPlayback()
		return false
	}
	return true
}

func (s *STESound) stopPlayback() {
	s.control &^= steSoundControlEnable
	s.lastSample = 0
}

func (s *STESound) playing() bool {
	return s.control&steSoundControlEnable != 0
}

func (s *STESound) accessInRange(address uint32, count int) bool {
	if count <= 0 {
		return false
	}
	end := address + uint32(count) - 1
	return s.Contains(address) && s.Contains(end)
}

func steSoundAccessSize(size cpu.Size) (int, error) {
	switch size {
	case cpu.Byte:
		return 1, nil
	case cpu.Word:
		return 2, nil
	case cpu.Long:
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported STE sound access size %d", size)
	}
}

func stePCMByteToFloat(value byte) float32 {
	return float32(int8(value)) / 128 * steSoundOutputGain
}

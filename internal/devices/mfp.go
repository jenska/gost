package devices

import cpu "github.com/jenska/m68kemu"

const (
	mfpBase               = 0xFFFA00
	mfpSize               = 0x40
	defaultMFPHostClockHz = 8_000_000
	// Timer C divide-by-64 with data 192 is the ST's 200 Hz system timer,
	// which implies a 2.4576 MHz MFP timer input clock.
	mfpTimerInputHz = 2_457_600

	mfpGPIP  = 0x01
	mfpAER   = 0x03
	mfpDDR   = 0x05
	mfpIERA  = 0x07
	mfpIERB  = 0x09
	mfpIPRA  = 0x0B
	mfpIPRB  = 0x0D
	mfpISRA  = 0x0F
	mfpISRB  = 0x11
	mfpIMRA  = 0x13
	mfpIMRB  = 0x15
	mfpVR    = 0x17
	mfpTACR  = 0x19
	mfpTBCR  = 0x1B
	mfpTCDCR = 0x1D
	mfpTADR  = 0x1F
	mfpTBDR  = 0x21
	mfpTCDR  = 0x23
	mfpTDDR  = 0x25
	mfpSCR   = 0x27
	mfpUCR   = 0x29
	mfpRSR   = 0x2B
	mfpTSR   = 0x2D
	mfpUDR   = 0x2F
)

type mfpTimer struct {
	channel   int
	dataReg   uint32
	control   byte
	counter   uint64
	prescaler uint64
	enabled   bool
}

// MFP models the STF's 68901 interrupt controller with timer-backed IRQs.
type MFP struct {
	registers      [mfpSize]byte
	vectorBase     uint8
	softwareEOI    bool
	timers         [4]mfpTimer
	serialBuffer   byte
	hostClockHz    uint64
	clockRemainder uint64
}

func NewMFP(hostClockHz uint64) *MFP {
	if hostClockHz == 0 {
		hostClockHz = defaultMFPHostClockHz
	}
	m := &MFP{hostClockHz: hostClockHz}
	m.Reset()
	return m
}

func (m *MFP) Contains(address uint32) bool {
	return address >= mfpBase && address < mfpBase+mfpSize
}

func (m *MFP) WaitStates(cpu.Size, uint32) uint32 {
	return 4
}

func (m *MFP) Reset() {
	clear(m.registers[:])
	m.vectorBase = 0x40
	m.softwareEOI = false
	m.registers[mfpVR] = m.vectorBase
	m.timers[0] = mfpTimer{channel: 13, dataReg: mfpTADR}
	m.timers[1] = mfpTimer{channel: 8, dataReg: mfpTBDR}
	m.timers[2] = mfpTimer{channel: 5, dataReg: mfpTCDR}
	m.timers[3] = mfpTimer{channel: 4, dataReg: mfpTDDR}
	m.serialBuffer = 0
	m.clockRemainder = 0
}

func (m *MFP) Read(size cpu.Size, address uint32) (uint32, error) {
	offset := address - mfpBase
	switch size {
	case cpu.Byte:
		return uint32(m.readByte(offset)), nil
	case cpu.Word:
		hi := m.readByte(offset)
		lo := m.readByte(offset + 1)
		return uint32(hi)<<8 | uint32(lo), nil
	default:
		return 0, nil
	}
}

func (m *MFP) Write(size cpu.Size, address uint32, value uint32) error {
	offset := address - mfpBase
	switch size {
	case cpu.Byte:
		m.writeByte(offset, byte(value))
	case cpu.Word:
		m.writeByte(offset, byte(value>>8))
		m.writeByte(offset+1, byte(value))
	}
	return nil
}

func (m *MFP) Advance(cycles uint64) {
	m.clockRemainder += cycles * mfpTimerInputHz
	ticks := m.clockRemainder / m.hostClockHz
	m.clockRemainder %= m.hostClockHz
	if ticks == 0 {
		return
	}

	for i := range m.timers {
		timer := &m.timers[i]
		if !timer.enabled || timer.prescaler == 0 {
			continue
		}

		remaining := ticks
		for remaining >= timer.counter && timer.counter > 0 {
			remaining -= timer.counter
			timer.counter = uint64(m.timerReloadValue(timer.dataReg)) * timer.prescaler
			m.raiseChannel(timer.channel)
		}
		if timer.counter > 0 {
			timer.counter -= remaining
		}
	}
}

func (m *MFP) DrainInterrupts() []Interrupt {
	channel, ok := m.nextPendingChannel()
	if !ok {
		return nil
	}

	m.clearRegisterBit(pendingRegisterForChannel(channel), channelBit(channel))
	if m.softwareEOI {
		m.setRegisterBit(serviceRegisterForChannel(channel), channelBit(channel))
	}

	vector := m.vectorBase + uint8(channel)
	return []Interrupt{{Level: 6, Vector: &vector}}
}

func (m *MFP) NextEventCycles() (uint64, bool) {
	minTicks := uint64(0)
	for i := range m.timers {
		timer := &m.timers[i]
		if !timer.enabled || timer.prescaler == 0 || timer.counter == 0 {
			continue
		}
		if minTicks == 0 || timer.counter < minTicks {
			minTicks = timer.counter
		}
	}
	if minTicks == 0 {
		return 0, false
	}

	numerator := minTicks*m.hostClockHz - m.clockRemainder
	if numerator == 0 {
		return 1, true
	}
	cycles := numerator / mfpTimerInputHz
	if numerator%mfpTimerInputHz != 0 {
		cycles++
	}
	if cycles == 0 {
		cycles = 1
	}
	return cycles, true
}

func (m *MFP) readByte(offset uint32) byte {
	switch offset {
	case mfpTADR:
		return m.timerCurrentValue(0)
	case mfpTBDR:
		return m.timerCurrentValue(1)
	case mfpTCDR:
		return m.timerCurrentValue(2)
	case mfpTDDR:
		return m.timerCurrentValue(3)
	case mfpUDR:
		return m.serialBuffer
	default:
		return m.registers[offset]
	}
}

func (m *MFP) writeByte(offset uint32, value byte) {
	switch offset {
	case mfpGPIP, mfpAER, mfpDDR, mfpSCR, mfpUCR, mfpRSR, mfpTSR:
		m.registers[offset] = value
	case mfpIERA, mfpIERB, mfpIMRA, mfpIMRB:
		m.registers[offset] = value
	case mfpIPRA, mfpIPRB:
		m.registers[offset] &= value
	case mfpISRA, mfpISRB:
		m.registers[offset] &= value
	case mfpVR:
		m.vectorBase = value & 0xF0
		m.softwareEOI = value&0x08 != 0
		m.registers[offset] = (value & 0xF8)
	case mfpTACR:
		m.registers[offset] = value
		m.configureTimer(0, value&0x0F)
	case mfpTBCR:
		m.registers[offset] = value
		m.configureTimer(1, value&0x0F)
	case mfpTCDCR:
		m.registers[offset] = value
		m.configureTimer(2, value>>4)
		m.configureTimer(3, value&0x0F)
	case mfpTADR:
		m.registers[offset] = value
		m.reloadTimer(0)
	case mfpTBDR:
		m.registers[offset] = value
		m.reloadTimer(1)
	case mfpTCDR:
		m.registers[offset] = value
		m.reloadTimer(2)
	case mfpTDDR:
		m.registers[offset] = value
		m.reloadTimer(3)
	case mfpUDR:
		m.serialBuffer = value
		m.registers[offset] = value
	default:
		m.registers[offset] = value
	}
}

func (m *MFP) configureTimer(index int, control byte) {
	timer := &m.timers[index]
	timer.control = control & 0x0F
	timer.prescaler = timerPrescaler(timer.control)
	timer.enabled = timer.prescaler != 0

	if !timer.enabled {
		timer.counter = 0
		return
	}

	timer.counter = uint64(m.timerReloadValue(timer.dataReg)) * timer.prescaler
}

func (m *MFP) reloadTimer(index int) {
	timer := &m.timers[index]
	if !timer.enabled || timer.prescaler == 0 {
		return
	}
	timer.counter = uint64(m.timerReloadValue(timer.dataReg)) * timer.prescaler
}

func (m *MFP) timerReloadValue(dataReg uint32) uint16 {
	value := m.registers[dataReg]
	if value == 0 {
		return 256
	}
	return uint16(value)
}

func timerPrescaler(mode byte) uint64 {
	switch mode {
	case 1:
		return 4
	case 2:
		return 10
	case 3:
		return 16
	case 4:
		return 50
	case 5:
		return 64
	case 6:
		return 100
	case 7:
		return 200
	default:
		return 0
	}
}

func (m *MFP) timerCurrentValue(index int) byte {
	timer := &m.timers[index]
	if !timer.enabled || timer.prescaler == 0 || timer.counter == 0 {
		return m.registers[timer.dataReg]
	}

	ticks := (timer.counter + timer.prescaler - 1) / timer.prescaler
	if ticks >= 256 {
		return 0
	}
	return byte(ticks)
}

func (m *MFP) raiseChannel(channel int) {
	m.setRegisterBit(pendingRegisterForChannel(channel), channelBit(channel))
}

func (m *MFP) nextPendingChannel() (int, bool) {
	blockedAt := -1
	if m.softwareEOI {
		blockedAt = m.highestInServiceChannel()
	}

	for channel := 15; channel >= 0; channel-- {
		if blockedAt >= 0 && channel <= blockedAt {
			continue
		}
		bit := channelBit(channel)
		if m.registers[pendingRegisterForChannel(channel)]&bit == 0 {
			continue
		}
		if m.registers[enableRegisterForChannel(channel)]&bit == 0 {
			continue
		}
		if m.registers[maskRegisterForChannel(channel)]&bit == 0 {
			continue
		}
		return channel, true
	}

	return 0, false
}

func (m *MFP) highestInServiceChannel() int {
	for channel := 15; channel >= 0; channel-- {
		bit := channelBit(channel)
		if m.registers[serviceRegisterForChannel(channel)]&bit != 0 {
			return channel
		}
	}
	return -1
}

func (m *MFP) setRegisterBit(offset uint32, bit byte) {
	m.registers[offset] |= bit
}

func (m *MFP) clearRegisterBit(offset uint32, bit byte) {
	m.registers[offset] &^= bit
}

func pendingRegisterForChannel(channel int) uint32 {
	if channel >= 8 {
		return mfpIPRA
	}
	return mfpIPRB
}

func serviceRegisterForChannel(channel int) uint32 {
	if channel >= 8 {
		return mfpISRA
	}
	return mfpISRB
}

func enableRegisterForChannel(channel int) uint32 {
	if channel >= 8 {
		return mfpIERA
	}
	return mfpIERB
}

func maskRegisterForChannel(channel int) uint32 {
	if channel >= 8 {
		return mfpIMRA
	}
	return mfpIMRB
}

func channelBit(channel int) byte {
	if channel >= 8 {
		return 1 << uint(channel-8)
	}
	return 1 << uint(channel)
}

package devices

import (
	"github.com/jenska/gost/internal/config"
	cpu "github.com/jenska/m68kemu"
)

const (
	mfpBase = 0xFFFA00
	mfpSize = 0x40
	// Timer C divide-by-64 with data 192 is the ST's 200 Hz system timer,
	// which implies a 2.4576 MHz MFP timer input clock.
	mfpTimerInputHz = 2_457_600

	mfpPALScanlines     = 313
	mfpNTSCScanlines    = 263
	mfpActiveVideoLines = 200
	mfpDefaultClockHz   = config.DefaultClockHz
	mfpDefaultFrameHz   = config.DefaultFrameHz

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

	mfpUSARTReceiveBufferFullChannel   = 12
	mfpUSARTTransmitBufferEmptyChannel = 10

	mfpRSRBufferFull     = 0x80
	mfpRSROverrunError   = 0x40
	mfpRSRParityError    = 0x20
	mfpRSRFramingError   = 0x10
	mfpRSRReceiverEnable = 0x01
	mfpTSRBufferEmpty    = 0x80
	mfpTSRUnderrunError  = 0x40
	mfpTSRTransmitterOn  = 0x01
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
	cfg            *config.Config
	registers      [mfpSize]byte
	vectorBase     uint8
	softwareEOI    bool
	inFlight       [16]bool
	aciaIRQActive  bool
	printerBusy    bool
	fdcIRQActive   bool
	gpipInput      byte
	rtc            *ICDRTC
	rs232          *RS232
	timers         [4]mfpTimer
	serialRx       byte
	serialRxReady  bool
	serialRxActive bool
	serialRxByte   byte
	serialRxCycles uint64
	serialTxActive bool
	serialTxByte   byte
	serialTxCycles uint64
	clockRemainder uint64

	eventCountFrameCycles uint64
	eventCountScanlines   uint64
	eventCountActiveLines uint64
	eventCountCycle       uint64
	eventCountNextLine    uint64
}

func NewMFP(cfg *config.Config) *MFP {
	m := &MFP{cfg: cfg}
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
	clear(m.inFlight[:])
	m.vectorBase = 0x40
	m.softwareEOI = false
	// Unmodeled GPIP inputs idle high on a plain ST, which prevents EmuTOS
	// from falsely detecting optional hardware like the ICD RTC.
	m.registers[mfpGPIP] = 0x6F
	m.registers[mfpVR] = m.vectorBase
	m.timers[0] = mfpTimer{channel: 13, dataReg: mfpTADR}
	m.timers[1] = mfpTimer{channel: 8, dataReg: mfpTBDR}
	m.timers[2] = mfpTimer{channel: 5, dataReg: mfpTCDR}
	m.timers[3] = mfpTimer{channel: 4, dataReg: mfpTDDR}
	m.serialRx = 0
	m.serialRxReady = false
	m.serialRxActive = false
	m.serialRxByte = 0
	m.serialRxCycles = 0
	m.serialTxActive = false
	m.serialTxByte = 0
	m.serialTxCycles = 0
	m.registers[mfpTSR] = mfpTSRBufferEmpty
	m.clockRemainder = 0
	m.configureEventCountTiming()
	m.eventCountCycle = 0
	m.eventCountNextLine = 1
	m.aciaIRQActive = false
	m.printerBusy = false
	m.fdcIRQActive = false
	m.gpipInput = m.gpipInputState()
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
	m.advanceUSART(cycles)
	m.advanceEventCountTimers(cycles)

	clockHz := m.clockHz()
	m.clockRemainder += cycles * mfpTimerInputHz
	ticks := m.clockRemainder / clockHz
	m.clockRemainder %= clockHz
	if ticks == 0 {
		return
	}

	for i := range m.timers {
		timer := &m.timers[i]
		if !timer.enabled || timer.control == 8 || timer.prescaler == 0 {
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
	m.updateGPIPEdges()

	channel, ok := m.nextPendingChannel()
	if !ok {
		return nil
	}

	m.clearRegisterBit(pendingRegisterForChannel(channel), channelBit(channel))
	if m.softwareEOI {
		m.inFlight[channel] = true
		m.setRegisterBit(serviceRegisterForChannel(channel), channelBit(channel))
	}

	vector := m.vectorBase + uint8(channel)
	return []Interrupt{{Level: 6, Vector: &vector}}
}

func (m *MFP) NextEventCycles() (uint64, bool) {
	minTicks := uint64(0)
	for i := range m.timers {
		timer := &m.timers[i]
		if !timer.enabled || timer.control == 8 || timer.prescaler == 0 || timer.counter == 0 {
			continue
		}
		if minTicks == 0 || timer.counter < minTicks {
			minTicks = timer.counter
		}
	}

	var cycles uint64
	if minTicks != 0 {
		numerator := minTicks*m.clockHz() - m.clockRemainder
		if numerator == 0 {
			cycles = 1
		} else {
			cycles = numerator / mfpTimerInputHz
			if numerator%mfpTimerInputHz != 0 {
				cycles++
			}
			if cycles == 0 {
				cycles = 1
			}
		}
	}
	if eventCycles, ok := m.nextEventCountCycles(); ok && eventCycles < cycles {
		return eventCycles, true
	}
	if serialCycles, ok := m.nextUSARTEventCycles(); ok && serialCycles < cycles {
		return serialCycles, true
	}
	if cycles == 0 {
		if eventCycles, ok := m.nextEventCountCycles(); ok {
			cycles = eventCycles
		}
		if serialCycles, ok := m.nextUSARTEventCycles(); ok && (cycles == 0 || serialCycles < cycles) {
			cycles = serialCycles
		}
		if cycles == 0 {
			return 0, false
		}
	}
	return cycles, true
}

func (m *MFP) readByte(offset uint32) byte {
	switch offset {
	case mfpGPIP:
		return m.gpipState()
	case mfpTADR:
		return m.timerCurrentValue(0)
	case mfpTBDR:
		return m.timerCurrentValue(1)
	case mfpTCDR:
		return m.timerCurrentValue(2)
	case mfpTDDR:
		return m.timerCurrentValue(3)
	case mfpRSR:
		return m.receiverStatus()
	case mfpTSR:
		return m.transmitterStatus()
	case mfpUDR:
		return m.readUSARTData()
	default:
		return m.registers[offset]
	}
}

func (m *MFP) writeByte(offset uint32, value byte) {
	switch offset {
	case mfpGPIP, mfpSCR, mfpUCR:
		m.registers[offset] = value
	case mfpRSR:
		m.registers[offset] = value &^ (mfpRSRBufferFull | mfpRSROverrunError | mfpRSRParityError | mfpRSRFramingError)
		if value&mfpRSRReceiverEnable == 0 {
			m.serialRxReady = false
			m.serialRxActive = false
			m.serialRxCycles = 0
		} else {
			m.loadNextRS232Byte()
		}
	case mfpTSR:
		m.registers[offset] = value &^ (mfpTSRBufferEmpty | mfpTSRUnderrunError)
	case mfpDDR:
		m.updateGPIPEdges()
		m.registers[offset] = value
	case mfpAER:
		m.updateGPIPEdges()
		m.registers[offset] = value
	case mfpIERA, mfpIERB, mfpIMRA, mfpIMRB:
		m.registers[offset] = value
	case mfpIPRA, mfpIPRB:
		m.registers[offset] &= value
	case mfpISRA, mfpISRB:
		before := m.registers[offset]
		m.registers[offset] &= value
		m.clearInFlightBits(serviceOffsetToChannelBase(offset), before&^m.registers[offset])
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
		m.writeUSARTData(value)
	default:
		m.registers[offset] = value
	}
}

func (m *MFP) configureTimer(index int, control byte) {
	timer := &m.timers[index]
	timer.control = control & 0x0F
	timer.prescaler = timerPrescaler(timer.control)
	timer.enabled = timer.prescaler != 0 || timer.control == 8

	if !timer.enabled {
		timer.counter = 0
		return
	}
	if timer.control == 8 {
		timer.counter = uint64(m.timerReloadValue(timer.dataReg))
		return
	}

	timer.counter = uint64(m.timerReloadValue(timer.dataReg)) * timer.prescaler
}

func (m *MFP) reloadTimer(index int) {
	timer := &m.timers[index]
	if !timer.enabled {
		return
	}
	if timer.control == 8 {
		timer.counter = uint64(m.timerReloadValue(timer.dataReg))
		return
	}
	if timer.prescaler == 0 {
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
	if timer.control == 8 {
		if !timer.enabled || timer.counter >= 256 {
			return m.registers[timer.dataReg]
		}
		return byte(timer.counter)
	}
	if !timer.enabled || timer.prescaler == 0 || timer.counter == 0 {
		return m.registers[timer.dataReg]
	}

	ticks := (timer.counter + timer.prescaler - 1) / timer.prescaler
	if ticks >= 256 {
		return 0
	}
	return byte(ticks)
}

func (m *MFP) configureEventCountTiming() {
	clockHz := uint64(mfpDefaultClockHz)
	frameHz := uint64(mfpDefaultFrameHz)
	if m.cfg != nil {
		if m.cfg.ClockHz != 0 {
			clockHz = m.cfg.ClockHz
		}
		if m.cfg.FrameHz != 0 {
			frameHz = m.cfg.FrameHz
		}
	}
	m.eventCountFrameCycles = clockHz / frameHz
	if m.eventCountFrameCycles == 0 {
		m.eventCountFrameCycles = 1
	}
	if frameHz >= 55 {
		m.eventCountScanlines = mfpNTSCScanlines
	} else {
		m.eventCountScanlines = mfpPALScanlines
	}
	m.eventCountActiveLines = mfpActiveVideoLines
	if m.eventCountActiveLines > m.eventCountScanlines {
		m.eventCountActiveLines = m.eventCountScanlines
	}
}

func (m *MFP) advanceEventCountTimers(cycles uint64) {
	if m.eventCountFrameCycles == 0 || m.eventCountScanlines == 0 {
		return
	}
	for {
		next := m.eventCountNextLineCycle()
		if m.eventCountCycle < next {
			step := next - m.eventCountCycle
			if step > cycles {
				m.eventCountCycle += cycles
				return
			}
			m.eventCountCycle += step
			cycles -= step
		}

		if m.eventCountNextLine >= m.eventCountScanlines {
			m.eventCountCycle -= m.eventCountFrameCycles
			m.eventCountNextLine = 1
			if cycles == 0 {
				return
			}
			continue
		}

		if m.eventCountNextLine <= m.eventCountActiveLines {
			m.pulseEventCountTimers()
		}
		m.eventCountNextLine++
		if cycles == 0 {
			return
		}
	}
}

func (m *MFP) pulseEventCountTimers() {
	for i := range m.timers {
		timer := &m.timers[i]
		if !timer.enabled || timer.control != 8 {
			continue
		}
		if timer.counter > 1 {
			timer.counter--
			continue
		}
		timer.counter = uint64(m.timerReloadValue(timer.dataReg))
		m.raiseChannel(timer.channel)
	}
}

func (m *MFP) nextEventCountCycles() (uint64, bool) {
	if !m.hasEnabledEventCountTimer() || m.eventCountFrameCycles == 0 || m.eventCountScanlines == 0 {
		return 0, false
	}
	if m.eventCountNextLine <= m.eventCountActiveLines {
		next := m.eventCountNextLineCycle()
		if m.eventCountCycle >= next {
			return 1, true
		}
		return next - m.eventCountCycle, true
	}

	firstLine := m.eventCountFrameCycles / m.eventCountScanlines
	if firstLine == 0 {
		firstLine = 1
	}
	cyclesToFrameEnd := m.eventCountFrameCycles - m.eventCountCycle
	if m.eventCountCycle >= m.eventCountFrameCycles {
		cyclesToFrameEnd = 1
	}
	return cyclesToFrameEnd + firstLine, true
}

func (m *MFP) hasEnabledEventCountTimer() bool {
	for i := range m.timers {
		timer := &m.timers[i]
		if timer.enabled && timer.control == 8 {
			return true
		}
	}
	return false
}

func (m *MFP) eventCountNextLineCycle() uint64 {
	if m.eventCountNextLine == 0 {
		m.eventCountNextLine = 1
	}
	cycle := m.eventCountNextLine * m.eventCountFrameCycles / m.eventCountScanlines
	if cycle == 0 {
		return 1
	}
	return cycle
}

func (m *MFP) raiseChannel(channel int) {
	m.setRegisterBit(pendingRegisterForChannel(channel), channelBit(channel))
}

func (m *MFP) SetACIAInterrupt(asserted bool) {
	if asserted == m.aciaIRQActive {
		return
	}
	m.aciaIRQActive = asserted
	m.updateGPIPEdges()
}

func (m *MFP) SetPrinterBusy(busy bool) {
	if busy == m.printerBusy {
		return
	}
	m.printerBusy = busy
	m.updateGPIPEdges()
}

func (m *MFP) SetFDCInterrupt(asserted bool) {
	if asserted == m.fdcIRQActive {
		return
	}
	m.fdcIRQActive = asserted
	m.updateGPIPEdges()
}

func (m *MFP) AttachICDRTC(rtc *ICDRTC) {
	m.rtc = rtc
}

func (m *MFP) AttachRS232(rs232 *RS232) {
	m.rs232 = rs232
	m.loadNextRS232Byte()
}

func (m *MFP) PushRS232Input(data []byte) {
	if m.rs232 == nil {
		return
	}
	m.rs232.PushInput(data)
	m.loadNextRS232Byte()
}

func (m *MFP) receiverStatus() byte {
	if m.serialRxReady {
		return m.registers[mfpRSR] | mfpRSRBufferFull
	}
	return m.registers[mfpRSR] &^ mfpRSRBufferFull
}

func (m *MFP) transmitterStatus() byte {
	if m.serialTxActive {
		return m.registers[mfpTSR] &^ mfpTSRBufferEmpty
	}
	return m.registers[mfpTSR] | mfpTSRBufferEmpty
}

func (m *MFP) readUSARTData() byte {
	if !m.serialRxReady {
		return m.serialRx
	}

	value := m.serialRx
	m.serialRxReady = false
	m.loadNextRS232Byte()
	return value
}

func (m *MFP) writeUSARTData(value byte) {
	m.registers[mfpUDR] = value
	if m.usartTimingEnabled() {
		if m.serialTxActive {
			m.registers[mfpTSR] |= mfpTSRUnderrunError
		}
		m.serialTxByte = value
		m.serialTxCycles = m.usartFrameCycles()
		if m.serialTxCycles == 0 {
			m.serialTxCycles = 1
		}
		m.serialTxActive = true
		m.registers[mfpTSR] &^= mfpTSRBufferEmpty
		return
	}

	m.registers[mfpTSR] |= mfpTSRBufferEmpty
	if m.rs232 != nil {
		m.rs232.WriteOutput(value)
	}
	m.raiseChannel(mfpUSARTTransmitBufferEmptyChannel)
}

func (m *MFP) loadNextRS232Byte() {
	if m.serialRxActive || m.rs232 == nil {
		return
	}
	if m.usartTimingEnabled() {
		if m.registers[mfpRSR]&mfpRSRReceiverEnable == 0 {
			return
		}
		value, ok := m.rs232.PopInput()
		if !ok {
			return
		}
		m.serialRxByte = value
		m.serialRxCycles = m.usartFrameCycles()
		if m.serialRxCycles == 0 {
			m.serialRxCycles = 1
		}
		m.serialRxActive = true
		return
	}
	if m.serialRxReady {
		return
	}
	value, ok := m.rs232.PopInput()
	if !ok {
		return
	}
	m.serialRx = value
	m.serialRxReady = true
	m.raiseChannel(mfpUSARTReceiveBufferFullChannel)
}

func (m *MFP) advanceUSART(cycles uint64) {
	if cycles == 0 {
		return
	}
	if m.serialRxActive {
		if cycles >= m.serialRxCycles {
			m.serialRxActive = false
			m.serialRxCycles = 0
			if m.serialRxReady {
				m.registers[mfpRSR] |= mfpRSROverrunError
			}
			m.serialRx = m.serialRxByte
			m.serialRxReady = true
			m.raiseChannel(mfpUSARTReceiveBufferFullChannel)
		} else {
			m.serialRxCycles -= cycles
		}
	}
	if m.serialTxActive {
		if cycles >= m.serialTxCycles {
			m.serialTxActive = false
			m.serialTxCycles = 0
			if m.rs232 != nil {
				m.rs232.WriteOutput(m.serialTxByte)
			}
			m.registers[mfpTSR] |= mfpTSRBufferEmpty
			m.raiseChannel(mfpUSARTTransmitBufferEmptyChannel)
		} else {
			m.serialTxCycles -= cycles
		}
	}
}

func (m *MFP) nextUSARTEventCycles() (uint64, bool) {
	var next uint64
	if m.serialRxActive && m.serialRxCycles != 0 {
		next = m.serialRxCycles
	}
	if m.serialTxActive && m.serialTxCycles != 0 && (next == 0 || m.serialTxCycles < next) {
		next = m.serialTxCycles
	}
	if next == 0 {
		return 0, false
	}
	return next, true
}

func (m *MFP) usartTimingEnabled() bool {
	return m.registers[mfpUCR] != 0 || m.registers[mfpSCR] != 0 || m.timers[3].enabled
}

func (m *MFP) usartFrameCycles() uint64 {
	bitCycles := m.usartBitCycles()
	if bitCycles == 0 {
		return 0
	}
	return bitCycles * uint64(m.usartFrameBits())
}

func (m *MFP) usartBitCycles() uint64 {
	prescaler := m.timers[3].prescaler
	if prescaler == 0 {
		prescaler = 1
	}
	divisor := uint64(m.timerReloadValue(mfpTDDR))
	if divisor == 0 {
		divisor = 256
	}
	// The ST commonly drives the USART from Timer D with a 16x serial clock.
	numerator := uint64(m.clockHz()) * prescaler * divisor * 16
	cycles := numerator / mfpTimerInputHz
	if numerator%mfpTimerInputHz != 0 {
		cycles++
	}
	if cycles == 0 {
		return 1
	}
	return cycles
}

func (m *MFP) usartFrameBits() int {
	// Model the common async format fields from UCR: start bit, data bits,
	// optional parity, and stop bits. A zero UCR keeps the historical 8N1 path.
	ucr := m.registers[mfpUCR]
	dataBits := 8
	switch (ucr >> 5) & 0x03 {
	case 1:
		dataBits = 7
	case 2:
		dataBits = 6
	case 3:
		dataBits = 5
	}
	parityBits := 0
	if ucr&0x04 != 0 {
		parityBits = 1
	}
	stopBits := 1
	if ucr&0x03 == 0x03 {
		stopBits = 2
	}
	return 1 + dataBits + parityBits + stopBits
}

func (m *MFP) clockHz() uint64 {
	if m.cfg != nil && m.cfg.ClockHz != 0 {
		return m.cfg.ClockHz
	}
	return mfpDefaultClockHz
}

func (m *MFP) gpipState() byte {
	m.updateGPIPEdges()

	latch := m.registers[mfpGPIP]
	ddr := m.registers[mfpDDR]
	inputs := m.gpipInput
	return (latch & ddr) | (inputs &^ ddr)
}

func (m *MFP) gpipInputState() byte {
	value := byte(0xFF)
	if m.printerBusy {
		value |= 0x01
	} else {
		value &^= 0x01
	}
	if m.cfg.ColorMonitor {
		value |= 0x80
	} else {
		value &^= 0x80
	}
	if m.aciaIRQActive {
		value &^= 0x10
	} else {
		value |= 0x10
	}
	if m.fdcIRQActive || (m.rtc != nil && m.rtc.Active()) {
		value &^= 0x20
	} else {
		value |= 0x20
	}
	return value
}

func (m *MFP) updateGPIPEdges() {
	next := m.gpipInputState()
	changed := m.gpipInput ^ next
	if changed == 0 {
		return
	}

	inputMask := ^m.registers[mfpDDR]
	rising := changed & next
	falling := changed &^ next
	active := (rising & m.registers[mfpAER]) | (falling &^ m.registers[mfpAER])
	active &= inputMask
	for bit := 0; bit < 8; bit++ {
		if active&(1<<uint(bit)) != 0 {
			m.raiseChannel(gpipInterruptChannel(bit))
		}
	}
	m.gpipInput = next
}

func gpipInterruptChannel(bit int) int {
	switch bit {
	case 4:
		return 6
	case 5:
		return 7
	case 6:
		return 14
	case 7:
		return 15
	default:
		return bit
	}
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
		if m.inFlight[channel] {
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

func serviceOffsetToChannelBase(offset uint32) int {
	if offset == mfpISRA {
		return 8
	}
	return 0
}

func (m *MFP) clearInFlightBits(channelBase int, bits byte) {
	for bit := range 8 {
		mask := byte(1 << uint(bit))
		if bits&mask == 0 {
			continue
		}
		m.inFlight[channelBase+bit] = false
	}
}

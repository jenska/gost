package devices

type PrinterPort struct {
	data       byte
	strobeHigh bool
	strobeSeen bool
	busy       bool
	output     []byte
}

func NewPrinterPort() *PrinterPort {
	return &PrinterPort{strobeHigh: true}
}

func (p *PrinterPort) SetData(data byte) {
	p.data = data
}

func (p *PrinterPort) SetStrobe(high bool) {
	if p.strobeSeen && p.strobeHigh && !high && !p.busy {
		p.output = append(p.output, p.data)
	}
	p.strobeHigh = high
	p.strobeSeen = true
}

func (p *PrinterPort) SetBusy(busy bool) {
	p.busy = busy
}

func (p *PrinterPort) Busy() bool {
	return p.busy
}

func (p *PrinterPort) Output() []byte {
	if len(p.output) == 0 {
		return nil
	}
	return append([]byte(nil), p.output...)
}

func (p *PrinterPort) ClearOutput() {
	p.output = p.output[:0]
}

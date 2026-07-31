package device

import (
	"io"
	"time"

	"go.bug.st/serial"
)

// Port is the serial I/O surface the driver needs. go.bug.st/serial.Port
// satisfies it; tests use an in-memory pipe.
type Port interface {
	io.ReadWriteCloser
	Drain() error
	ResetInputBuffer() error
	SetReadTimeout(timeout time.Duration) error
}

// openSerialFn opens a serial port. It is a variable so tests can inject an
// in-memory pipe.
var openSerialFn = OpenSerial

// OpenSerial opens a serial port in the GT-511C3's fixed frame format
// (8 data bits, 1 stop bit, no parity).
func OpenSerial(portName string, baud int) (Port, error) {
	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	p, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}
	// Short read timeout keeps the readN deadline loop responsive.
	if err := p.SetReadTimeout(50 * time.Millisecond); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

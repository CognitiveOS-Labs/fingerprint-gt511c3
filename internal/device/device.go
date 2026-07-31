// Package device implements a GT-511C3 fingerprint scanner driver. The wire
// protocol is described in the GT-511C3 datasheet; every command and data
// transfer here follows the packet structures in "Protocol: Packet Structure".
package device

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/protocol"
)

// Default baud rates. The device boots at 9600 baud (datasheet §5.5) but keeps
// the last configured baud across power cycles, so Connect scans the list.
var scanBauds = []int{9600, 115200, 57600, 38400, 19200}

// DefaultWaitTimeout is how long Enroll/WaitFinger waits for a finger.
const DefaultWaitTimeout = 10 * time.Second

// ErrNotConnected is returned when a command is issued before Connect.
var ErrNotConnected = errors.New("device: not connected")

// TimeoutError reports that an operation did not complete in time.
type TimeoutError struct {
	Op string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("device: %s timed out", e.Op)
}

// Options tune device behaviour.
type Options struct {
	DeviceID      uint16
	CmdTimeout    time.Duration
	UploadTimeout time.Duration
	DataTimeout   time.Duration
	ImageTimeout  time.Duration
	ReadPoll      time.Duration
}

// Device drives a GT-511C3 over a serial port. All methods are safe for
// concurrent use; multi-command sequences hold the internal lock for the whole
// sequence.
type Device struct {
	mu       sync.Mutex
	port     Port
	portName string
	baud     int
	info     *protocol.DevInfo

	devID         uint16
	cmdTimeout    time.Duration
	uploadTimeout time.Duration
	dataTimeout   time.Duration
	imageTimeout  time.Duration
	readPoll      time.Duration
}

// New returns a Device with default timeouts.
func New(opts *Options) *Device {
	d := &Device{
		devID: protocol.DefaultDeviceID,
	}
	if opts != nil {
		if opts.DeviceID != 0 {
			d.devID = opts.DeviceID
		}
		if opts.CmdTimeout > 0 {
			d.cmdTimeout = opts.CmdTimeout
		}
		if opts.UploadTimeout > 0 {
			d.uploadTimeout = opts.UploadTimeout
		}
		if opts.DataTimeout > 0 {
			d.dataTimeout = opts.DataTimeout
		}
		if opts.ImageTimeout > 0 {
			d.imageTimeout = opts.ImageTimeout
		}
		if opts.ReadPoll > 0 {
			d.readPoll = opts.ReadPoll
		}
	}
	if d.cmdTimeout == 0 {
		d.cmdTimeout = 2 * time.Second
	}
	if d.uploadTimeout == 0 {
		d.uploadTimeout = 2 * time.Second
	}
	if d.dataTimeout == 0 {
		d.dataTimeout = 10 * time.Second
	}
	if d.imageTimeout == 0 {
		d.imageTimeout = 30 * time.Second
	}
	if d.readPoll == 0 {
		d.readPoll = 2 * time.Millisecond
	}
	return d
}

// Connected reports whether the device is connected and initialized.
func (d *Device) Connected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.port != nil
}

// PortName returns the serial port the device is connected through.
func (d *Device) PortName() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.portName
}

// Baud returns the current connection baud rate.
func (d *Device) Baud() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.baud
}

// Info returns the device static info from Open, or nil when unavailable.
func (d *Device) Info() *protocol.DevInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.info
}

// Connect opens the serial port and initializes the device with Open
// (datasheet §5.1). If baud is non-zero it is tried first; otherwise (and as a
// fallback) the well-known baud rates are scanned so devices left at a
// non-default baud are still reachable.
func (d *Device) Connect(portName string, baud int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.port != nil {
		return errors.New("device: already connected")
	}

	candidates := baudCandidates(baud)
	var lastErr error
	for _, b := range candidates {
		p, err := openSerialFn(portName, b)
		if err != nil {
			lastErr = err
			continue
		}
		d.port = p
		d.portName = portName
		d.baud = b

		if _, err := d.sendCommandLocked(protocol.CmdOpen, 1); err != nil {
			lastErr = err
			p.Close()
			d.port = nil
			continue
		}
		frame, err := d.readNLocked(protocol.DataFrameHeaderSize+protocol.DevInfoSize, d.dataTimeout)
		if err != nil {
			lastErr = err
			p.Close()
			d.port = nil
			continue
		}
		payload, err := protocol.UnmarshalDataFrame(frame, d.devID)
		if err != nil {
			lastErr = err
			p.Close()
			d.port = nil
			continue
		}
		info, err := protocol.ParseDevInfo(payload)
		if err != nil {
			lastErr = err
			p.Close()
			d.port = nil
			continue
		}
		d.info = info
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("device: no candidate baud rate matched on %s", portName)
	}
	return lastErr
}

// Disconnect closes the serial port without a terminating handshake.
func (d *Device) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closeLocked()
}

// Close terminates the session (Close command, datasheet §5.2) and closes the
// port.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.port != nil {
		_, _ = d.sendCommandLocked(protocol.CmdClose, 0)
	}
	return d.closeLocked()
}

func (d *Device) closeLocked() error {
	var err error
	if d.port != nil {
		err = d.port.Close()
	}
	d.port = nil
	d.portName = ""
	d.baud = 0
	d.info = nil
	return err
}

// baudCandidates orders the baud scan, honouring an explicit preference first.
func baudCandidates(baud int) []int {
	var out []int
	if baud > 0 {
		out = append(out, baud)
	}
	for _, b := range scanBauds {
		if b != baud {
			out = append(out, b)
		}
	}
	return out
}

// sendCommandLocked writes a command packet and reads the 12-byte response.
// It returns the response parameter (output parameter on ACK, error code on
// NACK).
func (d *Device) sendCommandLocked(cmd uint16, param uint32) (uint32, error) {
	if d.port == nil {
		return 0, ErrNotConnected
	}
	if err := d.port.ResetInputBuffer(); err != nil {
		return 0, err
	}
	pkt := protocol.CommandPacket{DeviceID: d.devID, Param: param, Command: cmd}
	raw := pkt.Marshal()
	if _, err := d.port.Write(raw[:]); err != nil {
		return 0, err
	}
	if err := d.port.Drain(); err != nil {
		return 0, err
	}
	respRaw, err := d.readNLocked(protocol.PacketSize, d.cmdTimeout)
	if err != nil {
		return 0, err
	}
	resp, err := protocol.ParseResponse(respRaw, d.devID)
	if err != nil {
		return 0, err
	}
	if resp.IsNack() {
		return resp.Param, &protocol.NackError{Code: uint16(resp.Param)}
	}
	return resp.Param, nil
}

// receiveDataLocked reads one complete data frame and returns its payload.
func (d *Device) receiveDataLocked(size int, timeout time.Duration) ([]byte, error) {
	if d.port == nil {
		return nil, ErrNotConnected
	}
	frame, err := d.readNLocked(protocol.DataFrameHeaderSize+size, timeout)
	if err != nil {
		return nil, err
	}
	payload, err := protocol.UnmarshalDataFrame(frame, d.devID)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// sendDataLocked uploads one data frame and reads the follow-up response.
func (d *Device) sendDataLocked(payload []byte, timeout time.Duration) (uint32, error) {
	if d.port == nil {
		return 0, ErrNotConnected
	}
	frame := protocol.MarshalDataFrame(d.devID, payload)
	if _, err := d.port.Write(frame); err != nil {
		return 0, err
	}
	if err := d.port.Drain(); err != nil {
		return 0, err
	}
	respRaw, err := d.readNLocked(protocol.PacketSize, timeout)
	if err != nil {
		return 0, err
	}
	resp, err := protocol.ParseResponse(respRaw, d.devID)
	if err != nil {
		return 0, err
	}
	if resp.IsNack() {
		return resp.Param, &protocol.NackError{Code: uint16(resp.Param)}
	}
	return resp.Param, nil
}

// readNLocked reads exactly n bytes, returning a TimeoutError past deadline.
func (d *Device) readNLocked(n int, timeout time.Duration) ([]byte, error) {
	if d.port == nil {
		return nil, ErrNotConnected
	}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 0, n)
	tmp := make([]byte, 1024)
	for len(buf) < n {
		max := n - len(buf)
		if max > len(tmp) {
			max = len(tmp)
		}
		m, err := d.port.Read(tmp[:max])
		if err != nil {
			return nil, err
		}
		if m > 0 {
			buf = append(buf, tmp[:m]...)
			continue
		}
		if time.Now().After(deadline) {
			return nil, &TimeoutError{Op: "read"}
		}
		time.Sleep(d.readPoll)
	}
	return buf, nil
}

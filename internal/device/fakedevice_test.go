package device

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/protocol"
)

// connPort adapts a net.Conn to the Port interface. Timeouts and buffer
// resets are no-ops on the in-memory pipe.
type connPort struct {
	net.Conn
}

func (connPort) Drain() error                       { return nil }
func (connPort) ResetInputBuffer() error            { return nil }
func (connPort) SetReadTimeout(time.Duration) error { return nil }

// cmdRec records a command observed by the fake device.
type cmdRec struct {
	cmd   uint16
	param uint32
}

// fakeDevice is an in-memory GT-511C3 responder driven by the datasheet
// semantics. It validates the driver's command sequences and data transfers.
type fakeDevice struct {
	conn    net.Conn
	mu      sync.Mutex
	cmds    []cmdRec
	uploads [][]byte
	pressed bool

	// phase drives IsPressFinger during enrollment (see run).
	phase    string
	enrolled map[uint16]bool
	template map[uint16][]byte
	count    uint32
	enrollID uint16
}

func newFakeDevice(conn net.Conn) *fakeDevice {
	f := &fakeDevice{
		conn:     conn,
		enrolled: map[uint16]bool{},
		template: map[uint16][]byte{},
	}
	f.setPhase("wait-press")
	return f
}

func (f *fakeDevice) logCmd(cmd uint16, param uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds = append(f.cmds, cmdRec{cmd, param})
}

func (f *fakeDevice) commands() []cmdRec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]cmdRec(nil), f.cmds...)
}

func (f *fakeDevice) uploaded() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.uploads...)
}

func (f *fakeDevice) setPressed(b bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pressed = b
}

func (f *fakeDevice) readN(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(f.conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (f *fakeDevice) write(b []byte) error {
	_, err := f.conn.Write(b)
	return err
}

func (f *fakeDevice) ack(param uint32) error {
	var p [protocol.PacketSize]byte
	p[0], p[1] = protocol.CommandStartCode1, protocol.CommandStartCode2
	binary.LittleEndian.PutUint16(p[2:], protocol.DefaultDeviceID)
	binary.LittleEndian.PutUint32(p[4:], param)
	binary.LittleEndian.PutUint16(p[8:], protocol.Ack)
	binary.LittleEndian.PutUint16(p[10:], protocol.Checksum(p[:10]))
	return f.write(p[:])
}

func (f *fakeDevice) nack(code uint16) error {
	var p [protocol.PacketSize]byte
	p[0], p[1] = protocol.CommandStartCode1, protocol.CommandStartCode2
	binary.LittleEndian.PutUint16(p[2:], protocol.DefaultDeviceID)
	binary.LittleEndian.PutUint32(p[4:], uint32(code))
	binary.LittleEndian.PutUint16(p[8:], protocol.Nack)
	binary.LittleEndian.PutUint16(p[10:], protocol.Checksum(p[:10]))
	return f.write(p[:])
}

func (f *fakeDevice) sendDataFrame(payload []byte) error {
	return f.write(protocol.MarshalDataFrame(protocol.DefaultDeviceID, payload))
}

// setPhase flips the finger-present simulation; captures only succeed when
// pressed, and IsPressFinger reports accordingly.
func (f *fakeDevice) setPhase(phase string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phase = phase
	if phase == "wait-press" {
		f.pressed = true
	} else {
		f.pressed = false
	}
}

// run serves the device until the connection closes.
func (f *fakeDevice) run(t *testing.T) {
	t.Helper()
	for {
		raw, err := f.readN(protocol.PacketSize)
		if err != nil {
			return
		}
		cmd := binary.LittleEndian.Uint16(raw[8:])
		param := binary.LittleEndian.Uint32(raw[4:])
		f.logCmd(cmd, param)

		releasePhase := func() {
			f.mu.Lock()
			if f.phase == "wait-release" {
				f.phase = "wait-press"
				f.pressed = true
			}
			f.mu.Unlock()
		}

		switch cmd {
		case protocol.CmdOpen:
			if err := f.ack(0); err != nil {
				return
			}
			info := make([]byte, protocol.DevInfoSize)
			binary.LittleEndian.PutUint32(info[0:], 0x01020304)
			binary.LittleEndian.PutUint32(info[4:], 0x00100000)
			copy(info[8:], "GT-511C3-UNIT01")
			if err := f.sendDataFrame(info); err != nil {
				return
			}
		case protocol.CmdClose, protocol.CmdCmosLed, protocol.CmdDeleteAll:
			if protocol.CmdDeleteAll == cmd {
				f.mu.Lock()
				f.enrolled = map[uint16]bool{}
				f.template = map[uint16][]byte{}
				f.count = 0
				f.mu.Unlock()
			}
			if err := f.ack(0); err != nil {
				return
			}
		case protocol.CmdDeleteID:
			f.mu.Lock()
			id := uint16(param)
			if f.enrolled[id] {
				delete(f.enrolled, id)
				delete(f.template, id)
				if f.count > 0 {
					f.count--
				}
			}
			f.mu.Unlock()
			if err := f.ack(0); err != nil {
				return
			}
		case protocol.CmdGetEnrollCount:
			f.mu.Lock()
			count := f.count
			f.mu.Unlock()
			if err := f.ack(count); err != nil {
				return
			}
		case protocol.CmdCheckEnrolled:
			f.mu.Lock()
			used := f.enrolled[uint16(param)]
			f.mu.Unlock()
			if used {
				if err := f.ack(0); err != nil {
					return
				}
			} else {
				if err := f.nack(protocol.NackIsNotUsed); err != nil {
					return
				}
			}
		case protocol.CmdEnrollStart:
			f.mu.Lock()
			used := f.enrolled[uint16(param)]
			f.mu.Unlock()
			if used {
				if err := f.nack(protocol.NackIsAlreadyUsed); err != nil {
					return
				}
			} else {
				f.mu.Lock()
				f.enrollID = uint16(param)
				f.mu.Unlock()
				if err := f.ack(0); err != nil {
					return
				}
			}
		case protocol.CmdEnroll1, protocol.CmdEnroll2:
			if err := f.ack(0); err != nil {
				return
			}
			f.setPhase("wait-release")
		case protocol.CmdEnroll3:
			f.mu.Lock()
			f.phase = "wait-release"
			f.pressed = false
			id := f.enrollID
			f.enrolled[id] = true
			f.template[id] = make([]byte, protocol.TemplateSize)
			f.count++
			f.mu.Unlock()
			if err := f.ack(0); err != nil {
				return
			}
		case protocol.CmdIsPressFinger:
			f.mu.Lock()
			pressed := f.pressed
			phase := f.phase
			f.mu.Unlock()
			if pressed {
				if err := f.ack(0); err != nil {
					return
				}
			} else {
				if err := f.ack(1); err != nil {
					return
				}
			}
			if phase == "wait-release" {
				releasePhase()
			}
		case protocol.CmdCaptureFinger:
			f.mu.Lock()
			pressed := f.pressed
			f.mu.Unlock()
			if pressed {
				if err := f.ack(0); err != nil {
					return
				}
			} else {
				if err := f.nack(protocol.NackFingerNotPressed); err != nil {
					return
				}
			}
		case protocol.CmdVerify:
			f.mu.Lock()
			used := f.enrolled[uint16(param)]
			pressed := f.pressed
			f.mu.Unlock()
			if used && pressed {
				if err := f.ack(0); err != nil {
					return
				}
			} else if !used {
				if err := f.nack(protocol.NackIsNotUsed); err != nil {
					return
				}
			} else {
				if err := f.nack(protocol.NackVerifyFailed); err != nil {
					return
				}
			}
		case protocol.CmdIdentify:
			f.mu.Lock()
			pressed := f.pressed
			first := uint16(0)
			found := false
			for id := range f.enrolled {
				if !found || id < first {
					first = id
					found = true
				}
			}
			f.mu.Unlock()
			if pressed && found {
				if err := f.ack(uint32(first)); err != nil {
					return
				}
			} else if err := f.nack(protocol.NackIdentifyFailed); err != nil {
				return
			}
		case protocol.CmdGetTemplate:
			f.mu.Lock()
			tmpl, ok := f.template[uint16(param)]
			f.mu.Unlock()
			if !ok {
				if err := f.nack(protocol.NackIsNotUsed); err != nil {
					return
				}
			} else if err := f.ack(0); err != nil {
				return
			} else if err := f.sendDataFrame(tmpl); err != nil {
				return
			}
		case protocol.CmdSetTemplate:
			if err := f.ack(0); err != nil {
				return
			}
			payload, err := f.readN(protocol.DataFrameHeaderSize + protocol.TemplateSize)
			if err != nil {
				return
			}
			tmpl, err := protocol.UnmarshalDataFrame(payload, protocol.DefaultDeviceID)
			if err != nil {
				t.Errorf("fake: bad upload frame: %v", err)
				return
			}
			f.mu.Lock()
			f.uploads = append(f.uploads, tmpl)
			f.template[uint16(param&0xFFFF)] = tmpl
			f.enrolled[uint16(param&0xFFFF)] = true
			f.count++
			f.mu.Unlock()
			if err := f.ack(0); err != nil {
				return
			}
		case protocol.CmdMakeTemplate, protocol.CmdGetImage, protocol.CmdGetRawImage:
			if err := f.ack(0); err != nil {
				return
			}
			size := protocol.TemplateSize
			switch cmd {
			case protocol.CmdGetImage:
				size = protocol.ImageSize
			case protocol.CmdGetRawImage:
				size = protocol.RawImageSize
			}
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i)
			}
			if err := f.sendDataFrame(payload); err != nil {
				return
			}
		case protocol.CmdVerifyTemplate:
			if err := f.ack(0); err != nil {
				return
			}
			payload, err := f.readN(protocol.DataFrameHeaderSize + protocol.TemplateSize)
			if err != nil {
				return
			}
			tmpl, err := protocol.UnmarshalDataFrame(payload, protocol.DefaultDeviceID)
			if err != nil {
				t.Errorf("fake: bad upload frame: %v", err)
				return
			}
			f.mu.Lock()
			f.uploads = append(f.uploads, tmpl)
			f.mu.Unlock()
			if err := f.ack(0); err != nil {
				return
			}
		case protocol.CmdIdentifyTemplate:
			if err := f.ack(0); err != nil {
				return
			}
			payload, err := f.readN(protocol.DataFrameHeaderSize + protocol.TemplateSize)
			if err != nil {
				return
			}
			tmpl, err := protocol.UnmarshalDataFrame(payload, protocol.DefaultDeviceID)
			if err != nil {
				t.Errorf("fake: bad upload frame: %v", err)
				return
			}
			f.mu.Lock()
			f.uploads = append(f.uploads, tmpl)
			matched := uint32(7)
			f.mu.Unlock()
			if err := f.ack(matched); err != nil {
				return
			}
		case protocol.CmdChangeBaudrate:
			if err := f.ack(0); err != nil {
				return
			}
		default:
			t.Errorf("fake: unexpected command 0x%02x", cmd)
			return
		}
	}
}

// newPair wires a Device to a fakeDevice over an in-memory pipe.
func newPair(t *testing.T) (*Device, *fakeDevice) {
	t.Helper()
	server, client := net.Pipe()
	d := New(nil)
	d.mu.Lock()
	d.port = &connPort{Conn: client}
	d.portName = "pipe"
	d.baud = 9600
	d.mu.Unlock()
	f := newFakeDevice(server)
	go f.run(t)
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return d, f
}

// hasCmd reports whether the fake device saw the given command.
func (f *fakeDevice) hasCmd(cmd uint16) bool {
	for _, c := range f.commands() {
		if c.cmd == cmd {
			return true
		}
	}
	return false
}

// netPipePair returns an in-memory pipe.
func netPipePair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	return net.Pipe()
}

// hasCmdInOrder reports whether want appears as a subsequence of got.
func hasCmdInOrder(got, want []uint16) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

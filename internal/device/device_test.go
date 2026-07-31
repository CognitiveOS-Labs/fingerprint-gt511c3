package device

import (
	"bytes"
	"testing"
	"time"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/protocol"
)

func TestSetLED(t *testing.T) {
	d, f := newPair(t)
	if err := d.SetLED(true); err != nil {
		t.Fatalf("SetLED(true): %v", err)
	}
	if err := d.SetLED(false); err != nil {
		t.Fatalf("SetLED(false): %v", err)
	}
	cmds := f.commands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0].cmd != protocol.CmdCmosLed || cmds[0].param != 1 {
		t.Fatalf("cmd1 = %+v", cmds[0])
	}
	if cmds[1].cmd != protocol.CmdCmosLed || cmds[1].param != 0 {
		t.Fatalf("cmd2 = %+v", cmds[1])
	}
}

func TestGetEnrollCountAndCheckEnrolled(t *testing.T) {
	d, f := newPair(t)
	f.setPressed(true)

	count, err := d.GetEnrollCount()
	if err != nil {
		t.Fatalf("GetEnrollCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}

	enrolled, err := d.CheckEnrolled(3)
	if err != nil {
		t.Fatalf("CheckEnrolled: %v", err)
	}
	if enrolled {
		t.Fatalf("id 3 should not be enrolled")
	}
}

func TestVerifyMatchAndNoMatch(t *testing.T) {
	d, f := newPair(t)
	f.mu.Lock()
	f.enrolled[9] = true
	f.mu.Unlock()

	f.setPressed(true)
	matched, err := d.Verify(9)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !matched {
		t.Fatalf("expected match for id 9")
	}

	f.setPressed(false)
	matched, err = d.Verify(9)
	if err != nil {
		t.Fatalf("Verify no finger: %v", err)
	}
	if matched {
		t.Fatalf("expected no match without finger")
	}
}

func TestIdentify(t *testing.T) {
	d, f := newPair(t)
	f.mu.Lock()
	f.enrolled[42] = true
	f.mu.Unlock()

	f.setPressed(true)
	id, matched, err := d.Identify()
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if !matched || id != 42 {
		t.Fatalf("identify = (%d, %v), want (42, true)", id, matched)
	}

	f.setPressed(false)
	_, matched, err = d.Identify()
	if err != nil {
		t.Fatalf("Identify no match: %v", err)
	}
	if matched {
		t.Fatalf("expected no match")
	}
}

func TestGetTemplate(t *testing.T) {
	d, f := newPair(t)
	f.mu.Lock()
	f.template[5] = bytes.Repeat([]byte{0xEE}, protocol.TemplateSize)
	f.enrolled[5] = true
	f.mu.Unlock()

	tmpl, err := d.GetTemplate(5)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if len(tmpl) != protocol.TemplateSize {
		t.Fatalf("template len = %d, want %d", len(tmpl), protocol.TemplateSize)
	}
	if !bytes.Equal(tmpl, bytes.Repeat([]byte{0xEE}, protocol.TemplateSize)) {
		t.Fatalf("template mismatch")
	}

	if _, err := d.GetTemplate(6); err == nil {
		t.Fatalf("expected error for unenrolled id 6")
	} else {
		var nack *protocol.NackError
		if !asNack(err, &nack) || nack.Code != protocol.NackIsNotUsed {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestSetTemplateUpload(t *testing.T) {
	d, f := newPair(t)
	tmpl := bytes.Repeat([]byte{0x77}, protocol.TemplateSize)
	if err := d.SetTemplate(11, tmpl, true); err != nil {
		t.Fatalf("SetTemplate: %v", err)
	}
	up := f.uploaded()
	if len(up) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(up))
	}
	if !bytes.Equal(up[0], tmpl) {
		t.Fatalf("uploaded template mismatch")
	}

	enrolled, err := d.CheckEnrolled(11)
	if err != nil {
		t.Fatalf("CheckEnrolled: %v", err)
	}
	if !enrolled {
		t.Fatalf("id 11 should be enrolled after set")
	}
}

func TestVerifyTemplateAndIdentifyTemplate(t *testing.T) {
	d, _ := newPair(t)
	tmpl := bytes.Repeat([]byte{0x88}, protocol.TemplateSize)

	matched, err := d.VerifyTemplate(3, tmpl)
	if err != nil {
		t.Fatalf("VerifyTemplate: %v", err)
	}
	if !matched {
		t.Fatalf("expected match")
	}

	id, matched, err := d.IdentifyTemplate(tmpl)
	if err != nil {
		t.Fatalf("IdentifyTemplate: %v", err)
	}
	if !matched || id != 7 {
		t.Fatalf("identifyTemplate = (%d, %v), want (7, true)", id, matched)
	}

	if _, err := d.VerifyTemplate(3, tmpl[:100]); err == nil {
		t.Fatalf("expected error for short template")
	}
}

func TestGetImageAndRawImage(t *testing.T) {
	d, _ := newPair(t)

	img, err := d.GetImage()
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if len(img) != protocol.ImageSize {
		t.Fatalf("image len = %d, want %d", len(img), protocol.ImageSize)
	}
	if img[0] != 0 || img[1000] != byte(1000&0xFF) {
		t.Fatalf("image content mismatch")
	}

	raw, err := d.GetRawImage()
	if err != nil {
		t.Fatalf("GetRawImage: %v", err)
	}
	if len(raw) != protocol.RawImageSize {
		t.Fatalf("raw image len = %d, want %d", len(raw), protocol.RawImageSize)
	}
}

func TestMakeTemplate(t *testing.T) {
	d, _ := newPair(t)
	tmpl, err := d.MakeTemplate()
	if err != nil {
		t.Fatalf("MakeTemplate: %v", err)
	}
	if len(tmpl) != protocol.TemplateSize {
		t.Fatalf("template len = %d, want %d", len(tmpl), protocol.TemplateSize)
	}
}

func TestCaptureFingerNoFinger(t *testing.T) {
	d, f := newPair(t)
	f.setPressed(false)
	if err := d.CaptureFinger(true); err == nil {
		t.Fatalf("expected NACK for capture without finger")
	} else {
		var nack *protocol.NackError
		if !asNack(err, &nack) || nack.Code != protocol.NackFingerNotPressed {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestWaitFingerTimeout(t *testing.T) {
	d, f := newPair(t)
	f.setPressed(false)
	start := time.Now()
	err := d.WaitFinger(200 * time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout")
	}
	if _, ok := err.(*TimeoutError); !ok {
		t.Fatalf("expected TimeoutError, got %T: %v", err, err)
	}
	if time.Since(start) < 150*time.Millisecond {
		t.Fatalf("returned too early")
	}
}

func TestWaitFingerPressed(t *testing.T) {
	d, f := newPair(t)
	f.setPressed(true)
	if err := d.WaitFinger(2 * time.Second); err != nil {
		t.Fatalf("WaitFinger: %v", err)
	}
}

func TestEnrollFlow(t *testing.T) {
	d, f := newPair(t)
	if err := d.Enroll(4); err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	cmds := f.commands()
	sequence := make([]uint16, 0, len(cmds))
	for _, c := range cmds {
		sequence = append(sequence, c.cmd)
	}
	// LED on, wait-press polls, EnrollStart, Capture, Enroll1, wait-release
	// polls, wait-press polls, Capture, Enroll2, wait-release polls,
	// wait-press polls, Capture, Enroll3, LED off.
	if !hasCmdInOrder(sequence, []uint16{protocol.CmdCmosLed, protocol.CmdEnrollStart, protocol.CmdCaptureFinger, protocol.CmdEnroll1, protocol.CmdCaptureFinger, protocol.CmdEnroll2, protocol.CmdCaptureFinger, protocol.CmdEnroll3, protocol.CmdCmosLed}) {
		t.Fatalf("enroll sequence wrong: %v", sequence)
	}
	if err := d.SetLED(true); err != nil {
		t.Fatalf("SetLED after enroll: %v", err)
	}
	enrolled, err := d.CheckEnrolled(4)
	if err != nil {
		t.Fatalf("CheckEnrolled: %v", err)
	}
	if !enrolled {
		t.Fatalf("id 4 should be enrolled")
	}
}

func TestEnrollIDInUse(t *testing.T) {
	d, f := newPair(t)
	f.mu.Lock()
	f.enrolled[4] = true
	f.mu.Unlock()
	f.setPressed(true)

	err := d.Enroll(4)
	if err == nil {
		t.Fatalf("expected error enrolling used id")
	}
	var nack *protocol.NackError
	if !asNack(err, &nack) || nack.Code != protocol.NackIsAlreadyUsed {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteIDAndDeleteAll(t *testing.T) {
	d, f := newPair(t)
	f.mu.Lock()
	f.enrolled[1] = true
	f.mu.Unlock()

	if err := d.DeleteID(1); err != nil {
		t.Fatalf("DeleteID: %v", err)
	}
	if err := d.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if !f.hasCmd(protocol.CmdDeleteID) || !f.hasCmd(protocol.CmdDeleteAll) {
		t.Fatalf("delete commands not sent")
	}
}

func TestNotConnected(t *testing.T) {
	d := New(nil)
	if err := d.SetLED(true); err == nil {
		t.Fatalf("expected ErrNotConnected")
	}
	if d.Connected() {
		t.Fatalf("should not be connected")
	}
}

func TestConnect(t *testing.T) {
	orig := openSerialFn
	defer func() { openSerialFn = orig }()

	server, client := netPipePair(t)
	openSerialFn = func(name string, baud int) (Port, error) {
		return &connPort{Conn: client}, nil
	}

	f := newFakeDevice(server)
	go f.run(t)
	defer server.Close()

	d := New(nil)
	if err := d.Connect("pipe", 0); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer d.Close()

	info := d.Info()
	if info == nil {
		t.Fatalf("no devinfo")
	}
	if info.FirmwareVersion != 0x01020304 {
		t.Fatalf("firmware = %#x", info.FirmwareVersion)
	}
	if d.Baud() != 9600 {
		t.Fatalf("baud = %d, want 9600", d.Baud())
	}
	if !d.Connected() {
		t.Fatalf("should be connected")
	}
}

func TestChangeBaudrate(t *testing.T) {
	orig := openSerialFn
	defer func() { openSerialFn = orig }()

	server, client := netPipePair(t)
	var openedAt []int
	openSerialFn = func(name string, baud int) (Port, error) {
		openedAt = append(openedAt, baud)
		return &connPort{Conn: client}, nil
	}

	f := newFakeDevice(server)
	go f.run(t)
	defer server.Close()

	d := New(nil)
	d.mu.Lock()
	d.port = &connPort{Conn: client}
	d.portName = "pipe"
	d.baud = 9600
	d.mu.Unlock()

	if err := d.ChangeBaudrate(115200); err != nil {
		t.Fatalf("ChangeBaudrate: %v", err)
	}
	if d.Baud() != 115200 {
		t.Fatalf("baud = %d, want 115200", d.Baud())
	}
	if !f.hasCmd(protocol.CmdChangeBaudrate) {
		t.Fatalf("change baudrate command not sent")
	}
}

func asNack(err error, target **protocol.NackError) bool {
	var n *protocol.NackError
	if ok := asErr(err, &n); ok {
		*target = n
		return true
	}
	return false
}

// asErr is a tiny helper avoiding errors.As boilerplate in the helpers above.
func asErr(err error, target **protocol.NackError) bool {
	for e := err; e != nil; {
		if n, ok := e.(*protocol.NackError); ok {
			*target = n
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}

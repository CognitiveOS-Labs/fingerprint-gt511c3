package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// Golden vectors captured from a real GT-511C3 (SparkFun tutorial, "Verifying
// the Checksum Value"). Packets are printed as space-separated hex bytes.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	clean := strings.Join(strings.Fields(s), "")
	b, err := hex.DecodeString(clean)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestChecksumLEDOff(t *testing.T) {
	// 0x55 + 0xAA + 0x01 + 0x00 + 0x00 + 0x00 + 0x00 + 0x00 + 0x12 + 0x00 = 0x112
	if got, want := Checksum(mustHex(t, "55 AA 01 00 00 00 00 00 12 00")), uint16(0x0112); got != want {
		t.Fatalf("checksum = %#04x, want %#04x", got, want)
	}
}

func TestMarshalCommandGolden(t *testing.T) {
	cases := []struct {
		name string
		cmd  CommandPacket
		want string
	}{
		{"open", CommandPacket{DefaultDeviceID, 0, CmdOpen}, "55 AA 01 00 00 00 00 00 01 00 01 01"},
		{"led on", CommandPacket{DefaultDeviceID, 1, CmdCmosLed}, "55 AA 01 00 01 00 00 00 12 00 13 01"},
		{"led off", CommandPacket{DefaultDeviceID, 0, CmdCmosLed}, "55 AA 01 00 00 00 00 00 12 00 12 01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cmd.Marshal()
			if want := mustHex(t, tc.want); !bytes.Equal(got[:], want) {
				t.Fatalf("packet = %x, want %x", got[:], want)
			}
		})
	}
}

func TestParseResponseGolden(t *testing.T) {
	// ACK response seen on the wire for Open/LED commands.
	resp, err := ParseResponse(mustHex(t, "55 AA 01 00 00 00 00 00 30 00 30 01"), DefaultDeviceID)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !resp.IsAck() {
		t.Fatalf("expected ack, got %#04x", resp.Response)
	}
	if resp.Param != 0 {
		t.Fatalf("param = %d, want 0", resp.Param)
	}
}

func TestParseResponseErrors(t *testing.T) {
	valid := mustHex(t, "55 AA 01 00 00 00 00 00 30 00 30 01")

	if _, err := ParseResponse(valid[:10], DefaultDeviceID); err != ErrShortPacket {
		t.Fatalf("short: got %v", err)
	}
	badHeader := append([]byte(nil), valid...)
	badHeader[0] = 0x5A
	if _, err := ParseResponse(badHeader, DefaultDeviceID); err != ErrBadHeader {
		t.Fatalf("bad header: got %v", err)
	}
	badSum := append([]byte(nil), valid...)
	badSum[10] = 0x00
	if _, err := ParseResponse(badSum, DefaultDeviceID); err != ErrBadChecksum {
		t.Fatalf("bad checksum: got %v", err)
	}
	if _, err := ParseResponse(valid, 0x0002); err != ErrBadDeviceID {
		t.Fatalf("bad device id: got %v", err)
	}
}

func TestNackResponse(t *testing.T) {
	// NACK with parameter 0x1004 (ID not used): bytes 4-5 = 04 10.
	raw := []byte{0x55, 0xAA, 0x01, 0x00, 0x04, 0x10, 0x00, 0x00, 0x31, 0x00, 0x00, 0x00}
	raw[10] = byte(Checksum(raw[:10]) & 0xff)
	raw[11] = byte(Checksum(raw[:10]) >> 8)
	resp, err := ParseResponse(raw, DefaultDeviceID)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !resp.IsNack() {
		t.Fatalf("expected nack")
	}
	if resp.Param != NackIsNotUsed {
		t.Fatalf("param = %#04x, want %#04x", resp.Param, NackIsNotUsed)
	}
}

func TestDataFrameRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte{0xAB, 0xCD}, TemplateSize/2)
	frame := MarshalDataFrame(DefaultDeviceID, payload)
	if len(frame) != TemplateSize+6 {
		t.Fatalf("frame len = %d, want %d", len(frame), TemplateSize+6)
	}
	if frame[0] != DataStartCode1 || frame[1] != DataStartCode2 {
		t.Fatalf("bad data start codes")
	}
	got, err := UnmarshalDataFrame(frame, DefaultDeviceID)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch")
	}

	// Corrupt one payload byte; checksum must fail.
	frame[10] ^= 0xff
	if _, err := UnmarshalDataFrame(frame, DefaultDeviceID); err != ErrBadChecksum {
		t.Fatalf("corrupt frame: got %v", err)
	}
}

func TestParseDevInfo(t *testing.T) {
	payload := make([]byte, DevInfoSize)
	payload[0] = 0x01 // firmware 0x00000001
	payload[4] = 0xFF // iso area 0x000000FF
	payload[8] = 'A'
	payload[9] = 'B'
	info, err := ParseDevInfo(payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.FirmwareVersion != 1 || info.IsoAreaMaxSize != 0xFF {
		t.Fatalf("unexpected info: %+v", info)
	}
	if string(info.SerialNumber[:2]) != "AB" {
		t.Fatalf("serial = %x", info.SerialNumber)
	}
}

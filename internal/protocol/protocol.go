// Package protocol implements the GT-511C3 serial wire protocol described in
// the GT-511C3 datasheet ("Protocol: Packet Structure"). All multi-byte
// fields are little-endian.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Start codes and packet framing.
const (
	CommandStartCode1 = 0x55
	CommandStartCode2 = 0xAA
	DataStartCode1    = 0x5A
	DataStartCode2    = 0xA5

	DefaultDeviceID = 0x0001
	PacketSize      = 12 // command and response packets

	// DataFrameHeaderSize is the non-payload size of a data packet: start
	// codes (2) + device id (2) + checksum (2).
	DataFrameHeaderSize = 6
)

// Response codes (response packet byte 8-9).
const (
	Ack  = 0x30
	Nack = 0x31
)

// Payload sizes of data packets, per datasheet command details.
const (
	TemplateSize = 498 // GetTemplate/SetTemplate/MakeTemplate/VerifyTemplate/IdentifyTemplate
	DevInfoSize  = 24  // Open(extra info): FirmwareVersion(4) + IsoAreaMaxSize(4) + SerialNumber(16)

	ImageWidth  = 258
	ImageHeight = 202
	ImageSize   = ImageWidth * ImageHeight // 52116

	RawImageWidth  = 160
	RawImageHeight = 120
	RawImageSize   = RawImageWidth * RawImageHeight // 19200
)

var (
	ErrShortPacket = errors.New("protocol: short packet")
	ErrBadHeader   = errors.New("protocol: invalid start code")
	ErrBadChecksum = errors.New("protocol: checksum mismatch")
	ErrBadDeviceID = errors.New("protocol: unexpected device id")
	ErrBadResponse = errors.New("protocol: invalid response code")
)

// Checksum returns the 16-bit byte-sum of buf (datasheet: "Check Sum (byte
// addition) OFFSET[0]+...+OFFSET[n-1]"), truncated to a word.
func Checksum(buf []byte) uint16 {
	var sum uint16
	for _, b := range buf {
		sum += uint16(b)
	}
	return sum
}

// CommandPacket is the 12-byte host-to-device command.
type CommandPacket struct {
	DeviceID uint16
	Param    uint32
	Command  uint16
}

// Marshal serializes the packet into its 12 bytes.
func (c CommandPacket) Marshal() [PacketSize]byte {
	var p [PacketSize]byte
	p[0] = CommandStartCode1
	p[1] = CommandStartCode2
	binary.LittleEndian.PutUint16(p[2:], c.DeviceID)
	binary.LittleEndian.PutUint32(p[4:], c.Param)
	binary.LittleEndian.PutUint16(p[8:], c.Command)
	binary.LittleEndian.PutUint16(p[10:], Checksum(p[:10]))
	return p
}

// ResponsePacket is the 12-byte device response. For an ACK the Param holds
// the output parameter; for a NACK it holds the error code.
type ResponsePacket struct {
	DeviceID uint16
	Param    uint32
	Response uint16
}

// IsAck reports whether the response is an acknowledgement.
func (r *ResponsePacket) IsAck() bool { return r.Response == Ack }

// IsNack reports whether the response is a non-acknowledgement.
func (r *ResponsePacket) IsNack() bool { return r.Response == Nack }

// ParseResponse validates and parses a raw 12-byte response packet.
func ParseResponse(raw []byte, deviceID uint16) (*ResponsePacket, error) {
	if len(raw) < PacketSize {
		return nil, ErrShortPacket
	}
	if raw[0] != CommandStartCode1 || raw[1] != CommandStartCode2 {
		return nil, ErrBadHeader
	}
	if got, want := binary.LittleEndian.Uint16(raw[2:]), deviceID; want != 0 && got != want {
		return nil, ErrBadDeviceID
	}
	if got, want := binary.LittleEndian.Uint16(raw[10:]), Checksum(raw[:10]); got != want {
		return nil, ErrBadChecksum
	}
	resp := &ResponsePacket{
		DeviceID: binary.LittleEndian.Uint16(raw[2:]),
		Param:    binary.LittleEndian.Uint32(raw[4:]),
		Response: binary.LittleEndian.Uint16(raw[8:]),
	}
	if !resp.IsAck() && !resp.IsNack() {
		return nil, ErrBadResponse
	}
	return resp, nil
}

// MarshalDataFrame serializes a data packet: start codes, device id, payload
// and checksum. The returned frame is len(payload)+6 bytes.
func MarshalDataFrame(deviceID uint16, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+6)
	frame = append(frame, DataStartCode1, DataStartCode2)
	var id [2]byte
	binary.LittleEndian.PutUint16(id[:], deviceID)
	frame = append(frame, id[:]...)
	frame = append(frame, payload...)
	binary.LittleEndian.PutUint16(id[:], Checksum(frame))
	frame = append(frame, id[:]...)
	return frame
}

// UnmarshalDataFrame validates a complete data frame (start codes + device id
// + payload + checksum) and returns the payload.
func UnmarshalDataFrame(frame []byte, deviceID uint16) ([]byte, error) {
	if len(frame) < 6 {
		return nil, ErrShortPacket
	}
	if frame[0] != DataStartCode1 || frame[1] != DataStartCode2 {
		return nil, ErrBadHeader
	}
	if got, want := binary.LittleEndian.Uint16(frame[2:]), deviceID; want != 0 && got != want {
		return nil, ErrBadDeviceID
	}
	if got, want := binary.LittleEndian.Uint16(frame[len(frame)-2:]), Checksum(frame[:len(frame)-2]); got != want {
		return nil, ErrBadChecksum
	}
	return frame[4 : len(frame)-2], nil
}

// DevInfo is the device static information returned by Open with a non-zero
// parameter (datasheet §5.1).
type DevInfo struct {
	FirmwareVersion uint32
	IsoAreaMaxSize  uint32
	SerialNumber    [16]byte
}

// ParseDevInfo parses a 24-byte devinfo payload.
func ParseDevInfo(payload []byte) (*DevInfo, error) {
	if len(payload) < DevInfoSize {
		return nil, fmt.Errorf("protocol: devinfo payload too short: %d", len(payload))
	}
	info := &DevInfo{
		FirmwareVersion: binary.LittleEndian.Uint32(payload[0:]),
		IsoAreaMaxSize:  binary.LittleEndian.Uint32(payload[4:]),
	}
	copy(info.SerialNumber[:], payload[8:24])
	return info, nil
}

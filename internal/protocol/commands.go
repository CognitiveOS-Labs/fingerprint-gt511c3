package protocol

import "fmt"

// Command codes (datasheet §3 "Protocol: Commands Summary").
const (
	CmdOpen             = 0x01
	CmdClose            = 0x02
	CmdUsbInternalCheck = 0x03
	CmdChangeBaudrate   = 0x04
	CmdSetIAPMode       = 0x05
	CmdCmosLed          = 0x12
	CmdGetEnrollCount   = 0x20
	CmdCheckEnrolled    = 0x21
	CmdEnrollStart      = 0x22
	CmdEnroll1          = 0x23
	CmdEnroll2          = 0x24
	CmdEnroll3          = 0x25
	CmdIsPressFinger    = 0x26
	CmdDeleteID         = 0x40
	CmdDeleteAll        = 0x41
	CmdVerify           = 0x50
	CmdIdentify         = 0x51
	CmdVerifyTemplate   = 0x52
	CmdIdentifyTemplate = 0x53
	CmdCaptureFinger    = 0x60
	CmdMakeTemplate     = 0x61
	CmdGetImage         = 0x62
	CmdGetRawImage      = 0x63
	CmdGetTemplate      = 0x70
	CmdSetTemplate      = 0x71
)

// NACK error codes (datasheet §4 "Protocol: Error Codes"). These appear in
// the response packet's Parameter field when the response is NACK.
const (
	NackTimeout          = 0x1001 // obsolete, capture timeout
	NackInvalidBaudrate  = 0x1002 // obsolete, invalid serial baud rate
	NackInvalidPos       = 0x1003 // ID not between 0 and 199
	NackIsNotUsed        = 0x1004 // the specified ID is not used
	NackIsAlreadyUsed    = 0x1005 // the specified ID is already used
	NackCommErr          = 0x1006 // communication error
	NackVerifyFailed     = 0x1007 // 1:1 verification failure
	NackIdentifyFailed   = 0x1008 // 1:N identification failure
	NackDBIsFull         = 0x1009 // the database is full
	NackDBIsEmpty        = 0x100A // the database is empty
	NackTurnErr          = 0x100B // obsolete, invalid order of enrollment
	NackBadFinger        = 0x100C // too bad fingerprint
	NackEnrollFailed     = 0x100D // enrollment failure
	NackIsNotSupported   = 0x100E // the specified command is not supported
	NackDevErr           = 0x100F // device error, especially crypto-chip trouble
	NackCaptureCanceled  = 0x1010 // obsolete, the capturing is canceled
	NackInvalidParam     = 0x1011 // invalid parameter
	NackFingerNotPressed = 0x1012 // finger is not pressed
)

// NackError is returned when the device answers a command with NACK. Code is
// one of the Nack* constants above, or an ID 0-199 when the device reports a
// duplicated fingerprint.
type NackError struct {
	Code uint16
}

func (e *NackError) Error() string {
	if name, ok := nackNames[e.Code]; ok {
		return fmt.Sprintf("NACK: %s (0x%04x)", name, e.Code)
	}
	return fmt.Sprintf("NACK: duplicate id %d (0x%04x)", e.Code, e.Code)
}

func nackName(code uint16) string {
	if name, ok := nackNames[code]; ok {
		return name
	}
	return fmt.Sprintf("0x%04x", code)
}

var nackNames = map[uint16]string{
	NackTimeout:          "NACK_TIMEOUT",
	NackInvalidBaudrate:  "NACK_INVALID_BAUDRATE",
	NackInvalidPos:       "NACK_INVALID_POS",
	NackIsNotUsed:        "NACK_IS_NOT_USED",
	NackIsAlreadyUsed:    "NACK_IS_ALREADY_USED",
	NackCommErr:          "NACK_COMM_ERR",
	NackVerifyFailed:     "NACK_VERIFY_FAILED",
	NackIdentifyFailed:   "NACK_IDENTIFY_FAILED",
	NackDBIsFull:         "NACK_DB_IS_FULL",
	NackDBIsEmpty:        "NACK_DB_IS_EMPTY",
	NackTurnErr:          "NACK_TURN_ERR",
	NackBadFinger:        "NACK_BAD_FINGER",
	NackEnrollFailed:     "NACK_ENROLL_FAILED",
	NackIsNotSupported:   "NACK_IS_NOT_SUPPORTED",
	NackDevErr:           "NACK_DEV_ERR",
	NackCaptureCanceled:  "NACK_CAPTURE_CANCELED",
	NackInvalidParam:     "NACK_INVALID_PARAM",
	NackFingerNotPressed: "NACK_FINGER_IS_NOT_PRESSED",
}

// Timeouts (milliseconds) used by the reference implementations. The SDK uses
// a 15 s default; the Node library uses shorter per-operation timeouts.
const (
	TimeoutCommand = 1000
	TimeoutUpload  = 1000
	TimeoutData    = 5000
	TimeoutImage   = 10000
)

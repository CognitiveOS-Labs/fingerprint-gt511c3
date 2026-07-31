package device

import (
	"errors"
	"fmt"
	"time"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/protocol"
)

// SetLED turns the CMOS sensor LED on or off (datasheet §5.4). The LED is OFF
// by default and must be ON before capturing a fingerprint.
func (d *Device) SetLED(on bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.setLEDLocked(on)
}

func (d *Device) setLEDLocked(on bool) error {
	var param uint32
	if on {
		param = 1
	}
	_, err := d.sendCommandLocked(protocol.CmdCmosLed, param)
	return err
}

// GetEnrollCount returns the number of enrolled fingerprints (§5.6).
func (d *Device) GetEnrollCount() (uint32, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sendCommandLocked(protocol.CmdGetEnrollCount, 0)
}

// CheckEnrolled reports whether the given ID (0-199) is enrolled (§5.7).
func (d *Device) CheckEnrolled(id uint16) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.checkEnrolledLocked(id)
}

func (d *Device) checkEnrolledLocked(id uint16) (bool, error) {
	_, err := d.sendCommandLocked(protocol.CmdCheckEnrolled, uint32(id))
	if err == nil {
		return true, nil
	}
	var nack *protocol.NackError
	if errors.As(err, &nack) && nack.Code == protocol.NackIsNotUsed {
		return false, nil
	}
	return false, err
}

// ListIDs returns the enrolled IDs in the database (0-199).
func (d *Device) ListIDs() ([]uint16, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var ids []uint16
	for id := uint16(0); id < 200; id++ {
		enrolled, err := d.checkEnrolledLocked(id)
		if err != nil {
			return nil, err
		}
		if enrolled {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// EnrollStart begins enrollment for an ID (§5.8).
func (d *Device) EnrollStart(id uint16) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enrollStartLocked(id)
}

func (d *Device) enrollStartLocked(id uint16) error {
	_, err := d.sendCommandLocked(protocol.CmdEnrollStart, uint32(id))
	return err
}

// Enroll1 makes the first template scan (§5.9).
func (d *Device) Enroll1() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enroll1Locked()
}

func (d *Device) enroll1Locked() error {
	_, err := d.sendCommandLocked(protocol.CmdEnroll1, 0)
	return err
}

// Enroll2 makes the second template scan (§5.10).
func (d *Device) Enroll2() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enroll2Locked()
}

func (d *Device) enroll2Locked() error {
	_, err := d.sendCommandLocked(protocol.CmdEnroll2, 0)
	return err
}

// Enroll3 merges the three scans and saves the template (§5.11).
func (d *Device) Enroll3() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enroll3Locked()
}

func (d *Device) enroll3Locked() error {
	_, err := d.sendCommandLocked(protocol.CmdEnroll3, 0)
	return err
}

// CaptureFinger captures the pressed finger into RAM (§5.19). Use best=true
// for enrollment, false for fast identification.
func (d *Device) CaptureFinger(best bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.captureFingerLocked(best)
}

func (d *Device) captureFingerLocked(best bool) error {
	var param uint32
	if best {
		param = 1
	}
	_, err := d.sendCommandLocked(protocol.CmdCaptureFinger, param)
	return err
}

// IsPressFinger reports whether a finger is currently on the sensor (§5.12).
func (d *Device) IsPressFinger() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.isPressFingerLocked()
}

func (d *Device) isPressFingerLocked() (bool, error) {
	param, err := d.sendCommandLocked(protocol.CmdIsPressFinger, 0)
	if err != nil {
		return false, err
	}
	return param == 0, nil
}

// WaitFinger polls until a finger is placed on the sensor or the timeout
// elapses.
func (d *Device) WaitFinger(timeout time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.waitFingerLocked(timeout)
}

func (d *Device) waitFingerLocked(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pressed, err := d.isPressFingerLocked()
		if err != nil {
			return err
		}
		if pressed {
			return nil
		}
		if time.Now().After(deadline) {
			return &TimeoutError{Op: "wait finger"}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// WaitFingerRelease polls until the finger is lifted or the timeout elapses.
func (d *Device) WaitFingerRelease(timeout time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.waitFingerReleaseLocked(timeout)
}

func (d *Device) waitFingerReleaseLocked(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pressed, err := d.isPressFingerLocked()
		if err != nil {
			return err
		}
		if !pressed {
			return nil
		}
		if time.Now().After(deadline) {
			return &TimeoutError{Op: "wait finger release"}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Verify checks the captured finger against a single ID (§5.15). It returns
// matched=false (without error) when the device reports verification failed.
func (d *Device) Verify(id uint16) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sendCommandLocked(protocol.CmdVerify, uint32(id))
	if err == nil {
		return true, nil
	}
	var nack *protocol.NackError
	if errors.As(err, &nack) && nack.Code == protocol.NackVerifyFailed {
		return false, nil
	}
	return false, err
}

// Identify checks the captured finger against the whole database (§5.16).
// On success it returns the matched ID; matched=false when not found.
func (d *Device) Identify() (uint16, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	param, err := d.sendCommandLocked(protocol.CmdIdentify, 0)
	if err == nil {
		return uint16(param), true, nil
	}
	var nack *protocol.NackError
	if errors.As(err, &nack) && nack.Code == protocol.NackIdentifyFailed {
		return 0, false, nil
	}
	return 0, false, err
}

// MakeTemplate builds a template from the captured image and downloads it
// (§5.20). Requires a prior CaptureFinger.
func (d *Device) MakeTemplate() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.sendCommandLocked(protocol.CmdMakeTemplate, 0); err != nil {
		return nil, err
	}
	return d.receiveDataLocked(protocol.TemplateSize, d.dataTimeout)
}

// GetImage downloads the captured 258x202 image (52116 bytes, §5.21). A valid
// finger press must have been captured first.
func (d *Device) GetImage() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.sendCommandLocked(protocol.CmdGetImage, 0); err != nil {
		return nil, err
	}
	return d.receiveDataLocked(protocol.ImageSize, d.imageTimeout)
}

// GetRawImage captures and downloads the 160x120 raw image (19200 bytes,
// §5.22). It does not require a pressed finger.
func (d *Device) GetRawImage() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.sendCommandLocked(protocol.CmdGetRawImage, 0); err != nil {
		return nil, err
	}
	return d.receiveDataLocked(protocol.RawImageSize, d.imageTimeout)
}

// GetTemplate downloads the 498-byte template for an ID (§5.23).
func (d *Device) GetTemplate(id uint16) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.sendCommandLocked(protocol.CmdGetTemplate, uint32(id)); err != nil {
		return nil, err
	}
	return d.receiveDataLocked(protocol.TemplateSize, d.dataTimeout)
}

// SetTemplate uploads a 498-byte template to an ID (§5.24). When skipDupCheck
// is true the fingerprint duplication check is skipped (parameter HIWORD
// non-zero).
func (d *Device) SetTemplate(id uint16, tmpl []byte, skipDupCheck bool) error {
	if len(tmpl) != protocol.TemplateSize {
		return fmt.Errorf("device: template must be %d bytes, got %d", protocol.TemplateSize, len(tmpl))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var param uint32 = uint32(id)
	if skipDupCheck {
		param |= 0x10000
	}
	if _, err := d.sendCommandLocked(protocol.CmdSetTemplate, param); err != nil {
		return err
	}
	_, err := d.sendDataLocked(tmpl, d.uploadTimeout)
	return err
}

// VerifyTemplate checks a 498-byte template against a single ID (§5.17).
func (d *Device) VerifyTemplate(id uint16, tmpl []byte) (bool, error) {
	if len(tmpl) != protocol.TemplateSize {
		return false, fmt.Errorf("device: template must be %d bytes, got %d", protocol.TemplateSize, len(tmpl))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.sendCommandLocked(protocol.CmdVerifyTemplate, uint32(id)); err != nil {
		return false, err
	}
	_, err := d.sendDataLocked(tmpl, d.uploadTimeout)
	if err == nil {
		return true, nil
	}
	var nack *protocol.NackError
	if errors.As(err, &nack) && nack.Code == protocol.NackVerifyFailed {
		return false, nil
	}
	return false, err
}

// IdentifyTemplate checks a 498-byte template against the whole database
// (§5.18). On success it returns the matched ID.
func (d *Device) IdentifyTemplate(tmpl []byte) (uint16, bool, error) {
	if len(tmpl) != protocol.TemplateSize {
		return 0, false, fmt.Errorf("device: template must be %d bytes, got %d", protocol.TemplateSize, len(tmpl))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.sendCommandLocked(protocol.CmdIdentifyTemplate, 0); err != nil {
		return 0, false, err
	}
	param, err := d.sendDataLocked(tmpl, d.uploadTimeout)
	if err == nil {
		return uint16(param), true, nil
	}
	var nack *protocol.NackError
	if errors.As(err, &nack) && nack.Code == protocol.NackIdentifyFailed {
		return 0, false, nil
	}
	return 0, false, err
}

// DeleteID deletes the fingerprint with the given ID (§5.13).
func (d *Device) DeleteID(id uint16) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sendCommandLocked(protocol.CmdDeleteID, uint32(id))
	return err
}

// DeleteAll deletes all fingerprints (§5.14).
func (d *Device) DeleteAll() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.sendCommandLocked(protocol.CmdDeleteAll, 0)
	return err
}

// ChangeBaudrate switches the link to a new baud rate (§5.5) and re-opens the
// port at that rate.
func (d *Device) ChangeBaudrate(baud int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.port == nil {
		return ErrNotConnected
	}
	if baud == d.baud {
		return nil
	}
	if _, err := d.sendCommandLocked(protocol.CmdChangeBaudrate, uint32(baud)); err != nil {
		return err
	}
	if d.portName == "" {
		return nil
	}
	if err := d.port.Close(); err != nil {
		return err
	}
	p, err := openSerialFn(d.portName, baud)
	if err != nil {
		return err
	}
	d.port = p
	d.baud = baud
	return nil
}

// Enroll runs the full enrollment flowchart (datasheet §6.3) for the given ID:
// EnrollStart, then three CaptureFinger+Enroll steps with finger wait/release
// in between. The LED is turned on for the duration.
func (d *Device) Enroll(id uint16) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.setLEDLocked(true); err != nil {
		return fmt.Errorf("enroll: led on: %w", err)
	}
	cleanup := func() { _ = d.setLEDLocked(false) }

	if err := d.waitFingerLocked(DefaultWaitTimeout); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.enrollStartLocked(id); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.captureFingerLocked(true); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.enroll1Locked(); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.waitFingerReleaseLocked(DefaultWaitTimeout); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.waitFingerLocked(DefaultWaitTimeout); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.captureFingerLocked(true); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.enroll2Locked(); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.waitFingerReleaseLocked(DefaultWaitTimeout); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.waitFingerLocked(DefaultWaitTimeout); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.captureFingerLocked(true); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	if err := d.enroll3Locked(); err != nil {
		cleanup()
		return fmt.Errorf("enroll: %w", err)
	}
	cleanup()
	return nil
}

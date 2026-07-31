package mcp

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/device"
	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/protocol"
)

// argError reports an invalid or missing tool argument.
type argError struct{ msg string }

func (e *argError) Error() string { return e.msg }

func invalidArg(format string, args ...interface{}) error {
	return &argError{msg: fmt.Sprintf(format, args...)}
}

func getString(args map[string]interface{}, name string) (string, error) {
	v, ok := args[name]
	if !ok {
		return "", invalidArg("missing argument %q", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", invalidArg("argument %q must be a string", name)
	}
	return s, nil
}

func getInt(args map[string]interface{}, name string) (int, error) {
	v, ok := args[name]
	if !ok {
		return 0, invalidArg("missing argument %q", name)
	}
	// JSON numbers decode as float64.
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, invalidArg("argument %q must be an integer", name)
	}
}

func getBool(args map[string]interface{}, name string, def bool) (bool, error) {
	v, ok := args[name]
	if !ok {
		return def, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, invalidArg("argument %q must be a boolean", name)
	}
	return b, nil
}

func getTemplateArg(args map[string]interface{}, name string) ([]byte, error) {
	s, err := getString(args, name)
	if err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, invalidArg("argument %q must be base64", name)
	}
	if len(b) != protocol.TemplateSize {
		return nil, invalidArg("argument %q must decode to %d bytes, got %d", name, protocol.TemplateSize, len(b))
	}
	return b, nil
}

// buildTools constructs the full tool registry.
func buildTools(dev *device.Device) []tool {
	prefix := ToolPrefix
	t := func(name, desc string, schema map[string]interface{}, h toolHandler) tool {
		return tool{
			Name:        prefix + "." + name,
			Description: desc,
			InputSchema: schema,
			handler:     h,
		}
	}
	obj := func(props map[string]interface{}, required ...string) map[string]interface{} {
		req := required
		if req == nil {
			req = []string{}
		}
		return map[string]interface{}{
			"type":       "object",
			"properties": props,
			"required":   req,
		}
	}
	str := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	num := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "integer", "description": desc}
	}
	bl := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "boolean", "description": desc}
	}

	tools := []tool{
		t("connect", "Open the serial port and initialize the scanner (baud scan).", obj(map[string]interface{}{
			"port": str("Serial port device path, e.g. /dev/ttyUSB0"),
			"baud": num("Optional baud rate; defaults to a scan of 9600/115200/57600/38400/19200"),
		}, "port"), func(args map[string]interface{}) (interface{}, error) {
			port, err := getString(args, "port")
			if err != nil {
				return nil, err
			}
			baud, err := optionalInt(args, "baud", 0)
			if err != nil {
				return nil, err
			}
			return connectResult(dev, port, baud)
		}),

		t("disconnect", "Close the serial port (best-effort Close command).", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			err := dev.Close()
			return map[string]interface{}{"connected": false}, err
		}),

		t("status", "Report connection state, port, baud and enrollment count.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			out := map[string]interface{}{
				"connected": dev.Connected(),
				"port":      dev.PortName(),
				"baud":      dev.Baud(),
			}
			if dev.Connected() {
				count, err := dev.GetEnrollCount()
				if err != nil {
					return nil, err
				}
				out["enroll_count"] = count
			}
			return out, nil
		}),

		t("info", "Return device static info (firmware version, ISO area, serial number).", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			info := dev.Info()
			if info == nil {
				return nil, device.ErrNotConnected
			}
			return map[string]interface{}{
				"firmware_version":  fmt.Sprintf("0x%08x", info.FirmwareVersion),
				"iso_area_max_size": info.IsoAreaMaxSize,
				"serial_number":     string(info.SerialNumber[:]),
			}, nil
		}),

		t("set_led", "Turn the CMOS sensor LED on or off (required before capture).", obj(map[string]interface{}{
			"on": bl("true to turn the LED on"),
		}, "on"), func(args map[string]interface{}) (interface{}, error) {
			on, err := getBool(args, "on", false)
			if err != nil {
				return nil, err
			}
			if err := dev.SetLED(on); err != nil {
				return nil, err
			}
			return map[string]interface{}{"led_on": on}, nil
		}),

		t("get_enroll_count", "Return the number of enrolled fingerprints.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			count, err := dev.GetEnrollCount()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"count": count}, nil
		}),

		t("check_enrolled", "Check whether an ID (0-199) is enrolled.", obj(map[string]interface{}{
			"id": num("Enrollment ID 0-199"),
		}, "id"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			enrolled, err := dev.CheckEnrolled(id)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "enrolled": enrolled}, nil
		}),

		t("list_ids", "List all enrolled IDs.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			ids, err := dev.ListIDs()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"ids": ids, "count": len(ids)}, nil
		}),

		t("enroll", "Run the full enrollment flow for an ID (three scans).", obj(map[string]interface{}{
			"id": num("Enrollment ID 0-199"),
		}, "id"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			if err := dev.Enroll(id); err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "enrolled": true}, nil
		}),

		t("enroll_start", "Begin enrollment for an ID.", obj(map[string]interface{}{
			"id": num("Enrollment ID 0-199"),
		}, "id"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			if err := dev.EnrollStart(id); err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id}, nil
		}),

		t("enroll_1", "First enrollment scan (finger must be pressed).", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			return ackOnly(dev.Enroll1())
		}),

		t("enroll_2", "Second enrollment scan (finger must be pressed).", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			return ackOnly(dev.Enroll2())
		}),

		t("enroll_3", "Merge the three scans and save the template.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			return ackOnly(dev.Enroll3())
		}),

		t("capture_finger", "Capture the pressed finger into RAM.", obj(map[string]interface{}{
			"best": bl("true (default) uses the best image for enrollment; false for fast identification"),
		}), func(args map[string]interface{}) (interface{}, error) {
			best, err := getBool(args, "best", true)
			if err != nil {
				return nil, err
			}
			if err := dev.CaptureFinger(best); err != nil {
				return nil, err
			}
			return map[string]interface{}{"captured": true}, nil
		}),

		t("is_press_finger", "Check whether a finger is currently on the sensor.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			pressed, err := dev.IsPressFinger()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"pressed": pressed}, nil
		}),

		t("wait_finger", "Wait until a finger is placed on the sensor or the timeout elapses.", obj(map[string]interface{}{
			"timeout_seconds": num("Timeout in seconds (default 10)"),
		}), func(args map[string]interface{}) (interface{}, error) {
			timeout, err := optionalInt(args, "timeout_seconds", int(device.DefaultWaitTimeout/time.Second))
			if err != nil {
				return nil, err
			}
			if err := dev.WaitFinger(time.Duration(timeout) * time.Second); err != nil {
				return nil, err
			}
			return map[string]interface{}{"pressed": true}, nil
		}),

		t("wait_finger_release", "Wait until the finger is lifted or the timeout elapses.", obj(map[string]interface{}{
			"timeout_seconds": num("Timeout in seconds (default 10)"),
		}), func(args map[string]interface{}) (interface{}, error) {
			timeout, err := optionalInt(args, "timeout_seconds", int(device.DefaultWaitTimeout/time.Second))
			if err != nil {
				return nil, err
			}
			if err := dev.WaitFingerRelease(time.Duration(timeout) * time.Second); err != nil {
				return nil, err
			}
			return map[string]interface{}{"released": true}, nil
		}),

		t("verify", "Verify the captured finger against a single ID (1:1).", obj(map[string]interface{}{
			"id": num("Enrollment ID 0-199"),
		}, "id"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			matched, err := dev.Verify(id)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "matched": matched}, nil
		}),

		t("identify", "Identify the captured finger against the whole database (1:N).", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			id, matched, err := dev.Identify()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"matched": matched, "id": id}, nil
		}),

		t("make_template", "Build a template from the captured image and download it (base64).", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			tmpl, err := dev.MakeTemplate()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"template": base64.StdEncoding.EncodeToString(tmpl)}, nil
		}),

		t("get_image", "Download the captured 258x202 image as base64.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			img, err := dev.GetImage()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"image":  base64.StdEncoding.EncodeToString(img),
				"width":  protocol.ImageWidth,
				"height": protocol.ImageHeight,
			}, nil
		}),

		t("get_raw_image", "Capture and download the 160x120 raw image as base64.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			img, err := dev.GetRawImage()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"image":  base64.StdEncoding.EncodeToString(img),
				"width":  protocol.RawImageWidth,
				"height": protocol.RawImageHeight,
			}, nil
		}),

		t("get_template", "Download the 498-byte template for an ID as base64.", obj(map[string]interface{}{
			"id": num("Enrollment ID 0-199"),
		}, "id"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			tmpl, err := dev.GetTemplate(id)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "template": base64.StdEncoding.EncodeToString(tmpl)}, nil
		}),

		t("set_template", "Upload a 498-byte template (base64) to an ID.", obj(map[string]interface{}{
			"id":             num("Enrollment ID 0-199"),
			"template":       str("Base64-encoded 498-byte template"),
			"skip_dup_check": bl("Skip the fingerprint duplication check"),
		}, "id", "template"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			tmpl, err := getTemplateArg(args, "template")
			if err != nil {
				return nil, err
			}
			skip, err := getBool(args, "skip_dup_check", false)
			if err != nil {
				return nil, err
			}
			if err := dev.SetTemplate(id, tmpl, skip); err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "enrolled": true}, nil
		}),

		t("verify_template", "Verify a 498-byte template (base64) against a single ID.", obj(map[string]interface{}{
			"id":       num("Enrollment ID 0-199"),
			"template": str("Base64-encoded 498-byte template"),
		}, "id", "template"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			tmpl, err := getTemplateArg(args, "template")
			if err != nil {
				return nil, err
			}
			matched, err := dev.VerifyTemplate(id, tmpl)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "matched": matched}, nil
		}),

		t("identify_template", "Identify a 498-byte template (base64) against the whole database.", obj(map[string]interface{}{
			"template": str("Base64-encoded 498-byte template"),
		}, "template"), func(args map[string]interface{}) (interface{}, error) {
			tmpl, err := getTemplateArg(args, "template")
			if err != nil {
				return nil, err
			}
			id, matched, err := dev.IdentifyTemplate(tmpl)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"matched": matched, "id": id}, nil
		}),

		t("delete_id", "Delete the fingerprint with the given ID.", obj(map[string]interface{}{
			"id": num("Enrollment ID 0-199"),
		}, "id"), func(args map[string]interface{}) (interface{}, error) {
			id, err := idArg(args)
			if err != nil {
				return nil, err
			}
			if err := dev.DeleteID(id); err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": id, "deleted": true}, nil
		}),

		t("delete_all", "Delete all enrolled fingerprints.", obj(map[string]interface{}{}), func(args map[string]interface{}) (interface{}, error) {
			if err := dev.DeleteAll(); err != nil {
				return nil, err
			}
			return map[string]interface{}{"deleted": true}, nil
		}),

		t("change_baudrate", "Switch the serial link to a new baud rate.", obj(map[string]interface{}{
			"baud": num("New baud rate, e.g. 115200"),
		}, "baud"), func(args map[string]interface{}) (interface{}, error) {
			baud, err := getInt(args, "baud")
			if err != nil {
				return nil, err
			}
			if err := dev.ChangeBaudrate(baud); err != nil {
				return nil, err
			}
			return map[string]interface{}{"baud": baud}, nil
		}),
	}

	return tools
}

// idArg validates an enrollment ID.
func idArg(args map[string]interface{}) (uint16, error) {
	v, err := getInt(args, "id")
	if err != nil {
		return 0, err
	}
	if v < 0 || v > 199 {
		return 0, invalidArg("id must be in range 0-199, got %d", v)
	}
	return uint16(v), nil
}

// optionalInt returns the named argument if present, otherwise def.
func optionalInt(args map[string]interface{}, name string, def int) (int, error) {
	if _, ok := args[name]; !ok {
		return def, nil
	}
	return getInt(args, name)
}

func ackOnly(err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true}, nil
}

func connectResult(dev *device.Device, port string, baud int) (interface{}, error) {
	if err := dev.Connect(port, baud); err != nil {
		return nil, err
	}
	info := dev.Info()
	out := map[string]interface{}{
		"connected": true,
		"port":      dev.PortName(),
		"baud":      dev.Baud(),
	}
	if info != nil {
		out["firmware_version"] = fmt.Sprintf("0x%08x", info.FirmwareVersion)
		out["iso_area_max_size"] = info.IsoAreaMaxSize
		out["serial_number"] = string(info.SerialNumber[:])
	}
	return out, nil
}

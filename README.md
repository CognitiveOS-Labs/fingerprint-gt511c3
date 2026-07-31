# fingerprint-gt511c3

A `.cgp` package for CognitiveOS providing a driver and MCP bridge for the
ADH Tech **GT-511C3** fingerprint scanner. The driver is implemented from the
datasheet in pure Go (no cgo) and speaks the GT-511C3 TTL UART protocol
(9600 baud default).

## Hardware

The GT-511C3 is a 200-fingerprint optical scanner with a TTL UART interface.
It is **3.3V only** — a 5V FTDI adapter requires a level shifter.

| GT-511C3 J2 pin | Signal    | Connect to  |
|-----------------|-----------|-------------|
| 1               | UART TX   | FTDI RX     |
| 2               | UART RX   | FTDI TX     |
| 3               | GND       | FTDI GND    |
| 4               | Vin 3.3-6V | 3.3V (or 5V) |

The scanner appears as a USB serial device (`/dev/ttyUSB0`). It boots at
9600 baud and persists its last configured baud; `connect` scans
9600/115200/57600/38400/19200.

## Tools

The MCP server exposes `com.cognitiveos.labs.fingerprint-gt511c3.*` over
stdio:

- `connect` / `disconnect` / `status` / `info`
- `set_led`, `is_press_finger`, `wait_finger`, `wait_finger_release`
- `enroll`, `enroll_start`, `enroll_1`, `enroll_2`, `enroll_3`,
  `capture_finger`
- `verify`, `identify`, `verify_template`, `identify_template`
- `make_template`, `get_template`, `set_template`
- `get_image`, `get_raw_image`
- `get_enroll_count`, `check_enrolled`, `list_ids`, `delete_id`,
  `delete_all`, `change_baudrate`

## Build

```bash
make build          # builds tools/gt511c3-mcp
make test           # unit + fake-device integration tests
make pack           # cpm pack -> .cgp archive
make verify         # cpm verify on the archive
```

## License

MIT

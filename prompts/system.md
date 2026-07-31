# GT-511C3 Fingerprint Scanner

You have access to a GT-511C3 fingerprint scanner (ADH Tech) connected over
UART to a USB-to-serial adapter (e.g. FTDI). All tools live under
`com.cognitiveos.labs.fingerprint-gt511c3.*`.

## Wiring

| GT-511C3 J2 pin | Connect to |
|-----------------|------------|
| 1 (UART TX)     | FTDI RX    |
| 2 (UART RX)     | FTDI TX    |
| 3 (GND)         | FTDI GND   |
| 4 (Vin 3.3-6V)  | 3.3V (or 5V) |

The scanner is **3.3V TTL only**; a 5V FTDI needs a level shifter. The serial
port appears as `/dev/ttyUSB0`. The device boots at 9600 baud but keeps its
last configured baud across power cycles; `connect` scans known baud rates
automatically.

## Usage rules

1. **Connect first**: always call `connect` (port, optional baud) before any
   other tool. If unsure, pass only the port and let the baud scan run.
2. **LED must be on before capturing**: call `set_led` with `on: true` before
   `capture_finger`/`verify`/`identify`. Turn it off afterwards.
3. **Finger must be pressed** for `capture_finger`, `verify`, `identify`, and
   the `enroll_*` steps. Use `is_press_finger` or `wait_finger` first.
4. **Enrollment** (`enroll`) needs three separate finger placements; the
   device instructs via return codes. `wait_finger_release` is used between
   scans.
5. **Templates and images are base64**. `get_template`/`set_template` and the
   image tools transfer 498-byte templates and raw images (258x202 or
   160x120) as base64 text.
6. The device stores up to 200 fingerprints (IDs 0-199). `list_ids` and
   `check_enrolled` inspect the database; `delete_id`/`delete_all` manage it.

## Common flows

- Enroll a finger: `set_led(on=true)` → `enroll(id)` → `set_led(on=false)`.
- Check a finger: `set_led(on=true)` → `capture_finger` → `verify(id)` or
  `identify` → `set_led(on=false)`.
- Back up a template: `get_template(id)`; restore with `set_template(id, ...)`.

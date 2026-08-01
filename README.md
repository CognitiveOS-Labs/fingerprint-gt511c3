# fingerprint-gt511c3

A **Cognitive Patch** (`.cgp`) for **CognitiveOS** — an independent operating
system where the AI is the OS — providing a driver and MCP bridge for the ADH
Tech **GT-511C3** fingerprint scanner. The driver is implemented from the
datasheet in pure Go (no cgo) and speaks the GT-511C3 TTL UART protocol
(9600 baud default).

See [cognitive-os.org](https://cognitive-os.org) to learn more about
CognitiveOS.

## Hardware

### Manufacturer: ADH Tech

The GT-511C3 is made by **ADH Technology Co., Ltd.** ([adh-tech.com.tw](http://www.adh-tech.com.tw)),
a Taiwanese manufacturer of embedded biometric modules. ADH Tech publishes the
official datasheet and Windows SDK demo that this driver was reconstructed
from. Distributors (DFRobot, SparkFun SEN-13007, rhydoLABZ, …) sell the same
module branded as the "ADH-Tech fingerprint scanner"; the product family also
includes the GT-511C1R variant.

The module is an all-in-one optical fingerprint reader: an optical sensor plus
a 32-bit CPU that does enrollment, template matching, and storage (up to 200
fingerprints) entirely on-board. It stores only the analyzed fingerprint
**template** (498 bytes) locally, but can also stream the full 258x202 image
or the 160x120 raw sensor image over UART, and templates can be downloaded and
distributed to other modules.

### Electrical interface

The GT-511C3 is a **3.3V TTL only** optical scanner — a 5V FTDI adapter
requires a level shifter.

| GT-511C3 J2 pin | Signal    | Connect to  |
|-----------------|-----------|-------------|
| 1               | UART TX   | FTDI RX     |
| 2               | UART RX   | FTDI TX     |
| 3               | GND       | FTDI GND    |
| 4               | Vin 3.3-6V | 3.3V (or 5V) |

The scanner appears as a USB serial device (`/dev/ttyUSB0`). It boots at
9600 baud and persists its last configured baud; `connect` scans
9600/115200/57600/38400/19200.

## References

Datasheets and tutorials are preserved in the [`docs/`](docs/) folder:

- GT-511C3 datasheet V2.1 (2016-10-25): `docs/GT-511C3_datasheet_V2.1_20161025.pdf`
- OCR-processed copy: `docs/GT-511C3_datasheet_V2.1_20161025-processed.pdf`
- SparkFun SEN-13007 tutorial: `docs/sen13007_GT511C3.pdf`

Original hosting links (may go offline; see `docs/README.md` for details and
checksums):

- https://cdn.sparkfun.com/datasheets/Sensors/Biometric/GT-511C3_datasheet_V2.1_20161025.pdf
- https://www.sigmaelectronica.net/wp-content/uploads/2018/08/sen13007.pdf

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

## Install

The published release can be installed directly from GitHub with the `cpm`
package manager:

```bash
cpm install ghr:CognitiveOS-Labs/fingerprint-gt511c3@v0.1.1
```

Patches are installed under `/cognitiveos/patches` by default. If that path is
not writable (e.g. installing into a custom patch directory), point
`CPM_PATCHES_DIR` at the target directory first:

```bash
export CPM_PATCHES_DIR=/path/to/patches
cpm install ghr:CognitiveOS-Labs/fingerprint-gt511c3@v0.1.1
```

## Build

```bash
make build          # builds tools/gt511c3-mcp
make test           # unit + fake-device integration tests
make pack           # cpm pack -> .cgp archive
make verify         # cpm verify on the archive
```

## License

MIT

## Author

Built and maintained by [jeanmachuca](https://github.com/jeanmachuca).

Support the author: <https://github.com/sponsors/jeanmachuca>

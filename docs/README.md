# Reference documents

Original datasheets and tutorials for the ADH Tech GT-511C3 fingerprint
scanner, preserved in this repo in case the original hosting links go
offline.

| File | Description | Size |
|------|-------------|------|
| `GT-511C3_datasheet_V2.1_20161025.pdf` | Official GT-511C3 datasheet, V2.1 (2016-10-25) | 4.5 MB |
| `GT-511C3_datasheet_V2.1_20161025-processed.pdf` | Same datasheet, OCR-processed copy | 8.2 MB |
| `sen13007_GT511C3.pdf` | SparkFun SEN-13007 tutorial (Fingerprint Scanner TTL, GT-511C3) | 1.5 MB |

## Source links

- GT-511C3 datasheet (SparkFun mirror):
  https://cdn.sparkfun.com/datasheets/Sensors/Biometric/GT-511C3_datasheet_V2.1_20161025.pdf
- SparkFun SEN-13007 tutorial (Sigma Electronica mirror):
  https://www.sigmaelectronica.net/wp-content/uploads/2018/08/sen13007.pdf
- SparkFun SEN-13007 product page:
  https://www.sparkfun.com/products/13007
- ADH Tech (manufacturer) product page:
  http://www.adh-tech.com.tw/?22,gt-511c3-gt-511c31-%28uart%29

## Integrity

`GT-511C3_datasheet_V2.1_20161025.pdf` was verified against the SparkFun CDN
mirror (same SHA-256: `8fcb091a1d64f96b87585e406bab24c8ff2a0fcd53c06e6e32e4d2bc799ffb48`).
The V2.1 datasheets are image-only PDFs (no text layer); protocol details were
cross-checked against the V1.1 text layer and the official SDK
(`sb_protocol_oem.cpp`).

# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased

## [0.3.0] - 2026-09-08

### Added
- Test suite covering argument validation, exporter scraping, basic auth, mTLS and the
  OK/CRITICAL/UNKNOWN exit paths.

### Changed
- Return UNKNOWN instead of OK when the requested metric is absent from the exporter
  response.
- Updated transitive dependencies and tidied `go.mod`.

### Fixed
- Panic on every scrape (`Invalid name validation scheme requested: unset`) after the
  upgrade to prometheus/common v0.71.0, which requires the text parser to be built with
  an explicit name validation scheme.
- Typos in the `--label` flag usage text and in the check's OK output.
- Rewrote the README, which was still the unmodified check-plugin-template boilerplate,
  and replaced the placeholder changelog with real release history.

## [0.2.4] - 2025-02-06

### Changed
- Upgraded Prometheus and other modules.

## [0.2.3] - 2025-01-10

### Changed
- Updated the Go version and modules.

## [0.2.2] - 2024-11-21

### Changed
- Updated modules.

## [0.2.1] - 2024-10-10

### Changed
- Updated modules.

## [0.2] - 2024-09-16

### Changed
- Updated Go and libraries.

## [0.1.2] - 2024-07-01

### Added
- Support for mTLS, HTTP basic auth and label matching.

## [0.1.1] - 2024-06-27

### Changed
- `--min`, `--max` and `--value` now default to an unset sentinel, so each threshold is
  only applied when it is explicitly set.

## [0.1.0] - 2024-06-26

### Added
- Initial release.

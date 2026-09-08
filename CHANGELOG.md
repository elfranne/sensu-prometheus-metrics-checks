# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased

### Fixed
- Panic on every scrape (`Invalid name validation scheme requested: unset`) after the
  upgrade to prometheus/common v0.71.0, which requires the text parser to be built with
  an explicit name validation scheme.

## [0.0.1] - 2000-01-01

### Added
- Initial release

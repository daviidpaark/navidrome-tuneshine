# Changelog

All notable changes to this project are documented in this file.

## [0.2.3] - 2026-07-04

### Fixed

- Refresh artwork correctly when switching tracks by handling playback state `starting` as an active playback event.

### Build

- Update `clean` target to remove accidental Windows executable artifacts (`tuneshine.exe`).
- Ignore `.exe` build outputs in Git.

## [0.2.2] - 2026-07-03

### Added

- Optional user allowlist (`user`) to restrict which Navidrome users can update the Tuneshine display.

### Changed

- Updated dependencies and docs for Navidrome `v0.62.0` compatibility.
- Updated install instructions to use plugin rescan flow.

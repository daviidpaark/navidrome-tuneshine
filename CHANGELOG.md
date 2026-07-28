# Changelog

All notable changes to this project are documented in this file.

## [0.3.0] - 2026-07-28

### Highlights

- Added debounced display clearing to eliminate screen flicker during seeks and rapid track transitions.
- Added artwork hash deduplication to skip redundant WebP re-encoding and HTTP uploads for identical cover art.

### Added

- Debounced display clearing via Navidrome Scheduler API (`scheduler` permission).
- Artwork hash deduplication (FNV-64a) for consecutive tracks with identical cover art.

### Changed

- Omit track title from uploaded metadata so display retains accurate album/artist info without stale track names.

## [0.2.4] - 2026-07-08

### Fixed

- Handle `json.Marshal` error for track metadata instead of silently discarding it.

## [0.2.3] - 2026-07-04

### Highlights

- Fixed artwork refresh when switching tracks during active playback.
- Improved packaging hygiene for Windows-based development environments.

### Fixed

- Refresh artwork correctly when switching tracks by handling playback state `starting` as an active playback event.

### Build

- Update `clean` target to remove accidental Windows executable artifacts (`tuneshine.exe`).
- Ignore `.exe` build outputs in Git.

## [0.2.2] - 2026-06-08

### Added

- Optional user allowlist (`user`) to restrict which Navidrome users can update the Tuneshine display.

### Changed

- Updated installation instructions to use plugin rescan flow.

## [0.2.1] - 2026-06-08

### Changed

- Updated dependencies for Navidrome `v0.62.0` compatibility.
- Updated README to document the Navidrome `v0.62.0+` requirement.

## [0.2.0] - 2026-05-03

### Changed

- Switched playback handling to `PlaybackReport` events.
- Removed scheduler-based state management.

### Fixed

- Clear the Tuneshine display immediately on pause, stop, or expired playback states.

## [0.1.0] - 2026-04-11

### Added

- Initial Tuneshine plugin release for Navidrome.
- Album art upload pipeline with metadata posting to Tuneshine devices.
- Display clearing on non-playing states.
- 64x64 artwork processing and WebP conversion.

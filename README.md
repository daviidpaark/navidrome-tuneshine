# Tuneshine Plugin for Navidrome

A [Navidrome](https://www.navidrome.org/) plugin that sends album art and track metadata to a [Tuneshine](https://www.tuneshine.rocks/) device on your local network in real time as you listen to music.

## Features

- Displays 64×64 album art on the Tuneshine device when a track starts playing
- Sends track title, artist, and album name as metadata
- Automatically clears the display when the track ends
- Deduplicates requests for the same track
- Works for all Navidrome users

## How It Works

When Navidrome reports a "now playing" event, the plugin:

1. Fetches the track's cover art server-side via the Subsonic API
2. Resizes and converts it to 64×64 lossless WebP
3. POSTs the image and metadata to the Tuneshine device as multipart/form-data
4. Schedules a timer to clear the display when the track finishes

> **Note:** The Navidrome PDK does not expose a "playback stopped" event, so the plugin uses a duration-based timer to clear the display. If playback is paused, the display may clear before the track is resumed. This is the same approach used by the official Discord Rich Presence plugin.

## Installation

1. Download `tuneshine.ndp` from the releases
2. Place it in your Navidrome plugins directory
3. Restart Navidrome
4. Go to **Settings → Plugins → Tuneshine** and configure:
   - **Tuneshine Device Host** — IP address or hostname of your Tuneshine (e.g. `192.168.1.100` or `tuneshine.local`)
   - **Service Name** — Label shown on the Tuneshine display (default: `Navidrome`)
5. Enable the plugin

## Building from Source

Requires Go 1.25+ with WASM support.

```sh
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
zip tuneshine.ndp plugin.wasm manifest.json
```

## Configuration

| Setting | Required | Default | Description |
|---------|----------|---------|-------------|
| `host` | Yes | — | Hostname or IP of the Tuneshine device |
| `servicename` | No | `Navidrome` | Music source name shown on the display |

## Dependencies

| Package | Purpose |
|---------|--------|
| [navidrome/navidrome/plugins/pdk/go](https://github.com/navidrome/navidrome) | Navidrome Plugin Development Kit |
| [HugoSmits86/nativewebp](https://github.com/HugoSmits86/nativewebp) | Pure Go lossless WebP encoder (VP8L) |
| [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) | Bilinear image scaling |

## References

- [Navidrome Plugin Development Kit Documentation](https://www.navidrome.org/docs/developers/plugins/)
- [Navidrome Discord Rich Presence Plugin](https://github.com/navidrome/navidrome/tree/master/plugins/discord) — used as the reference implementation for scrobbler + scheduler patterns
- [Tuneshine Help Page](https://tuneshine.com/help) — device API documentation
- [Navidrome OpenAPI Specification](https://www.navidrome.org/docs/developers/subsonic-api/) — Subsonic API endpoints for cover art retrieval

## Disclosure

This project was created entirely with **GitHub Copilot** using **Claude Opus 4.6**. All code, configuration, and documentation were generated through an iterative conversation with the AI assistant, including debugging network compatibility issues, discovering the Tuneshine's chunked transfer encoding limitation, and pivoting to server-side image processing.

// Tuneshine Plugin for Navidrome
//
// Sends album art and track metadata to a Tuneshine device on the LAN
// when playing, and clears the display when paused or stopped.
//
// Capabilities: Scrobbler
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/scrobbler"
	"golang.org/x/image/draw"
)

const (
	hostKey        = "host"
	serviceNameKey = "servicename"
	userKey        = "user"

	cacheKeyLastTrack = "last-track-id"
	cacheTTLSeconds   = 3600 // 1 hour

	tuneshineSize = 64 // Tuneshine display is 64x64
)

// tuneshine implements the scrobbler interface.
type tuneshine struct{}

func init() {
	scrobbler.Register(&tuneshine{})
}

// getConfig loads the plugin configuration.
func getConfig() (deviceHost, serviceName string, err error) {
	deviceHost, ok := pdk.GetConfig(hostKey)
	if !ok || deviceHost == "" {
		return "", "", fmt.Errorf("tuneshine host not configured")
	}

	serviceName, ok = pdk.GetConfig(serviceNameKey)
	if !ok || serviceName == "" {
		serviceName = "Navidrome"
	}

	return deviceHost, serviceName, nil
}

// trackMetadata is the JSON metadata sent alongside multipart image uploads.
type trackMetadata struct {
	TrackName   string `json:"trackName,omitempty"`
	ArtistName  string `json:"artistName,omitempty"`
	AlbumName   string `json:"albumName,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	ItemID      string `json:"itemId,omitempty"`
}

// convertToWebP decodes an image (JPEG/PNG), resizes it to 64x64, and encodes as lossless WebP.
func convertToWebP(imageData []byte) ([]byte, error) {
	// Decode the source image
	src, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Resize to 64x64 using bilinear interpolation
	dst := image.NewRGBA(image.Rect(0, 0, tuneshineSize, tuneshineSize))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	// Encode to WebP lossless
	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, dst, nil); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}

	return buf.Bytes(), nil
}

// ============================================================================
// Scrobbler Implementation
// ============================================================================

// IsAuthorized allows all users, or only the configured user(s) if "user" is set.
// "user" may be a single username or a comma-separated list (e.g. "user1,user2").
func (t *tuneshine) IsAuthorized(input scrobbler.IsAuthorizedRequest) (bool, error) {
	allowed, ok := pdk.GetConfig(userKey)
	if !ok || allowed == "" {
		return true, nil
	}
	for _, name := range strings.Split(allowed, ",") {
		if strings.TrimSpace(name) == input.Username {
			return true, nil
		}
	}
	return false, nil
}

// NowPlaying is a no-op (playback state is handled by PlaybackReport).
func (t *tuneshine) NowPlaying(_ scrobbler.NowPlayingRequest) error {
	return nil
}

// PlaybackReport handles playback state changes from Navidrome.
// Shows the track image when playing, clears it on paused/stopped/expired.
func (t *tuneshine) PlaybackReport(input scrobbler.PlaybackReportRequest) error {
	switch input.State {
	case "playing", "starting":
		return t.displayTrack(input)
	case "paused", "stopped", "expired":
		return t.clearDisplay()
	default:
		// Ignore unknown states.
		return nil
	}
}

// displayTrack uploads track artwork and metadata to the Tuneshine device.
func (t *tuneshine) displayTrack(input scrobbler.PlaybackReportRequest) error {
	deviceHost, serviceName, err := getConfig()
	if err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine config error: %v", err))
		return nil
	}

	// Check if the track changed — skip upload if already displaying this track.
	lastID, found, _ := host.CacheGetString(cacheKeyLastTrack)
	if found && lastID == input.Track.ID {
		pdk.Log(pdk.LogInfo, "Tuneshine: same track, skipping re-upload")
		return nil
	}

	if err := uploadTrackImage(input, deviceHost, serviceName); err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine: %v", err))
		return nil
	}
	_ = host.CacheSetString(cacheKeyLastTrack, input.Track.ID, cacheTTLSeconds)
	return nil
}

// clearDisplay sends DELETE /image to the Tuneshine to revert to the idle screen.
// The cached track ID is also cleared so the next play re-uploads fresh artwork.
func (t *tuneshine) clearDisplay() error {
	deviceHost, _, err := getConfig()
	if err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine config error: %v", err))
		return nil
	}

	url := fmt.Sprintf("http://%s/image", deviceHost)
	resp, err := host.HTTPSend(host.HTTPRequest{
		Method:    "DELETE",
		URL:       url,
		TimeoutMs: 10000,
	})
	if err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine: HTTP error deleting image: %v", err))
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine: DELETE /image returned %d: %s", resp.StatusCode, string(resp.Body)))
		return nil
	}

	pdk.Log(pdk.LogInfo, "Tuneshine: cleared display")
	_ = host.CacheRemove(cacheKeyLastTrack)
	return nil
}

// uploadTrackImage fetches artwork via SubsonicAPI, converts to WebP, and POSTs to the Tuneshine.
func uploadTrackImage(input scrobbler.PlaybackReportRequest, deviceHost, serviceName string) error {
	// Fetch artwork directly from Navidrome via SubsonicAPI (server-side)
	_, imageData, err := host.SubsonicAPICallRaw(
		fmt.Sprintf("/getCoverArt?u=%s&id=%s&size=%d", input.Username, input.Track.ID, tuneshineSize),
	)
	if err != nil {
		return fmt.Errorf("failed to fetch artwork: %w", err)
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Tuneshine: fetched artwork %d bytes", len(imageData)))

	// Convert JPEG/PNG to 64x64 WebP
	webpData, err := convertToWebP(imageData)
	if err != nil {
		return fmt.Errorf("failed to convert to WebP: %w", err)
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Tuneshine: converted to WebP %d bytes", len(webpData)))

	// Build metadata JSON
	meta := trackMetadata{
		TrackName:   input.Track.Title,
		ArtistName:  input.Track.Artist,
		AlbumName:   input.Track.Album,
		ServiceName: serviceName,
		ItemID:      input.Track.ID,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Build multipart/form-data body (no mime/multipart for WASM compat)
	boundary := "----TuneshineUpload"
	var body []byte

	// image field
	body = append(body, []byte(fmt.Sprintf("--%s\r\n", boundary))...)
	body = append(body, []byte("Content-Disposition: form-data; name=\"image\"; filename=\"cover.webp\"\r\n")...)
	body = append(body, []byte("Content-Type: image/webp\r\n")...)
	body = append(body, []byte("\r\n")...)
	body = append(body, webpData...)
	body = append(body, []byte("\r\n")...)

	// metadata field
	body = append(body, []byte(fmt.Sprintf("--%s\r\n", boundary))...)
	body = append(body, []byte("Content-Disposition: form-data; name=\"metadata\"\r\n")...)
	body = append(body, []byte("Content-Type: application/json\r\n")...)
	body = append(body, []byte("\r\n")...)
	body = append(body, metaJSON...)
	body = append(body, []byte("\r\n")...)

	// closing boundary
	body = append(body, []byte(fmt.Sprintf("--%s--\r\n", boundary))...)

	// POST multipart to Tuneshine
	url := fmt.Sprintf("http://%s/image", deviceHost)
	resp, err := host.HTTPSend(host.HTTPRequest{
		Method:    "POST",
		URL:       url,
		Headers:   map[string]string{"Content-Type": fmt.Sprintf("multipart/form-data; boundary=%s", boundary)},
		Body:      body,
		TimeoutMs: 15000,
	})
	if err != nil {
		return fmt.Errorf("HTTP error posting image: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST /image returned %d: %s", resp.StatusCode, string(resp.Body))
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Tuneshine: sent track '%s' by '%s'", input.Track.Title, input.Track.Artist))
	return nil
}

// Scrobble is a no-op for Tuneshine.
func (t *tuneshine) Scrobble(_ scrobbler.ScrobbleRequest) error {
	return nil
}

func main() {}

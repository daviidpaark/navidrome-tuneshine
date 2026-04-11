// Tuneshine Plugin for Navidrome
//
// Sends album art and track metadata to a Tuneshine device on the LAN
// when a track starts playing, and clears the display when playback ends.
//
// Capabilities: Scrobbler, SchedulerCallback
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/HugoSmits86/nativewebp"
	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/scheduler"
	"github.com/navidrome/navidrome/plugins/pdk/go/scrobbler"
	"golang.org/x/image/draw"
)

const (
	hostKey        = "host"
	serviceNameKey = "servicename"

	payloadClearImage = "clear-image"
	clearScheduleID   = "tuneshine-clear"

	cacheKeyLastTrack = "last-track-id"
	cacheTTLSeconds   = 3600 // 1 hour

	tuneshineSize = 64 // Tuneshine display is 64x64
)

// tuneshine implements the scrobbler and scheduler interfaces.
type tuneshine struct{}

func init() {
	scrobbler.Register(&tuneshine{})
	scheduler.Register(&tuneshine{})
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

// IsAuthorized allows all users.
func (t *tuneshine) IsAuthorized(input scrobbler.IsAuthorizedRequest) (bool, error) {
	pdk.Log(pdk.LogInfo, fmt.Sprintf("Tuneshine: IsAuthorized for user %s", input.Username))
	return true, nil
}

// NowPlaying sends the current track's artwork and metadata to the Tuneshine.
func (t *tuneshine) NowPlaying(input scrobbler.NowPlayingRequest) error {
	deviceHost, serviceName, err := getConfig()
	if err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine config error: %v", err))
		return nil
	}

	// Always cancel any pending clear timer first (handles track skip, replay, etc.)
	_ = host.SchedulerCancelSchedule(clearScheduleID)

	// Check if the track changed \u2014 skip image upload if already displaying this track
	lastID, found, _ := host.CacheGetString(cacheKeyLastTrack)
	sameTrack := found && lastID == input.Track.ID

	if sameTrack {
		pdk.Log(pdk.LogInfo, "Tuneshine: same track, rescheduling clear timer only")
	} else {
		// Fetch and upload new artwork
		if err := uploadTrackImage(input, deviceHost, serviceName); err != nil {
			pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine: %v", err))
			return nil
		}
		_ = host.CacheSetString(cacheKeyLastTrack, input.Track.ID, cacheTTLSeconds)
	}

	// Schedule clearing the display when the track ends.
	remainingSeconds := int32(input.Track.Duration) - input.Position
	if remainingSeconds < 1 {
		remainingSeconds = 1
	}
	_, err = host.SchedulerScheduleOneTime(remainingSeconds, payloadClearImage, clearScheduleID)
	if err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine: failed to schedule clear: %v", err))
	}

	return nil
}

// uploadTrackImage fetches artwork via SubsonicAPI, converts to WebP, and POSTs to the Tuneshine.
func uploadTrackImage(input scrobbler.NowPlayingRequest, deviceHost, serviceName string) error {
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
	metaJSON, _ := json.Marshal(meta)

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

// Scrobble fires when a track is considered "played" \u2014 this can happen mid-playback,
// so we don't clear the display here. The duration-based timer handles clearing.
func (t *tuneshine) Scrobble(_ scrobbler.ScrobbleRequest) error {
	return nil
}

// ============================================================================
// Scheduler Callback Implementation
// ============================================================================

// OnCallback handles scheduler callbacks (clears the Tuneshine display).
func (t *tuneshine) OnCallback(input scheduler.SchedulerCallbackRequest) error {
	if input.Payload != payloadClearImage {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine: unknown callback payload: %s", input.Payload))
		return nil
	}

	deviceHost, _, err := getConfig()
	if err != nil {
		pdk.Log(pdk.LogWarn, fmt.Sprintf("Tuneshine config error: %v", err))
		return nil
	}

	// DELETE /image to revert to idle screen
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

	pdk.Log(pdk.LogInfo, "Tuneshine: cleared display (track ended)")

	// Clear the cached track ID so the next play sends fresh
	_ = host.CacheRemove(cacheKeyLastTrack)

	return nil
}

func main() {}

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/HugoSmits86/nativewebp"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/scrobbler"
)

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestConvertToWebP(t *testing.T) {
	pngData := createTestPNG(120, 120)
	webpData, err := convertToWebP(pngData)
	if err != nil {
		t.Fatalf("convertToWebP failed: %v", err)
	}

	if len(webpData) == 0 {
		t.Fatal("expected non-empty WebP data")
	}

	// Decode back to check dimensions
	decoded, err := nativewebp.Decode(bytes.NewReader(webpData))
	if err != nil {
		t.Fatalf("failed to decode generated WebP: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != tuneshineSize || bounds.Dy() != tuneshineSize {
		t.Errorf("expected dimensions %dx%d, got %dx%d", tuneshineSize, tuneshineSize, bounds.Dx(), bounds.Dy())
	}
}

func TestImageHash(t *testing.T) {
	img1 := createTestPNG(64, 64)
	img2 := createTestPNG(64, 64)
	img3 := createTestPNG(32, 32)

	h1 := imageHash(img1)
	h2 := imageHash(img2)
	h3 := imageHash(img3)

	if h1 != h2 {
		t.Errorf("expected identical hashes for identical images, got %s vs %s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("expected different hashes for different images, both got %s", h1)
	}
}

func TestIsAuthorized(t *testing.T) {
	p := &tuneshine{}

	// Case 1: No user filter configured (allow all)
	pdk.ResetMock()
	pdk.PDKMock.On("GetConfig", userKey).Return("", false)
	auth, err := p.IsAuthorized(scrobbler.IsAuthorizedRequest{Username: "alice"})
	if err != nil || !auth {
		t.Errorf("expected authorized=true, got %v (err: %v)", auth, err)
	}

	// Case 2: Whitelist configured, user matches
	pdk.ResetMock()
	pdk.PDKMock.On("GetConfig", userKey).Return("alice, bob", true)
	auth, err = p.IsAuthorized(scrobbler.IsAuthorizedRequest{Username: "bob"})
	if err != nil || !auth {
		t.Errorf("expected bob to be authorized, got %v", auth)
	}

	// Case 3: Whitelist configured, user not in list
	pdk.ResetMock()
	pdk.PDKMock.On("GetConfig", userKey).Return("alice, bob", true)
	auth, err = p.IsAuthorized(scrobbler.IsAuthorizedRequest{Username: "charlie"})
	if err != nil || auth {
		t.Errorf("expected charlie to be rejected, got %v", auth)
	}
}

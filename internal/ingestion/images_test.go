package ingestion

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestValidateImageAcceptsRealPNGAndRejectsSpoofedMIME(t *testing.T) {
	var encoded bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("png.Encode(): %v", err)
	}

	mime, ext, err := validateImage(encoded.Bytes(), "image/png")
	if err != nil || mime != "image/png" || ext != ".png" {
		t.Fatalf("validateImage() = (%q, %q, %v), want valid PNG", mime, ext, err)
	}
	if _, _, err := validateImage(encoded.Bytes(), "image/jpeg"); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("validateImage() error = %v, want spoofed MIME rejection", err)
	}
}

func TestValidateImageDecodesJPEGWithLargeLeadingMetadata(t *testing.T) {
	var encoded bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 11), G: uint8(y * 13), B: uint8(x * y), A: 255})
		}
	}
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	base := encoded.Bytes()
	segmentLength := 1202
	withMetadata := make([]byte, 0, len(base)+segmentLength)
	withMetadata = append(withMetadata, base[:2]...)
	withMetadata = append(withMetadata, 0xff, 0xe1, byte(segmentLength>>8), byte(segmentLength))
	withMetadata = append(withMetadata, make([]byte, segmentLength-2)...)
	withMetadata = append(withMetadata, base[2:]...)
	if _, _, err := validateImage(withMetadata, "image/jpeg"); err != nil {
		t.Fatalf("valid JPEG with large APP1 segment rejected: %v", err)
	}
}

func TestValidateImageRejectsNonImageData(t *testing.T) {
	if _, _, err := validateImage([]byte("not an image"), "image/png"); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("validateImage() error = %v, want invalid image error", err)
	}
}

func TestSafeStorageKeyAllowsDigestNamesOnly(t *testing.T) {
	if !safeStorageKey.MatchString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.jpg") {
		t.Fatal("valid content-addressed JPEG key rejected")
	}
	for _, key := range []string{"../secret.jpg", "nested/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.png", "0123456789abcdef.jpg"} {
		if safeStorageKey.MatchString(key) {
			t.Errorf("unsafe storage key accepted: %q", key)
		}
	}
}

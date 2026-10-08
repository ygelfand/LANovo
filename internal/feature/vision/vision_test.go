package vision

import (
	"bytes"
	"image/jpeg"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
)

// ESPHome's CameraImageRequest carries no key; the library allows one camera.
func TestTheCameraIsOneEntityAndItTakesPictures(t *testing.T) {
	v := Get()

	ents := v.Entities()
	if len(ents) != 1 {
		t.Fatalf("%d entities, want the one camera", len(ents))
	}

	cam, ok := ents[0].(*esphome.Camera)
	if !ok {
		t.Fatalf("the entity is %T, want a camera", ents[0])
	}
	if cam.ObjectID == "" {
		t.Error("the camera has no object id, so Home Assistant has nothing to call it")
	}
	if cam.Image == nil {
		t.Error("the camera has no Image, so every request answers with nothing")
	}
}

func TestAPictureEncodesWithItsColour(t *testing.T) {
	w, h := 8, 8
	rgba := make([]byte, w*h*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = 220, 30, 30, 255
	}
	out, err := encode(livecam.Picture{RGBA: rgba, Width: w, Height: h})
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(4, 4).RGBA()
	if r>>8 < 150 || g>>8 > 90 || b>>8 > 90 {
		t.Errorf("a red picture came back as %d %d %d", r>>8, g>>8, b>>8)
	}
	if _, err := encode(livecam.Picture{RGBA: rgba[:3], Width: w, Height: h}); err == nil {
		t.Error("a short picture encoded")
	}
}

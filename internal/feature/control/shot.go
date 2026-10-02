package control

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

// DefaultShot is where a screenshot lands when the caller does not say.
const DefaultShot = "/data/misc/lanovo/screen.png"

// shot writes what is on the panel to a PNG and answers with the path.
//
// What is on it rather than what is being drawn: the buffer being drawn into holds the frame
// before last, so reading that would capture a picture nobody saw.
func shot(args []string) (string, error) {
	path := DefaultShot
	if len(args) > 0 {
		path = args[0]
	}

	img, err := composed()
	if err != nil {
		return "", err
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return path, nil
}

// composed is the panel as SurfaceFlinger composed it, GL and video layers included, or lanovod's own
// layer where there is no SurfaceFlinger helper.
func composed() (*image.RGBA, error) {
	if c := display.Get().Helper(); c != nil {
		pix, w, h, err := c.ScreenRead()
		if err != nil {
			return nil, err
		}
		o := display.Get().Orientation()
		vw, vh := o.Size(w, h)
		img := image.NewRGBA(image.Rect(0, 0, vw, vh))
		for vy := range vh {
			for vx := range vw {
				x, y := o.Project(w, h, vx, vy)
				copy(img.Pix[(vy*vw+vx)*4:(vy*vw+vx)*4+4], pix[(y*w+x)*4:(y*w+x)*4+4])
			}
		}
		return img, nil
	}

	pixels, w, h, err := display.Get().Shot()
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range w * h {
		img.Set(i%w, i/w, color.RGBA{R: pixels[i*3], G: pixels[i*3+1], B: pixels[i*3+2], A: 0xff})
	}
	return img, nil
}

// pause waits, for a sequence that has to let something settle or animate.
func pause(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("want milliseconds")
	}

	ms, err := strconv.Atoi(args[0])
	if err != nil || ms < 0 {
		return fmt.Errorf("milliseconds must be a number")
	}

	time.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}

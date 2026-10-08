package control

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const DefaultShot = "/data/misc/lanovo/screen.png"

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

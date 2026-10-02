package visual

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/jpeg"
	"sync"

	xdraw "golang.org/x/image/draw"

	"github.com/ygelfand/LANovo/internal/ui"
)

//go:embed thumbs/*.jpg
var thumbFiles embed.FS

var (
	thumbMu sync.Mutex
	thumbs  = map[string]*ui.Image{}
	fitted  = map[string]*image.RGBA{}
)

func thumbName(k Kind, portrait bool) string {
	if portrait {
		return string(k) + "-portrait"
	}
	return string(k)
}

func Thumbnail(k Kind, portrait bool) *ui.Image {
	name := thumbName(k, portrait)
	thumbMu.Lock()
	defer thumbMu.Unlock()
	if img, ok := thumbs[name]; ok {
		return img
	}
	b, err := thumbFiles.ReadFile("thumbs/" + name + ".jpg")
	var img *ui.Image
	if err == nil {
		img, _ = ui.Decode(b)
	}
	thumbs[name] = img
	return img
}

func ThumbnailFit(k Kind, w, h int) *image.RGBA {
	if w <= 0 || h <= 0 {
		return nil
	}
	name := thumbName(k, h > w)
	key := fmt.Sprintf("%s@%dx%d", name, w, h)
	thumbMu.Lock()
	defer thumbMu.Unlock()
	if img, ok := fitted[key]; ok {
		return img
	}
	var out *image.RGBA
	if b, err := thumbFiles.ReadFile("thumbs/" + name + ".jpg"); err == nil {
		if src, err := jpeg.Decode(bytes.NewReader(b)); err == nil {
			sb := src.Bounds()
			sw, sh := sb.Dx(), sb.Dy()
			if sw*h > sh*w {
				sw = sh * w / h
			} else {
				sh = sw * h / w
			}
			crop := image.Rect(0, 0, sw, sh).Add(sb.Min).Add(image.Pt((sb.Dx()-sw)/2, (sb.Dy()-sh)/2))
			out = image.NewRGBA(image.Rect(0, 0, w, h))
			xdraw.ApproxBiLinear.Scale(out, out.Bounds(), src, crop, xdraw.Src, nil)
		}
	}
	fitted[key] = out
	return out
}

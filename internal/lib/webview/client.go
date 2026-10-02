package webview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const keepaliveEvery = 60 * time.Second

type Client struct {
	conn *websocket.Conn
	wmu  sync.Mutex

	mu     sync.Mutex
	canvas *image.RGBA
	dirty  image.Rectangle
	page   string

	OnFrame func(dirty image.Rectangle)
	OnPage  func(url string)

	decoded, bytes int
	spent          time.Duration
}

func Dial(ctx context.Context, server, page string, opts Options) (*Client, error) {
	uri, err := URI(server, opts)
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("webview: %w", err)
	}
	c := &Client{conn: conn, canvas: image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))}
	if page != "" {
		if err := c.send(openURL(page)); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return c, nil
}

func (c *Client) Run(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() { c.conn.Close() })
	defer stop()
	go c.keepalive(ctx)
	for {
		kind, msg, err := c.conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("webview: %w", err)
		}
		if kind != websocket.BinaryMessage || len(msg) < 2 {
			continue
		}
		switch msg[0] {
		case msgFrame:
			if err := c.frame(msg); err != nil {
				return err
			}
		case msgFrameStats:
			c.mu.Lock()
			avg := uint32(0)
			if c.decoded > 0 {
				avg = uint32(c.spent.Milliseconds() / int64(c.decoded))
			}
			pkt := frameStats(avg, uint32(c.bytes))
			c.decoded, c.bytes, c.spent = 0, 0, 0
			c.mu.Unlock()
			if err := c.send(pkt); err != nil {
				return err
			}
		case msgCurrentURL:
			if u, err := ParseCurrentURL(msg); err == nil {
				c.mu.Lock()
				c.page = u
				c.mu.Unlock()
				if c.OnPage != nil {
					c.OnPage(u)
				}
			}
		}
	}
}

func (c *Client) frame(msg []byte) error {
	f, err := ParseFrame(msg)
	if err != nil {
		return err
	}
	began := time.Now()
	for _, t := range f.Tiles {
		if f.Encoding != JPEG {
			return fmt.Errorf("webview: tiles encoded as %d, want JPEG", f.Encoding)
		}
		img, err := jpeg.Decode(bytes.NewReader(t.Data))
		if err != nil {
			return fmt.Errorf("webview: tile at %d,%d: %w", t.X, t.Y, err)
		}
		r := image.Rect(t.X, t.Y, t.X+t.W, t.Y+t.H)
		c.mu.Lock()
		paste(c.canvas, r, img)
		c.dirty = c.dirty.Union(r.Intersect(c.canvas.Bounds()))
		c.mu.Unlock()
	}
	c.mu.Lock()
	c.bytes += len(msg)
	c.spent += time.Since(began)
	var dirty image.Rectangle
	if f.Flags&FlagLastOfFrame != 0 {
		c.decoded++
		dirty, c.dirty = c.dirty, image.Rectangle{}
	}
	c.mu.Unlock()
	if !dirty.Empty() && c.OnFrame != nil {
		c.OnFrame(dirty)
	}
	return nil
}

func paste(dst *image.RGBA, r image.Rectangle, src image.Image) {
	if y, ok := src.(*image.YCbCr); ok && y.SubsampleRatio == image.YCbCrSubsampleRatio420 {
		yuv420(dst, r, y)
		return
	}
	draw.Draw(dst, r, src, src.Bounds().Min, draw.Src)
}

func yuv420(dst *image.RGBA, r image.Rectangle, src *image.YCbCr) {
	b := src.Bounds()
	r = r.Intersect(dst.Bounds())
	w, h := min(r.Dx(), b.Dx()), min(r.Dy(), b.Dy())
	for y := range h {
		yr := src.Y[(b.Min.Y+y-src.Rect.Min.Y)*src.YStride:]
		cy := ((b.Min.Y+y)/2 - src.Rect.Min.Y/2) * src.CStride
		out := dst.Pix[(r.Min.Y+y-dst.Rect.Min.Y)*dst.Stride+(r.Min.X-dst.Rect.Min.X)*4:]
		for x := range w {
			sx := b.Min.X + x - src.Rect.Min.X
			ci := cy + (b.Min.X+x)/2 - src.Rect.Min.X/2
			yy := int32(yr[sx]) * 0x10101
			cb, cr := int32(src.Cb[ci])-128, int32(src.Cr[ci])-128
			o := out[x*4 : x*4+4 : x*4+4]
			o[0] = clip8((yy + 91881*cr) >> 16)
			o[1] = clip8((yy - 22554*cb - 46802*cr) >> 16)
			o[2] = clip8((yy + 116130*cb) >> 16)
			o[3] = 0xff
		}
	}
}

func clip8(v int32) uint8 {
	if uint32(v) <= 255 {
		return uint8(v)
	}
	if v < 0 {
		return 0
	}
	return 255
}

func (c *Client) View(fn func(canvas *image.RGBA)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c.canvas)
}

func (c *Client) Page() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.page
}

func (c *Client) Touch(kind TouchKind, x, y int) error { return c.send(touch(kind, 0, x, y)) }

func (c *Client) Open(page string) error { return c.send(openURL(page)) }

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) keepalive(ctx context.Context) {
	t := time.NewTicker(keepaliveEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if c.send(keepalive()) != nil {
				return
			}
		}
	}
}

func (c *Client) send(b []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return fmt.Errorf("webview: %w", err)
	}
	return nil
}

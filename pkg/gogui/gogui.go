package gogui

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"
	"time"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/shader"
	"github.com/go-gui-org/go-gui/gui/svg"
)

type Sink interface {
	Program(slot int, vs, fs string) error
	Texture(id uint32, w, h, x, y, rw, rh int, pix []byte) error
	Frame(rot int, clear [4]float32, quads []float32, runs []uint32) (submitUS, finishUS uint32, err error)
}

const (
	slotSolid    = 0
	slotGlyph    = 1
	slotShadow   = 2
	slotGradient = 3
	slotImage    = 4

	quadFloats = 36
	runWords   = 40

	imageIDs = 1 << 24

	touchWait = time.Second
)

const glyphFS = `
    #version 330
    uniform sampler2D tex;
    in vec2 uv;
    in vec4 color;
    in float params;
    out vec4 frag_color;
    void main() {
        frag_color = vec4(color.rgb, color.a * texture(tex, uv).a);
    }
`

type Stats struct {
	Quads, Runs, Bytes int
	Layout, Encode     time.Duration
	Send               time.Duration
	SubmitUS, FinishUS uint32
}

type Renderer struct {
	sink    Sink
	textSys *glyph.TextSystem
	back    *glyphBack

	quads []float32
	runs  []uint32
	slot  uint32
	tex   uint32
	clip  [4]uint32
	open  bool
	rot   int

	clear  *gui.Color
	events chan gui.Event
	wake   chan struct{}

	images    map[string]picture
	mem       map[string]memPicture
	free      []uint32
	frame     uint64
	nextImage uint32
	norm      []gui.GradientStop
	sampled   []gui.GradientStop
}

type picture struct {
	id uint32
	ok bool
}

type memPicture struct {
	id   uint32
	seen uint64
}

func New(sink Sink, w *gui.Window) (*Renderer, error) {
	if err := sink.Program(slotSolid, shader.VsGLSL, shader.FsGLSL); err != nil {
		return nil, err
	}
	if err := sink.Program(slotGlyph, shader.VsGLSL, glyphFS); err != nil {
		return nil, err
	}
	if err := sink.Program(slotShadow, shader.VsShadowGLSL, shader.FsShadowGLSL); err != nil {
		return nil, err
	}
	if err := sink.Program(slotGradient, shader.VsGradientGLSL, shader.FsGradientGLSL); err != nil {
		return nil, err
	}
	if err := sink.Program(slotImage, shader.VsGLSL, shader.FsImageClipGLSL); err != nil {
		return nil, err
	}
	r := &Renderer{sink: sink, images: map[string]picture{}, mem: map[string]memPicture{}, events: make(chan gui.Event, 256), wake: make(chan struct{}, 1)}
	r.back = &glyphBack{r: r, textures: map[glyph.TextureID]*page{}}
	ts, err := glyph.NewTextSystem(r.back)
	if err != nil {
		return nil, fmt.Errorf("gogui: text system: %w", err)
	}
	if data := gui.IconFontData; len(data) > 0 {
		ts.AddFontBytes(data)
	}
	gui.LoadAppFonts(ts, "gogui")
	r.textSys = ts
	w.SetTextMeasurer(&measurer{ts: ts})
	w.SetSvgParser(svg.New())
	return r, nil
}

func (r *Renderer) SetRotation(quarterTurns int) { r.rot = quarterTurns & 3 }

func (r *Renderer) SetClear(c *gui.Color) { r.clear = c }

type Phase int

const (
	Began Phase = iota
	Moved
	Ended
	Cancelled
)

var phases = [...]gui.EventType{gui.EventTouchesBegan, gui.EventTouchesMoved, gui.EventTouchesEnded, gui.EventTouchesCancelled}

func (r *Renderer) Touch(p Phase, id uint64, x, y float32) {
	if p < Began || p > Cancelled {
		return
	}
	e := gui.Event{Type: phases[p], NumTouches: 1}
	e.Touches[0] = gui.TouchPoint{Identifier: id, PosX: x, PosY: y, ToolType: gui.TouchToolFinger, Changed: true}
	if p == Moved {
		select {
		case r.events <- e:
		default:
		}
		return
	}
	select {
	case r.events <- e:
	case <-time.After(touchWait):
	}
}

func (r *Renderer) Type(ch rune) { r.queue(gui.Event{Type: gui.EventChar, CharCode: uint32(ch)}) }

func (r *Renderer) Press(k gui.KeyCode) {
	r.queue(gui.Event{Type: gui.EventKeyDown, KeyCode: k})
	r.queue(gui.Event{Type: gui.EventKeyUp, KeyCode: k})
}

func (r *Renderer) queue(e gui.Event) {
	select {
	case r.events <- e:
	default:
	}
	r.poke()
}

func (r *Renderer) poke() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Renderer) Run(ctx context.Context, w *gui.Window) error {
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	w.SetWakeMainFn(r.poke)
	for {
		for drained := false; !drained; {
			select {
			case e := <-r.events:
				w.EventFn(&e)
			default:
				drained = true
			}
		}
		if w.FrameFn() {
			w.Lock()
			_, err := r.draw(w)
			w.Unlock()
			if err != nil {
				return err
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case e := <-r.events:
			w.EventFn(&e)
		case <-r.wake:
		}
	}
}

func (r *Renderer) run(slot, tex uint32) { r.runWith(slot, tex, nil, nil) }

func (r *Renderer) runWith(slot, tex uint32, tm, tm2 *[16]float32) {
	if tm == nil && r.open && r.slot == slot && r.tex == tex {
		return
	}
	r.slot, r.tex, r.open = slot, tex, tm == nil
	first := uint32(len(r.quads) / quadFloats)
	r.runs = append(r.runs, slot, tex, r.clip[0], r.clip[1], r.clip[2], r.clip[3], first, 0)
	for _, m := range []*[16]float32{tm, tm2} {
		for i := range 16 {
			var v float32
			if m != nil {
				v = m[i]
			}
			r.runs = append(r.runs, math.Float32bits(v))
		}
	}
}

func (r *Renderer) shadow(c *gui.RenderCmd) {
	x, y := c.X+c.OffsetX, c.Y+c.OffsetY
	expand := c.BlurRadius*1.5 + c.Spread
	tm := identityTM()
	tm[12], tm[13], tm[14] = c.OffsetX, c.OffsetY, c.Spread
	r.runWith(slotShadow, 0, &tm, nil)
	v := buildQuad(x-expand, y-expand, c.W+2*expand, c.H+2*expand, c.Color, c.Radius+c.Spread, c.BlurRadius)
	r.push(&v)
}

func (r *Renderer) gradient(c *gui.RenderCmd) {
	if c.Gradient == nil || len(c.Gradient.Stops) == 0 || c.W <= 0 || c.H <= 0 {
		return
	}
	stops := gui.NormalizeGradientStopsInto(c.Gradient.Stops, &r.norm, &r.sampled)
	if len(stops) == 0 {
		return
	}
	tm, tm2 := packGradientUniforms(c.Gradient, stops, c.W, c.H)
	r.runWith(slotGradient, 0, &tm, &tm2)
	v := buildQuad(c.X, c.Y, c.W, c.H, gui.White, c.Radius, 0)
	r.push(&v)
}

func (r *Renderer) imageID() uint32 {
	if n := len(r.free); n > 0 {
		id := r.free[n-1]
		r.free = r.free[:n-1]
		return id
	}
	r.nextImage++
	return imageIDs + r.nextImage
}

func (r *Renderer) memPicture(src string) picture {
	if m, ok := r.mem[src]; ok {
		m.seen = r.frame
		r.mem[src] = m
		return picture{id: m.id, ok: true}
	}
	w, h, pix, ok := gui.LookupImage(src)
	if !ok {
		return picture{}
	}
	m := memPicture{id: r.imageID(), seen: r.frame}
	r.sink.Texture(m.id, w, h, 0, 0, w, h, pix)
	r.mem[src] = m
	return picture{id: m.id, ok: true}
}

func (r *Renderer) sweepMem() {
	for src, m := range r.mem {
		if m.seen != r.frame {
			delete(r.mem, src)
			r.free = append(r.free, m.id)
		}
	}
}

func (r *Renderer) picture(path string) picture {
	if strings.HasPrefix(path, "mem:") {
		return r.memPicture(path)
	}
	if p, ok := r.images[path]; ok {
		return p
	}
	p := picture{}
	if f, err := os.Open(path); err == nil {
		src, _, err := image.Decode(f)
		f.Close()
		if err == nil {
			b := src.Bounds()
			nrgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			draw.Draw(nrgba, nrgba.Bounds(), src, b.Min, draw.Src)
			p = picture{id: r.imageID(), ok: true}
			r.sink.Texture(p.id, b.Dx(), b.Dy(), 0, 0, b.Dx(), b.Dy(), nrgba.Pix)
		}
	}
	r.images[path] = p
	return p
}

func (r *Renderer) image(c *gui.RenderCmd) {
	p := r.picture(c.Resource)
	if !p.ok {
		return
	}
	if c.Color.A > 0 {
		r.quad(c.X, c.Y, c.W, c.H, c.Color, 0, 0)
	}
	r.run(slotImage, p.id)
	v := buildQuad(c.X, c.Y, c.W, c.H, gui.White.WithOpacity(c.Opacity), c.ClipRadius, 0)
	r.push(&v)
}

func (r *Renderer) push(v *[4]vertex) {
	for _, p := range v {
		r.quads = append(r.quads, p.X, p.Y, p.Z, p.U, p.V, p.R, p.G, p.B, p.A)
	}
	r.runs[len(r.runs)-runWords+7]++
}

func (r *Renderer) quad(x, y, w, h float32, c gui.Color, radius, thickness float32) {
	r.run(slotSolid, 0)
	v := buildQuad(x, y, w, h, c, radius, thickness)
	r.push(&v)
}

func (r *Renderer) setClip(x, y, w, h float32) {
	cx, cy, cw, ch := clipRect(x, y, w, h)
	r.clip = [4]uint32{uint32(max(cx, 0)), uint32(max(cy, 0)), uint32(max(cw, 0)), uint32(max(ch, 0))}
	r.open = false
}

func (r *Renderer) line(c *gui.RenderCmd) {
	dx, dy := c.OffsetX-c.X, c.OffsetY-c.Y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 0.001 {
		return
	}
	thick := max(c.Thickness, 1)
	nx, ny := -dy/length*thick*0.5, dx/length*thick*0.5
	cr, cg, cb, ca := normColor(c.Color.R, c.Color.G, c.Color.B, c.Color.A)
	r.run(slotSolid, 0)
	v := [4]vertex{
		{X: c.X + nx, Y: c.Y + ny, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: c.OffsetX + nx, Y: c.OffsetY + ny, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: c.OffsetX - nx, Y: c.OffsetY - ny, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: c.X - nx, Y: c.Y - ny, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
	}
	r.push(&v)
}

const maxTriangleFloats = 1_200_000

func (r *Renderer) triangles(c *gui.RenderCmd) {
	if c.IsClipMask || len(c.Triangles) == 0 || len(c.Triangles)%6 != 0 || len(c.Triangles) > maxTriangleFloats {
		return
	}
	n := len(c.Triangles) / 2
	colored := len(c.VertexColors) == n
	scale := float32(1)
	if c.HasVertexAlpha {
		scale = max(0, min(c.VertexAlphaScale, 1))
	}
	var sinA, cosA float64
	if c.RotAngle != 0 {
		sinA, cosA = math.Sincos(float64(c.RotAngle) * math.Pi / 180)
	}
	at := func(i int) vertex {
		x, y := c.Triangles[i*2], c.Triangles[i*2+1]
		if c.HasXform {
			x, y = x*c.ScaleX+c.TransX, y*c.ScaleY+c.TransY
		}
		if c.RotAngle != 0 {
			dx, dy := float64(x-c.RotCX), float64(y-c.RotCY)
			x, y = c.RotCX+float32(dx*cosA-dy*sinA), c.RotCY+float32(dx*sinA+dy*cosA)
		}
		col := c.Color
		if colored {
			col = c.VertexColors[i]
			if c.HasVertexAlpha {
				col.A = uint8(float32(col.A) * scale)
			}
		}
		cr, cg, cb, ca := normColor(col.R, col.G, col.B, col.A)
		return vertex{X: c.X + x*c.Scale, Y: c.Y + y*c.Scale, R: cr, G: cg, B: cb, A: ca}
	}
	r.run(slotSolid, 0)
	for i := 0; i < n; i += 3 {
		third := at(i + 2)
		v := [4]vertex{at(i), at(i + 1), third, third}
		r.push(&v)
	}
}

func (r *Renderer) Render(w *gui.Window) (Stats, error) {
	t0 := time.Now()
	w.SetHeadlessRender(true)
	w.TestRender(nil)
	st, err := r.draw(w)
	st.Layout = time.Since(t0) - st.Encode - st.Send
	return st, err
}

func (r *Renderer) draw(w *gui.Window) (Stats, error) {
	cmds := w.Renderers()
	t1 := time.Now()

	r.quads, r.runs = r.quads[:0], r.runs[:0]
	r.clip, r.open = [4]uint32{}, false
	r.frame++

	for i := range cmds {
		c := &cmds[i]
		switch c.Kind {
		case gui.RenderClip:
			r.setClip(c.X, c.Y, c.W, c.H)
		case gui.RenderRect:
			if c.Fill {
				r.quad(c.X, c.Y, c.W, c.H, c.Color, c.Radius, 0)
			}
		case gui.RenderStrokeRect:
			r.quad(c.X, c.Y, c.W, c.H, c.Color, c.Radius, c.Thickness)
		case gui.RenderCircle:
			if c.Fill && c.Radius > 0 {
				r.quad(c.X-c.Radius, c.Y-c.Radius, 2*c.Radius, 2*c.Radius, c.Color, c.Radius, 0)
			}
		case gui.RenderLine:
			r.line(c)
		case gui.RenderShadow:
			r.shadow(c)
		case gui.RenderGradient:
			r.gradient(c)
		case gui.RenderImage:
			r.image(c)
		case gui.RenderSvg:
			r.triangles(c)
		case gui.RenderText:
			if len(c.Text) > 0 {
				r.textSys.DrawText(c.X, c.Y, c.Text, textConfigFromRender(c))
			}
		case gui.RenderLayout, gui.RenderLayoutPlaced:
			if c.LayoutPtr != nil {
				r.textSys.DrawLayout(*c.LayoutPtr, c.X, c.Y)
			}
		}
	}
	r.textSys.Commit()
	r.sweepMem()
	t2 := time.Now()

	bg := w.FrameBackground()
	if r.clear != nil {
		bg = *r.clear
	}
	clear := [4]float32{float32(bg.R) / 255, float32(bg.G) / 255, float32(bg.B) / 255, float32(bg.A) / 255}
	sub, fin, err := r.sink.Frame(r.rot, clear, r.quads, r.runs)
	return Stats{
		Quads: len(r.quads) / quadFloats, Runs: len(r.runs) / runWords,
		Bytes: len(r.quads)*4 + len(r.runs)*4, Encode: t2.Sub(t1), Send: time.Since(t2), SubmitUS: sub, FinishUS: fin,
	}, err
}

type page struct {
	id   uint32
	w, h int
}

type glyphBack struct {
	r        *Renderer
	textures map[glyph.TextureID]*page
	next     glyph.TextureID
}

func (b *glyphBack) NewTexture(width, height int) glyph.TextureID {
	b.next++
	b.textures[b.next] = &page{id: uint32(b.next), w: width, h: height}
	b.r.sink.Texture(uint32(b.next), width, height, 0, 0, 0, 0, nil)
	return b.next
}

func (b *glyphBack) UpdateTexture(id glyph.TextureID, data []byte) {
	if p := b.textures[id]; p != nil && len(data) >= p.w*p.h*4 {
		b.r.sink.Texture(p.id, p.w, p.h, 0, 0, p.w, p.h, data[:p.w*p.h*4])
	}
}

func (b *glyphBack) UpdateTextureRect(id glyph.TextureID, data []byte, srcStride, _, y, _, h int) {
	p := b.textures[id]
	if p == nil || h <= 0 || srcStride != p.w*4 {
		return
	}
	y = max(y, 0)
	h = min(h, p.h-y)
	if h <= 0 || len(data) < (y+h)*srcStride {
		return
	}
	b.r.sink.Texture(p.id, p.w, p.h, 0, y, p.w, h, data[y*srcStride:(y+h)*srcStride])
}

func (b *glyphBack) DeleteTexture(id glyph.TextureID) { delete(b.textures, id) }

func (b *glyphBack) DrawTexturedQuad(id glyph.TextureID, src, dst glyph.Rect, c glyph.Color) {
	p := b.textures[id]
	if p == nil {
		return
	}
	cr, cg, cb, ca := normColor(c.R, c.G, c.B, c.A)
	tw, th := float32(p.w), float32(p.h)
	u0, v0 := src.X/tw, src.Y/th
	u1, v1 := (src.X+src.Width)/tw, (src.Y+src.Height)/th
	x0, y0, x1, y1 := dst.X, dst.Y, dst.X+dst.Width, dst.Y+dst.Height
	b.r.run(slotGlyph, p.id)
	v := [4]vertex{
		{X: x0, Y: y0, U: u0, V: v0, R: cr, G: cg, B: cb, A: ca},
		{X: x1, Y: y0, U: u1, V: v0, R: cr, G: cg, B: cb, A: ca},
		{X: x1, Y: y1, U: u1, V: v1, R: cr, G: cg, B: cb, A: ca},
		{X: x0, Y: y1, U: u0, V: v1, R: cr, G: cg, B: cb, A: ca},
	}
	b.r.push(&v)
}

func (b *glyphBack) DrawFilledRect(dst glyph.Rect, c glyph.Color) {
	b.r.quad(dst.X, dst.Y, dst.Width, dst.Height, gui.Color{R: c.R, G: c.G, B: c.B, A: c.A}, 0, 0)
}

func (b *glyphBack) DrawTexturedQuadTransformed(id glyph.TextureID, src, dst glyph.Rect, c glyph.Color, _ glyph.AffineTransform) {
	b.DrawTexturedQuad(id, src, dst, c)
}

func (b *glyphBack) DPIScale() float32 { return 1 }

var _ glyph.RectTextureUpdater = (*glyphBack)(nil)

type measurer struct{ ts *glyph.TextSystem }

func cfg(s gui.TextStyle) glyph.TextConfig { return styleToGlyphConfig(s) }

func (m *measurer) TextWidth(text string, s gui.TextStyle) float32 {
	w, _ := m.ts.TextWidth(text, cfg(s))
	return w
}

func (m *measurer) TextHeight(text string, s gui.TextStyle) float32 {
	h, _ := m.ts.TextHeight(text, cfg(s))
	return h
}

func (m *measurer) FontHeight(s gui.TextStyle) float32 {
	h, err := m.ts.FontHeight(cfg(s))
	if err != nil {
		return s.Size * 1.4
	}
	return h
}

func (m *measurer) FontAscent(s gui.TextStyle) float32 {
	f, err := m.ts.FontMetrics(cfg(s))
	if err != nil {
		return s.Size * 0.8
	}
	return f.Ascender
}

func (m *measurer) LayoutText(text string, s gui.TextStyle, wrap float32) (glyph.Layout, error) {
	c := cfg(s)
	if wrap > 0 {
		c.Block.Width, c.Block.Wrap = wrap, glyph.WrapWord
	} else if wrap < 0 {
		c.Block.Width, c.Block.Wrap = -wrap, glyph.WrapNone
	}
	return m.ts.LayoutText(text, c)
}

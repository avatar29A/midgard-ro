package assetworkbench

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/internal/engine/texture"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

type ActionInfo struct {
	Index      int     `json:"index"`
	Name       string  `json:"name"`
	Frames     int     `json:"frames"`
	IntervalMS float64 `json:"intervalMs"`
}
type Info struct {
	Entry      Entry        `json:"entry"`
	Version    string       `json:"version"`
	ImageCount int          `json:"imageCount"`
	Actions    []ActionInfo `json:"actions"`
	Pair       *Entry       `json:"pair"`
	Warnings   []string     `json:"warnings"`
}
type Dependency struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	SHA256 string `json:"sha256"`
}
type Preview struct {
	Info         Info         `json:"info"`
	Fingerprint  string       `json:"fingerprint"`
	Dependencies []Dependency `json:"dependencies"`
	Renderer     string       `json:"renderer"`
	Action       int          `json:"action"`
	FirstFrame   int          `json:"firstFrame"`
	FrameCount   int          `json:"frameCount"`
	Width        int          `json:"width"`
	Height       int          `json:"height"`
	Columns      int          `json:"columns"`
	OriginX      int          `json:"originX"`
	OriginY      int          `json:"originY"`
	IntervalMS   float64      `json:"intervalMs"`
	ColorKey     bool         `json:"colorKey"`
	PNG          string       `json:"png"`
}

func interval(a *formats.ACT, index int) float64 {
	idx := (index / charsprite.Directions) * charsprite.Directions
	if idx < len(a.Intervals) {
		ms := float64(a.Intervals[idx]) * charsprite.ActIntervalTickMs
		if ms > 0 && !math.IsNaN(ms) && !math.IsInf(ms, 0) {
			return ms
		}
	}
	return 100
}
func (c *Catalog) Inspect(id string, archive int) (Info, error) {
	e, err := c.entry(id, archive)
	if err != nil {
		return Info{}, err
	}
	return c.inspect(e, archive)
}
func (c *Catalog) inspect(e Entry, archive int) (Info, error) {
	out := Info{Entry: e, Actions: []ActionInfo{}, Warnings: []string{}}
	switch e.Type {
	case "spr":
		data, err := c.read(e)
		if err != nil {
			return out, err
		}
		s, err := formats.ParseSPR(data)
		if err != nil {
			return out, err
		}
		out.Version = s.Version.String()
		out.ImageCount = len(s.Images)
		if pair, err := c.pair(e, ".act", archive); err == nil {
			out.Pair = &pair
		}
	case "act":
		data, err := c.read(e)
		if err != nil {
			return out, err
		}
		a, err := formats.ParseACT(data)
		if err != nil {
			return out, err
		}
		out.Version = a.Version.String()
		if len(a.Actions) > 512 {
			return out, fmt.Errorf("too many ACT actions for preview")
		}
		for i, action := range a.Actions {
			out.Actions = append(out.Actions, ActionInfo{i, formats.GetActionName(i, len(a.Actions)), len(action.Frames), interval(a, i)})
		}
		if pair, err := c.pair(e, ".spr", archive); err == nil {
			out.Pair = &pair
		} else {
			out.Warnings = append(out.Warnings, "Соответствующий SPR не найден в выбранном наборе архивов.")
		}
	}
	return out, nil
}
func safeComposite(s *formats.SPR, frame formats.Frame) error {
	left, top, right, bottom := 1<<30, 1<<30, -1<<30, -1<<30
	for _, l := range frame.Layers {
		if l.SpriteID < 0 || int(l.SpriteID) >= len(s.Images) {
			continue
		}
		im := s.Images[l.SpriteID]
		x, y, w, h := int(l.X), int(l.Y), int(im.Width), int(im.Height)
		if x < -8192 || x > 8192 || y < -8192 || y > 8192 {
			return fmt.Errorf("ACT layer offset exceeds preview bounds")
		}
		if x-w/2 < left {
			left = x - w/2
		}
		if y-h/2 < top {
			top = y - h/2
		}
		if x-w/2+w > right {
			right = x - w/2 + w
		}
		if y-h/2+h > bottom {
			bottom = y - h/2 + h
		}
	}
	if right > left && bottom > top && (right-left > 4096 || bottom-top > 4096 || int64(right-left)*int64(bottom-top) > 4<<20) {
		return fmt.Errorf("ACT frame exceeds preview bounds")
	}
	return nil
}
func (c *Catalog) Render(id string, archive, action, frame int, animated, colorKey bool) (Preview, error) {
	if action < 0 || frame < 0 || archive < -1 {
		return Preview{}, fmt.Errorf("invalid preview parameters")
	}
	e, err := c.entry(id, archive)
	if err != nil {
		return Preview{}, err
	}
	info, err := c.inspect(e, archive)
	if err != nil {
		return Preview{}, err
	}
	out := Preview{Info: info, Fingerprint: c.Fingerprint, Dependencies: []Dependency{}, Renderer: "midgard-ro", Action: action, FirstFrame: frame, FrameCount: 1, IntervalMS: 100, ColorKey: colorKey}
	data, err := c.read(e)
	if err != nil {
		return out, err
	}
	out.Dependencies = append(out.Dependencies, Dependency{e.Path, e.Source, hash(data)})
	var frames func(int) (sprite.CompositeResult, error)
	count := 1
	switch e.Type {
	case "spr":
		s, err := formats.ParseSPR(data)
		if err != nil {
			return out, err
		}
		count = len(s.Images)
		frames = func(i int) (sprite.CompositeResult, error) {
			im := s.Images[i]
			return sprite.CompositeResult{Pixels: im.Pixels, Width: int(im.Width), Height: int(im.Height)}, nil
		}
		out.Renderer = "midgard-ro/formats.ParseSPR"
	case "act":
		a, err := formats.ParseACT(data)
		if err != nil {
			return out, err
		}
		if action >= len(a.Actions) {
			return out, fmt.Errorf("ACT action out of range")
		}
		pair, err := c.pair(e, ".spr", archive)
		if err != nil {
			return out, fmt.Errorf("paired SPR not found")
		}
		sd, err := c.read(pair)
		if err != nil {
			return out, err
		}
		out.Dependencies = append(out.Dependencies, Dependency{pair.Path, pair.Source, hash(sd)})
		s, err := formats.ParseSPR(sd)
		if err != nil {
			return out, err
		}
		count = len(a.Actions[action].Frames)
		out.IntervalMS = interval(a, action)
		out.Renderer = "midgard-ro/sprite.CompositeSprites + charsprite.ActIntervalTickMs"
		transformed := false
		for _, f := range a.Actions[action].Frames {
			for _, l := range f.Layers {
				if l.SpriteType != 0 || l.Rotation != 0 || l.ScaleX != 1 || l.ScaleY != 1 || l.Color != [4]uint8{255, 255, 255, 255} {
					transformed = true
				}
			}
		}
		if transformed {
			out.Info.Warnings = append(out.Info.Warnings, "Выбранное ACT содержит RGBA-слои или трансформации. Использован игровой композитор персонажей; специализированный рендер эффектов здесь пока не подключён.")
		}
		frames = func(i int) (sprite.CompositeResult, error) {
			if err := safeComposite(s, a.Actions[action].Frames[i]); err != nil {
				return sprite.CompositeResult{}, err
			}
			return sprite.CompositeSprites(s, a, nil, nil, action/charsprite.Directions, action%charsprite.Directions, i, 0), nil
		}
	default:
		switch e.Type {
		case "bmp", "png", "jpg", "jpeg", "tga":
		default:
			return out, fmt.Errorf("preview for .%s is not implemented", e.Type)
		}
		if e.Type == "tga" {
			if len(data) < 18 {
				return out, fmt.Errorf("truncated TGA")
			}
			if int64(binary.LittleEndian.Uint16(data[12:14]))*int64(binary.LittleEndian.Uint16(data[14:16])) > 8<<20 {
				return out, fmt.Errorf("oversized image")
			}
		} else {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return out, err
			}
			if int64(cfg.Width)*int64(cfg.Height) > 8<<20 {
				return out, fmt.Errorf("oversized image")
			}
		}
		im, err := formats.DecodeImage(data)
		if err != nil {
			return out, err
		}
		rgba := texture.ImageToRGBA(im, colorKey)
		out.Renderer = "midgard-ro/formats.DecodeImage + texture.ImageToRGBA"
		frames = func(i int) (sprite.CompositeResult, error) {
			return sprite.CompositeResult{Pixels: rgba.Pix, Width: rgba.Bounds().Dx(), Height: rgba.Bounds().Dy()}, nil
		}
	}
	if count == 0 || frame >= count {
		return out, fmt.Errorf("frame out of range (count %d)", count)
	}
	n := 1
	if animated {
		n = count - frame
		if n > 256 {
			return out, fmt.Errorf("animation preview is limited to 256 frames; choose a later first frame or a single frame")
		}
	}
	out.FrameCount = n
	left, top, right, bottom := 1<<30, 1<<30, -1<<30, -1<<30
	for i := frame; i < frame+n; i++ {
		r, err := frames(i)
		if err != nil {
			return out, err
		}
		if r.Width == 0 || r.Height == 0 {
			continue
		}
		if r.OffsetX < left {
			left = r.OffsetX
		}
		if r.OffsetY < top {
			top = r.OffsetY
		}
		if r.OffsetX+r.Width > right {
			right = r.OffsetX + r.Width
		}
		if r.OffsetY+r.Height > bottom {
			bottom = r.OffsetY + r.Height
		}
	}
	if right <= left || bottom <= top {
		left = 0
		top = 0
		right = 1
		bottom = 1
	}
	w, h := right-left, bottom-top
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := (n + cols - 1) / cols
	if w > 4096 || h > 4096 || int64(w)*int64(h)*int64(cols)*int64(rows) > 8<<20 {
		return out, fmt.Errorf("preview atlas is too large; request one frame")
	}
	atlas := image.NewNRGBA(image.Rect(0, 0, w*cols, h*rows))
	for j := 0; j < n; j++ {
		r, err := frames(frame + j)
		if err != nil {
			return out, err
		}
		if r.Width == 0 || r.Height == 0 {
			continue
		}
		src := &image.NRGBA{Pix: r.Pixels, Stride: r.Width * 4, Rect: image.Rect(0, 0, r.Width, r.Height)}
		x, y := (j%cols)*w+r.OffsetX-left, (j/cols)*h+r.OffsetY-top
		draw.Draw(atlas, image.Rect(x, y, x+r.Width, y+r.Height), src, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, atlas); err != nil {
		return out, err
	}
	if buf.Len() > 4<<20 {
		return out, fmt.Errorf("preview PNG exceeds transfer limit; request one frame")
	}
	out.Width = w
	out.Height = h
	out.Columns = cols
	out.OriginX = -left
	out.OriginY = -top
	out.PNG = base64.StdEncoding.EncodeToString(buf.Bytes())
	return out, nil
}

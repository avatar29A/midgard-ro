package skillstudio

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"sort"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/assetworkbench"
	"github.com/Faultbox/midgard-ro/internal/engine/effect"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

type strPreview struct {
	art   *formats.STR
	entry assetworkbench.Entry
	dep   assetworkbench.Dependency
}

// renderSTR uses the client's STR evaluator and OpenGL blend pass, without
// substituting a character animation compositor or inventing a skill owner.
func (r *Renderer) renderSTR(req Request) (Frame, error) {
	if r.strPreviews == nil {
		r.strPreviews = map[string]strPreview{}
	}
	cached, ok := r.strPreviews[req.STRResourceID]
	if !ok {
		data, dep, entry, err := r.catalog.ReadID(req.STRResourceID)
		if err != nil {
			return Frame{}, err
		}
		if entry.Type != "str" {
			return Frame{}, fmt.Errorf("resource is not STR")
		}
		art, err := formats.ParseSTR(data)
		if err != nil {
			return Frame{}, err
		}
		if art.DurationMs() <= 0 || art.DurationMs() > 60000 {
			return Frame{}, fmt.Errorf("STR preview supports positive durations up to 60 seconds")
		}
		for _, layer := range art.Layers {
			for _, texture := range layer.Textures {
				if texture != "" {
					if _, err := r.textures.LoadSmooth(`data\texture\effect\` + texture); err != nil {
						return Frame{}, err
					}
				}
			}
		}
		cached = strPreview{art, entry, dep}
		r.strPreviews[req.STRResourceID] = cached
	}
	art, entry, dep := cached.art, cached.entry, cached.dep
	var err error
	r.resources[dep.Path] = dep
	end := ticks(art.DurationMs())
	age := float32(req.Tick) * 1000 / 60
	quads := effect.Frames(art, age)
	if req.Tick >= end {
		quads = nil
	}
	r.final.Bind()
	r.final.Clear(.06, .07, .09, 1)
	r.overlay.Begin()
	scale := float32(190) / req.Camera.Distance
	for _, q := range quads {
		tex, err := r.textures.LoadSmooth(`data\texture\effect\` + q.Texture)
		if err != nil {
			r.overlay.End()
			return Frame{}, err
		}
		corners := q.Corners
		for i := range corners {
			corners[i][0] = Width/2 + (corners[i][0]+req.Camera.PanX)*scale
			corners[i][1] = Height/2 + (corners[i][1]+req.Camera.PanZ)*scale
		}
		r.overlay.DrawImageQuad(tex.ID, corners, q.UV, ui2d.Color{R: q.Color[0], G: q.Color[1], B: q.Color[2], A: q.Color[3]}, q.Additive)
	}
	r.overlay.End()
	gl.Finish()
	pixels := r.final.ReadPixels()
	im := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		copy(im.Pix[y*im.Stride:(y+1)*im.Stride], pixels[(Height-1-y)*Width*4:(Height-y)*Width*4])
	}
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err = enc.Encode(&b, im); err != nil {
		return Frame{}, err
	}
	deps := []assetworkbench.Dependency{}
	for _, d := range r.resources {
		deps = append(deps, d)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
	state := req
	state.Tick = 0
	raw, _ := json.Marshal(state)
	sum := sha256.Sum256(raw)
	phase := "active"
	if req.Tick >= end {
		phase = "finished"
	}
	return Frame{PNG: base64.StdEncoding.EncodeToString(b.Bytes()), Context: map[string]any{
		"schemaVersion": 1, "skillId": 0, "skillName": entry.Path, "effectId": "resource:" + entry.ID, "generator": "client.str", "contractVersion": 1,
		"phase": phase, "tick": req.Tick, "timeMs": age, "tickRate": 60, "scene": req, "sceneDigest": hex.EncodeToString(sum[:]), "definitionDigest": dep.SHA256, "definitionSource": entry.Path,
		"definition": map[string]any{"parameters": map[string]any{"fps": art.FPS, "frames": art.MaxKey, "layers": len(art.Layers), "space": "2D screen pixels", "viewScale": scale}},
		"hitCount":   0, "projectileCount": 0, "particleCount": len(quads), "visibleQuads": len(quads), "durationTicks": end, "impactTicks": []int{},
		"renderer": "midgard-ro: effect.Frames + ui2d.DrawImageQuad", "limitations": []string{"Standalone STR, no actor, cast, procedural siblings or skill sounds. Zoom scales the inspection image."},
		"dependencies": deps, "archiveFingerprint": r.catalog.Fingerprint, "draws": quads, "actors": []any{}, "sourceFrame": age * float32(art.FPS) / 1000,
	}}, nil
}

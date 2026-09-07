// Package skillstudio is an offline viewport, using the game's actor renderer,
// Soul Strike generator, projection and final effect pass.
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
	stdmath "math"
	"runtime"
	"sort"

	"github.com/go-gl/gl/v4.1-core/gl"

	"github.com/Faultbox/midgard-ro/internal/assetworkbench"
	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/engine/character"
	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/framebuffer"
	"github.com/Faultbox/midgard-ro/internal/engine/playerrender"
	"github.com/Faultbox/midgard-ro/internal/engine/scene"
	"github.com/Faultbox/midgard-ro/internal/engine/skillvisual"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/skills"
	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/game/ui"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

const Width, Height = 960, 640

type Camera struct {
	Yaw      float32 `json:"yaw"`
	Distance float32 `json:"distance"`
	PanX     float32 `json:"panX"`
	PanZ     float32 `json:"panZ"`
	Focus    string  `json:"focus"`
}
type Request struct {
	Tick       int     `json:"tick"`
	Level      int     `json:"level"`
	Hits       int     `json:"hits"`       // zero: generated client metadata; otherwise test input
	Separation float32 `json:"separation"` // cells
	Camera     Camera  `json:"camera"`
}

func (r Request) Validate() error {
	if r.Tick < 0 || r.Tick > 600 || r.Level < 1 || r.Level > 10 || r.Hits < 0 || r.Hits > 20 {
		return fmt.Errorf("invalid tick/level/hits")
	}
	for _, v := range []float32{r.Separation, r.Camera.Yaw, r.Camera.Distance, r.Camera.PanX, r.Camera.PanZ} {
		if stdmath.IsNaN(float64(v)) || stdmath.IsInf(float64(v), 0) {
			return fmt.Errorf("non-finite scene value")
		}
	}
	if r.Separation < 1 || r.Separation > 30 || r.Camera.Distance < 100 || r.Camera.Distance > 800 || r.Camera.Yaw < -360 || r.Camera.Yaw > 360 || stdmath.Abs(float64(r.Camera.PanX)) > 200 || stdmath.Abs(float64(r.Camera.PanZ)) > 200 {
		return fmt.Errorf("scene value outside supported bounds")
	}
	if r.Camera.Focus != "center" && r.Camera.Focus != "caster" && r.Camera.Focus != "target" {
		return fmt.Errorf("unknown camera focus")
	}
	return nil
}

type Frame struct {
	PNG     string         `json:"png"`
	Context map[string]any `json:"context"`
}
type Renderer struct {
	root      string
	catalog   *assetworkbench.Catalog
	scene     *scene.Scene
	final     *framebuffer.Framebuffer
	actors    *playerrender.Renderer
	overlay   *ui2d.Renderer
	textures  *ui.TextureCache
	resources map[string]assetworkbench.Dependency
	cache     map[string][]byte
}

var mage = charsprite.Spec{Job: 2, HairStyle: 1}
var rocker = charsprite.Spec{Kind: charsprite.KindMonster, Job: 1052}

func New(root string) (r *Renderer, err error) {
	r = &Renderer{root: root, resources: map[string]assetworkbench.Dependency{}, cache: map[string][]byte{}}
	defer func(created *Renderer) {
		if err != nil {
			created.Close()
		}
	}(r)
	if r.catalog, err = assetworkbench.Open(root); err != nil {
		return nil, err
	}
	if r.overlay, err = ui2d.New(Width, Height); err != nil {
		return nil, err
	}
	r.textures = ui.NewTextureCache(r.overlay, r.load)
	if r.final, err = framebuffer.New(Width, Height); err != nil {
		return nil, err
	}
	if r.scene, err = scene.New(scene.Config{Width: Width, Height: Height, ShadowResolution: 64}); err != nil {
		return nil, err
	}
	r.scene.SetClearColor([3]float32{.07, .08, .10})
	if r.actors, err = playerrender.New(); err != nil {
		return nil, err
	}
	if err = r.actors.LoadCharacter(r.load, mage); err != nil {
		return nil, fmt.Errorf("mage: %w", err)
	}
	// Preload before the first visible frame and reject missing actor art.
	ident := math.Identity()
	r.actors.RenderUnit(ident, &entity.Character{LastCameraSector: -1}, 0, -200, [3]float32{1, 0, 0}, [3]float32{0, 1, 0}, r.load, rocker, 1)
	if w, h := r.actors.UnitQuadSize(rocker); w <= 0 || h <= 0 {
		return nil, fmt.Errorf("rocker SPR/ACT could not be loaded")
	}
	// Preload the generator's actual resource key.
	p := states.NewSoulStrikePreview(1, [3]float32{1, 1, 1}, [3]float32{2, 1, 1}, skillvisual.BuiltInSoulDefinition())
	q := p.QuadsAt(1, func(x, y, z float32) (float32, float32, float32, bool) { return x, y, 1, true })
	if len(q) == 0 {
		return nil, fmt.Errorf("soul strike produced no preload quad")
	}
	if _, err = r.textures.LoadSmooth(q[0].Texture); err != nil {
		return nil, err
	}

	return r, nil
}
func (r *Renderer) load(path string) ([]byte, error) {
	if b, ok := r.cache[path]; ok {
		return b, nil
	}
	b, d, err := r.catalog.ReadPath(path)
	if err == nil {
		r.cache[path] = b
		r.resources[path] = d
	}
	return b, err
}
func (r *Renderer) Close() {
	if r.textures != nil {
		r.textures.Close()
	}
	if r.actors != nil {
		r.actors.Destroy()
	}
	if r.scene != nil {
		r.scene.Destroy()
	}
	if r.final != nil {
		r.final.Destroy()
	}
	if r.overlay != nil {
		r.overlay.Close()
	}
	if r.catalog != nil {
		r.catalog.Close()
	}
}

func (r *Renderer) Render(req Request) (Frame, error) {
	if err := req.Validate(); err != nil {
		return Frame{}, err
	}
	d, digest, source, err := skillvisual.LoadSoulDefinition(r.root)
	if err != nil {
		return Frame{}, err
	}
	hits := req.Hits
	if hits == 0 {
		info, _ := skills.InfoOf(13)
		hits = info.Hits[req.Level-1]
	}
	cam := camera.NewThirdPersonCamera()
	cam.Yaw = req.Camera.Yaw * stdmath.Pi / 180
	cam.Distance = req.Camera.Distance
	distance := req.Separation * entity.CellSize
	x, z := req.Camera.PanX, req.Camera.PanZ
	if req.Camera.Focus == "caster" {
		x -= distance / 2
	}
	if req.Camera.Focus == "target" {
		x += distance / 2
	}
	eye := cam.Position(x, 0, z)
	right, up := character.BillboardVectors(eye.X, eye.Y, eye.Z, x, camera.LookTargetLift, z)
	_, casterH := r.actors.QuadSize()
	_, targetH := r.actors.UnitQuadSize(rocker)
	from := [3]float32{-distance / 2, casterH / 2, 0}
	at := [3]float32{distance / 2, targetH / 2, 0}
	effect := states.NewSoulStrikePreview(hits, from, at, d)
	caster := entity.Character{RenderX: from[0], Direction: entity.DirE, LastCameraSector: -1}
	target := entity.Character{RenderX: at[0], Direction: entity.DirW, LastCameraSector: -1}
	ms := float32(req.Tick) * 1000 / 60
	caster.CurrentFrame = int(ms / max(1, r.actors.FrameInterval(entity.ActionIdle)))
	target.CurrentFrame = int(ms / max(1, r.actors.UnitFrameInterval(rocker, entity.ActionIdle)))
	casterDir, _ := character.CalculateVisualDirection(character.CameraAngleToPlayer(eye.X, eye.Z, caster.RenderX, caster.RenderZ), caster.Direction, -1)
	targetDir, _ := character.CalculateVisualDirection(character.CameraAngleToPlayer(eye.X, eye.Z, target.RenderX, target.RenderZ), target.Direction, -1)
	caster.CurrentFrame %= max(1, r.actors.FrameCount(entity.ActionIdle, casterDir))
	target.CurrentFrame %= max(1, r.actors.UnitFrameCount(rocker, entity.ActionIdle, targetDir))
	tex := r.scene.RenderWithThirdPersonExtras(cam, x, 0, z, func(vp math.Mat4) {
		r.grid(vp)
		drawCaster := func() { r.actors.Render(vp, &caster, eye.X, eye.Z, right, up) }
		drawTarget := func() { r.actors.RenderUnit(vp, &target, eye.X, eye.Z, right, up, r.load, rocker, 1) }
		if eye.X > 0 {
			drawCaster()
			drawTarget()
		} else {
			drawTarget()
			drawCaster()
		}
	})
	vp := r.scene.LastViewProj()
	project := func(x, y, z float32) (float32, float32, float32, bool) {
		sx, sy := scene.ProjectToScreen(vp, x, y, z, Width, Height)
		ux, uy := scene.ProjectToScreen(vp, x, y+1, z, Width, Height)
		per := float32(stdmath.Hypot(float64(ux-sx), float64(uy-sy)))
		return sx, sy, per, sx >= 0 && ux >= 0 && per > 0
	}
	quads := effect.QuadsAt(req.Tick, project)
	r.final.Bind()
	r.final.Clear(.06, .07, .09, 1)
	r.overlay.Begin()
	r.overlay.DrawSceneTexture(0, 0, Width, Height, tex)
	r.overlay.End()
	r.overlay.Begin()
	for _, q := range quads {
		t, err := r.textures.LoadSmooth(q.Texture)
		if err != nil {
			return Frame{}, err
		}
		r.overlay.DrawImageQuad(t.ID, q.Corners, q.UV, ui2d.Color{R: q.Color[0], G: q.Color[1], B: q.Color[2], A: q.Color[3]}, q.Additive)
	}
	r.overlay.End()
	gl.Finish()
	pixels := r.final.ReadPixels()
	// ReadPixels is GL bottom-up despite its legacy comment.
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		copy(img.Pix[y*img.Stride:(y+1)*img.Stride], pixels[(Height-1-y)*Width*4:(Height-y)*Width*4])
	}
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err = enc.Encode(&b, img); err != nil {
		return Frame{}, err
	}
	deps := make([]assetworkbench.Dependency, 0, len(r.resources))
	for _, d := range r.resources {
		deps = append(deps, d)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
	sceneState := req
	sceneState.Tick = 0
	sceneBytes, _ := json.Marshal(sceneState)
	sum := sha256.Sum256(sceneBytes)
	return Frame{PNG: base64.StdEncoding.EncodeToString(b.Bytes()), Context: map[string]any{
		"schemaVersion": 1, "goVersion": runtime.Version(), "gpu": gl.GoStr(gl.GetString(gl.RENDERER)), "glVersion": gl.GoStr(gl.GetString(gl.VERSION)), "generatorSource": "internal/game/states/burst.go:soulStrikePartsWithDefinition", "skillId": 13, "effectId": d.ID, "generator": d.Generator, "contractVersion": d.ContractVersion,
		"phase": "volley", "tick": req.Tick, "timeMs": ms, "tickRate": 60, "seed": 0, "randomness": "none: analytical generator",
		"scene": req, "sceneDigest": hex.EncodeToString(sum[:]), "definition": d, "definitionDigest": digest, "definitionSource": source,
		"hitCount": hits, "projectileCount": min(hits, 5), "particleCount": effect.ParticleCount(), "visibleQuads": len(quads),
		"durationTicks": int(stdmath.Ceil(float64(effect.DurationMS()) * 60 / 1000)), "impactTicks": states.SoulStrikeImpactTicks(hits),
		"actors":       []any{map[string]any{"role": "caster", "name": "Mage", "feet": [3]float32{from[0], 0, 0}, "anchor": from, "direction": caster.Direction, "action": "idle", "frame": caster.CurrentFrame, "visualDirection": casterDir}, map[string]any{"role": "target", "name": "Rocker", "feet": [3]float32{at[0], 0, 0}, "anchor": at, "direction": target.Direction, "action": "idle", "frame": target.CurrentFrame, "visualDirection": targetDir}},
		"viewport":     map[string]any{"width": Width, "height": Height, "dpi": 1, "pitch": cam.Pitch, "viewProjection": vp},
		"dependencies": deps, "archiveFingerprint": r.catalog.Fingerprint,
		"renderer":    "midgard-ro: scene + playerrender + burst.quadsAt + ui2d.DrawImageQuad",
		"limitations": []string{"Volley's flight, tails and flashes only; cast aura, sound and target reaction are next stage", "Neutral diagnostic grid; map occlusion is not tested"},
	}}, nil
}

func (r *Renderer) grid(vp math.Mat4) {
	r.overlay.Begin()
	for x := -20; x < 20; x++ {
		for z := -20; z < 20; z++ {
			var q [4][2]float32
			ok := true
			for i, p := range [4][2]int{{x, z}, {x + 1, z}, {x + 1, z + 1}, {x, z + 1}} {
				q[i][0], q[i][1] = scene.ProjectToScreen(vp, float32(p[0])*entity.CellSize, 0, float32(p[1])*entity.CellSize, Width, Height)
				if q[i][0] < 0 {
					ok = false
				}
			}
			if ok {
				v := float32(.12)
				if (x+z)%2 == 0 {
					v = .15
				}
				r.overlay.DrawQuad(q[0], q[1], q[2], q[3], ui2d.Color{R: v, G: v + .01, B: v + .02, A: 1})
			}
		}
	}
	r.overlay.End()
}

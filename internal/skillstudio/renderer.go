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
	"github.com/Faultbox/midgard-ro/internal/network/packets"
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
type Ground struct {
	Anchor string `json:"anchor"`
	Cells  int    `json:"cells"`
	Angle  int    `json:"angle"`
}
type Request struct {
	STRResourceID    string    `json:"strResourceId,omitempty"`
	LibraryEntryID   string    `json:"libraryEntryId,omitempty"`
	LibraryRevision  string    `json:"libraryRevision,omitempty"`
	TargetStatus     string    `json:"targetStatus,omitempty"`
	StatusDelayMS    int       `json:"statusDelayMs,omitempty"`
	SkillID          uint16    `json:"skillId,omitempty"`
	EffectDurationMS int       `json:"effectDurationMs,omitempty"`
	Ground           *Ground   `json:"ground,omitempty"`
	Tick             int       `json:"tick"`
	Level            int       `json:"level"`
	Hits             int       `json:"hits"`       // zero: generated client metadata; otherwise test input
	Separation       float32   `json:"separation"` // cells
	Camera           Camera    `json:"camera"`
	Sequence         *Sequence `json:"sequence,omitempty"`
}

func (r Request) skill() uint16 {
	if r.SkillID == 0 {
		return 13
	}
	return r.SkillID
}
func (r Request) lifetime() int {
	if r.EffectDurationMS == 0 {
		return 3000
	}
	return r.EffectDurationMS
}
func (r Request) cells() int {
	if r.skill() != 18 {
		return 1
	}
	if r.Ground != nil {
		return r.Ground.Cells
	}
	return 3
}
func (r Request) Validate() error {
	if r.STRResourceID != "" {
		b, err := hex.DecodeString(r.STRResourceID)
		if err != nil || len(b) != 32 {
			return fmt.Errorf("invalid STR resource identity")
		}
		if r.Sequence != nil {
			return fmt.Errorf("raw STR preview does not have a skill sequence")
		}
	}
	if len(r.LibraryEntryID) > 200 {
		return fmt.Errorf("invalid library entry")
	}
	if r.TargetStatus != "" && r.TargetStatus != "none" {
		if (r.skill() != 15 || r.TargetStatus != "frozen") && (r.skill() != 16 || r.TargetStatus != "stone") {
			return fmt.Errorf("unsupported target status for skill")
		}
	}
	if r.StatusDelayMS < 0 || r.StatusDelayMS > 10000 {
		return fmt.Errorf("invalid status delay")
	}

	if _, ok := skillvisual.PreviewOf(r.skill()); !ok {
		return fmt.Errorf("unsupported preview skill %d", r.skill())
	}
	if r.EffectDurationMS != 0 && (r.EffectDurationMS < 100 || r.EffectDurationMS > 30000) {
		return fmt.Errorf("invalid effect lifetime")
	}
	if r.Ground != nil {
		g := r.Ground
		if (g.Anchor != "caster" && g.Anchor != "target") || (g.Cells != 1 && g.Cells != 3) || (g.Angle != 0 && g.Angle != 90) {
			return fmt.Errorf("invalid ground fixture")
		}
		if r.skill() != 18 && g.Cells != 1 {
			return fmt.Errorf("this skill uses one anchor")
		}
	}

	if s := r.Sequence; s != nil {
		if s.CastMS < -1 || s.CastMS > 6000 {
			return fmt.Errorf("invalid cast duration")
		}
		if s.CancelTick != nil && (*s.CancelTick < 0 || *s.CancelTick >= ticks(float32(castDurationFor(r.skill(), r.Level, s)))) {
			return fmt.Errorf("cancellation must be before cast release")
		}
	}
	if r.Tick < 0 || r.Tick > 3600 || r.Level < 1 || r.Level > 10 || r.Hits < 0 || r.Hits > 20 {
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
	strPreviews   map[string]strPreview
	root          string
	catalog       *assetworkbench.Catalog
	scene         *scene.Scene
	aura          *scene.GroundMarker
	soundWarnings []string
	soundChecked  map[string]bool
	persistent    map[uint16]*states.PersistentPreview
	mage          map[string]*states.MagePreview
	final         *framebuffer.Framebuffer
	actors        *playerrender.Renderer
	overlay       *ui2d.Renderer
	textures      *ui.TextureCache
	resources     map[string]assetworkbench.Dependency
	cache         map[string][]byte
}

var mage = charsprite.Spec{Job: 2, HairStyle: 1}
var rocker = charsprite.Spec{Kind: charsprite.KindMonster, Job: 1052}

func New(root string) (r *Renderer, err error) {
	r = &Renderer{root: root, resources: map[string]assetworkbench.Dependency{}, cache: map[string][]byte{}, persistent: map[uint16]*states.PersistentPreview{}, soundChecked: map[string]bool{}, mage: map[string]*states.MagePreview{}}
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
	if r.aura != nil {
		r.aura.Destroy()
	}
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
	if req.STRResourceID != "" {
		return r.renderSTR(req)
	}
	adapter, _ := skillvisual.PreviewOf(req.skill())
	var definition any
	var digest, source string
	var err error
	hits := req.Hits
	if skillvisual.IsPersistent(req.skill()) || req.skill() == 16 {
		hits = 0
	} else if hits == 0 {
		info, _ := skills.InfoOf(req.skill())
		hits, _ = skills.At(info.Hits, req.Level)
		hits = max(hits, 1)
	}
	var mage *states.MagePreview
	var soulDefinition skillvisual.SoulDefinition
	if req.skill() == 13 {
		soulDefinition, digest, source, err = skillvisual.LoadSoulDefinition(r.root)
		if err != nil {
			return Frame{}, err
		}
		definition = soulDefinition
		p := states.NewSoulStrikePreview(1, [3]float32{1, 1, 1}, [3]float32{2, 1, 1}, soulDefinition)
		for _, q := range p.QuadsAt(1, func(x, y, z float32) (float32, float32, float32, bool) { return x, y, 1, true }) {
			if _, err = r.textures.LoadSmooth(q.Texture); err != nil {
				return Frame{}, err
			}
		}
	} else if skillvisual.IsMageAttack(req.skill()) {
		key := fmt.Sprintf("%d/%d", req.skill(), hits)
		mage = r.mage[key]
		if mage == nil {
			mage, err = states.LoadMagePreview(req.skill(), hits, r.load)
			if err != nil {
				return Frame{}, err
			}
			for _, path := range mage.Textures() {
				if _, err = r.textures.LoadSmooth(path); err != nil {
					return Frame{}, err
				}
			}
			r.mage[key] = mage
		}
		if req.TargetStatus == "frozen" {
			if _, err = r.textures.LoadSmooth(states.FrozenPreviewTexture()); err != nil {
				return Frame{}, err
			}
		}
		definition = map[string]any{"schemaVersion": 1, "id": adapter.ID, "generator": adapter.Generator, "contractVersion": 1, "parameters": map[string]any{"effectDurationMs": mage.DurationMS, "projectiles": mage.ProjectileCount, "events": len(mage.Cues), "routing": "client targetVisualPlan"}}
		data, _ := json.Marshal(definition)
		sum := sha256.Sum256(data)
		digest = hex.EncodeToString(sum[:])
		source = adapter.Source
	} else {
		p := r.persistent[req.skill()]
		if p == nil {
			p, err = states.LoadPersistentPreview(req.skill(), r.load)
			if err != nil {
				return Frame{}, err
			}
			for _, path := range p.Textures() {
				if _, err = r.textures.LoadSmooth(path); err != nil {
					return Frame{}, err
				}
			}
			r.persistent[req.skill()] = p
		}
		definition = map[string]any{"schemaVersion": 1, "id": adapter.ID, "generator": adapter.Generator, "contractVersion": 1, "parameters": p.Parameters()}
		data, _ := json.Marshal(definition)
		sum := sha256.Sum256(data)
		digest = hex.EncodeToString(sum[:])
		source = adapter.Source
	}
	if req.Sequence != nil && castDurationFor(req.skill(), req.Level, req.Sequence) > 0 && r.aura == nil {
		aura, err := scene.NewTube(scene.CastAuraTexture, scene.CastAuraTint, scene.CastAuraSides)
		if err != nil {
			return Frame{}, err
		}
		if err = aura.LoadTexture(r.load); err != nil {
			aura.Destroy()
			return Frame{}, err
		}
		r.aura = aura
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
	var effect *states.SoulStrikePreview
	var tape Timeline
	if req.skill() == 13 {
		effect = states.NewSoulStrikePreview(hits, from, at, soulDefinition)
		tape = makeTimeline(req.Sequence, hits, ticks(effect.DurationMS()), states.AttackHitDelayMS(r.actors.HitFrame(entity.ActionAttack), r.actors.FrameCount(entity.ActionAttack, entity.DirE), packets.SwingReferenceMs))
	} else if mage != nil {
		tape = mageTimeline(req, mage, states.MageReactionDelay(req.skill(), states.AttackHitDelayMS(r.actors.HitFrame(entity.ActionAttack), r.actors.FrameCount(entity.ActionAttack, entity.DirE), packets.SwingReferenceMs)))
	} else {
		tape = persistentTimeline(req)
	}
	for _, event := range tape.Events {
		path := event.Sound
		if path == "" || r.soundChecked[path] {
			continue
		}
		r.soundChecked[path] = true
		if _, err := r.load(path); err != nil {
			r.soundWarnings = append(r.soundWarnings, err.Error())
		}
	}
	anchors := [][3]float32{}
	if req.skill() == 10 {
		anchors = append(anchors, [3]float32{from[0], 0, from[2]})
	} else if skillvisual.IsPersistent(req.skill()) || req.skill() == 21 {
		center := [3]float32{at[0], 0, at[2]}
		if req.Ground != nil && req.Ground.Anchor == "caster" {
			center = [3]float32{from[0], 0, from[2]}
		}
		for i := 0; i < req.cells(); i++ {
			point := center
			offset := float32(i-req.cells()/2) * entity.CellSize
			if req.Ground != nil && req.Ground.Angle == 90 {
				point[0] += offset
			} else {
				point[2] += offset
			}
			anchors = append(anchors, point)
		}
	}
	caster := entity.Character{RenderX: from[0], Direction: entity.DirE, LastCameraSector: -1}
	target := entity.Character{RenderX: at[0], Direction: entity.DirW, LastCameraSector: -1}
	ms := float32(req.Tick) * 1000 / 60
	caster.CurrentFrame = int(ms / max(1, r.actors.FrameInterval(entity.ActionIdle)))
	target.CurrentFrame = int(ms / max(1, r.actors.UnitFrameInterval(rocker, entity.ActionIdle)))
	casterDir, _ := character.CalculateVisualDirection(character.CameraAngleToPlayer(eye.X, eye.Z, caster.RenderX, caster.RenderZ), caster.Direction, -1)
	targetDir, _ := character.CalculateVisualDirection(character.CameraAngleToPlayer(eye.X, eye.Z, target.RenderX, target.RenderZ), target.Direction, -1)
	caster.CurrentFrame %= max(1, r.actors.FrameCount(entity.ActionIdle, casterDir))
	target.CurrentFrame %= max(1, r.actors.UnitFrameCount(rocker, entity.ActionIdle, targetDir))
	if req.Sequence != nil {
		// Start a settled idle and replay the game's animation state machine.
		// World-facing frame counts make camera rotation independent of simulation.
		caster.AdvanceAnimation(entity.WalkHoldMs, 1, 1, 1, 1, 1)
		caster.CurrentFrame = 0
		caster.FrameTime = 0
		target.AdvanceAnimation(entity.WalkHoldMs, 1, 1, 1, 1, 1)
		target.CurrentFrame = 0
		target.FrameTime = 0
		for action := 0; action < entity.LoadedActions; action++ {
			target.AnimIntervalMs[action] = r.actors.UnitFrameInterval(rocker, action)
		}
		advance := func(body *entity.Character, player bool) {
			count := func(action int) int {
				if player {
					return r.actors.FrameCount(action, body.Direction)
				}
				return r.actors.UnitFrameCount(rocker, action, body.Direction)
			}
			once := 0
			if playing := body.PlayingAction(); playing >= 0 {
				once = count(playing)
			}
			body.AdvanceAnimation(1000.0/60, count(entity.ActionIdle), count(entity.ActionWalk), once, count(entity.ActionStandby), count(entity.ActionSit))
		}
		for tick := 0; tick <= req.Tick; tick++ {
			if tick > 0 {
				advance(&caster, true)
				advance(&target, false)
			}
			for _, hit := range tape.ReactionTicks {
				if hit == tick {
					target.PlayHurt()
				}
			}
		}
	}
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
		if req.Sequence != nil && req.Tick < tape.ReleaseTick && (tape.CancelTick == nil || req.Tick < *tape.CancelTick) {
			shape := states.CastAuraAt(ms, float32(tape.CastMS))
			r.aura.RenderTube(vp, from[0], 0, 0, shape.Bottom, shape.Top, shape.Height, shape.Alpha)
		}
	})
	vp := r.scene.LastViewProj()
	project := func(x, y, z float32) (float32, float32, float32, bool) {
		sx, sy := scene.ProjectToScreen(vp, x, y, z, Width, Height)
		ux, uy := scene.ProjectToScreen(vp, x, y+1, z, Width, Height)
		per := float32(stdmath.Hypot(float64(ux-sx), float64(uy-sy)))
		return sx, sy, per, sx >= 0 && ux >= 0 && per > 0
	}
	var quads []states.EffectQuad
	particleCount, projectileCount := 0, 0
	if req.skill() == 13 {
		quads = effect.QuadsAt(req.Tick-tape.ReleaseTick, project)
		particleCount = effect.ParticleCount()
		projectileCount = min(hits, 5)
	} else if mage != nil {
		ground := [3]float32{at[0], 0, at[2]}
		if len(anchors) > 0 {
			ground = anchors[0]
		}
		if req.Tick >= tape.ReleaseTick && req.Tick < tape.EndTick {
			quads = mage.QuadsAt(float32(req.Tick-tape.ReleaseTick)*1000/60, from, at, ground, project)
		}
		if req.TargetStatus == "frozen" && req.Tick >= tape.ReleaseTick+ticks(float32(req.StatusDelayMS)) && req.Tick < tape.ReleaseTick+ticks(float32(req.StatusDelayMS))+ticks(float32(req.lifetime())) {
			quads = append(quads, states.FrozenPreviewQuads([3]float32{at[0], 0, at[2]}, targetH, project)...)
		}
		projectileCount = mage.ProjectileCount
		particleCount = len(quads)
	} else if req.Tick >= tape.ReleaseTick && req.Tick < tape.EndTick {
		for _, anchor := range anchors {
			quads = append(quads, r.persistent[req.skill()].QuadsAt(float32(req.Tick-tape.ReleaseTick)*1000/60, anchor, project)...)
		}
		particleCount = len(quads)
	}
	if tape.CancelTick != nil {
		quads = nil
		particleCount = 0
		projectileCount = 0
	}
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
	instances := []map[string]any{}
	for i, anchor := range anchors {
		role := "ground"
		if req.skill() == 10 {
			role = "caster"
		}
		instances = append(instances, map[string]any{"id": fmt.Sprintf("instance-%d", i), "role": role, "anchor": anchor, "active": tape.CancelTick == nil && req.Tick >= tape.ReleaseTick && req.Tick < tape.EndTick})
	}
	bodyStatus := "none"
	if tape.CancelTick == nil && req.TargetStatus != "" && req.TargetStatus != "none" {
		start := tape.ReleaseTick + ticks(float32(req.StatusDelayMS))
		if req.Tick >= start && req.Tick < start+ticks(float32(req.lifetime())) {
			bodyStatus = req.TargetStatus
		}
	}
	warnings := []string{"Mage has no casting pose in this client. No server combat, collisions or map occlusion are simulated."}
	if mage != nil {
		warnings = append(warnings, mage.Warnings...)
	}
	if req.skill() == 12 {
		warnings = append(warnings, "STR uses client screen-pixel sizing, not world-billboard scaling.")
	}
	return Frame{PNG: base64.StdEncoding.EncodeToString(b.Bytes()), Context: map[string]any{
		"schemaVersion": 1, "goVersion": runtime.Version(), "gpu": gl.GoStr(gl.GetString(gl.RENDERER)), "glVersion": gl.GoStr(gl.GetString(gl.VERSION)), "generatorSource": adapter.Source, "skillId": req.skill(), "skillName": skills.Name(req.skill()), "effectId": adapter.ID, "generator": adapter.Generator, "contractVersion": 1,
		"phase": tape.phase(req.Tick), "timeline": tape, "soundWarnings": r.soundWarnings, "tick": req.Tick, "timeMs": ms, "tickRate": 60, "seed": 0, "randomness": "none: analytical generator",
		"scene": req, "sceneDigest": hex.EncodeToString(sum[:]), "definition": definition, "definitionDigest": digest, "definitionSource": source,
		"hitCount": hits, "projectileCount": projectileCount, "particleCount": particleCount, "visibleQuads": len(quads),
		"durationTicks": tape.DurationTicks, "impactTicks": tape.ImpactTicks,
		"actors":        []any{map[string]any{"role": "caster", "name": "Mage", "feet": [3]float32{from[0], 0, 0}, "anchor": from, "direction": caster.Direction, "action": actionName(caster.CurrentAction), "frame": caster.CurrentFrame % max(1, r.actors.FrameCount(caster.CurrentAction, casterDir)), "simulationFrame": caster.CurrentFrame, "visualDirection": casterDir}, map[string]any{"role": "target", "name": "Rocker", "bodyStatus": bodyStatus, "bodyStatusRendered": bodyStatus == "frozen", "feet": [3]float32{at[0], 0, 0}, "anchor": at, "direction": target.Direction, "action": actionName(target.CurrentAction), "frame": target.CurrentFrame % max(1, r.actors.UnitFrameCount(rocker, target.CurrentAction, targetDir)), "simulationFrame": target.CurrentFrame, "visualDirection": targetDir}},
		"effectAnchors": anchors, "instances": instances, "draws": quads,
		"viewport":     map[string]any{"width": Width, "height": Height, "dpi": 1, "pitch": cam.Pitch, "viewProjection": vp},
		"dependencies": deps, "archiveFingerprint": r.catalog.Fingerprint,
		"renderer":    "midgard-ro: scene + playerrender + shared effect evaluators + ui2d.DrawImageQuad",
		"limitations": warnings,
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

func actionName(action int) string {
	if action == entity.ActionHurt {
		return "hurt"
	}
	if action == entity.ActionWalk {
		return "walk"
	}
	return "idle"
}

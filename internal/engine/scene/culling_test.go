package scene

import (
	gomath "math"
	"testing"

	rsmmodel "github.com/Faultbox/midgard-ro/internal/engine/model"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

func verticesAt(positions ...[3]float32) []rsmmodel.Vertex {
	vertices := make([]rsmmodel.Vertex, len(positions))
	for i, p := range positions {
		vertices[i] = rsmmodel.Vertex{Position: p}
	}
	return vertices
}

// TestBoundingSphereEnclosesEveryVertex is the property that keeps culling
// honest. A sphere even slightly too small culls a model that is still partly
// on screen, which shows up as buildings popping in and out at the edge of
// view — so this errs on the side of too large, never too small.
func TestBoundingSphereEnclosesEveryVertex(t *testing.T) {
	tests := []struct {
		name     string
		vertices []rsmmodel.Vertex
	}{
		{
			name:     "cube around the origin",
			vertices: verticesAt([3]float32{-1, -1, -1}, [3]float32{1, 1, 1}, [3]float32{-1, 1, -1}, [3]float32{1, -1, 1}),
		},
		{
			name: "tall thin wall, the shape most of a town is made of",
			vertices: verticesAt(
				[3]float32{-15, 0, -1}, [3]float32{15, 0, -1},
				[3]float32{-15, 40, 1}, [3]float32{15, 40, 1},
			),
		},
		{
			name:     "off-center cluster",
			vertices: verticesAt([3]float32{100, 200, 300}, [3]float32{110, 205, 290}, [3]float32{105, 100, 295}),
		},
		{
			name:     "single vertex",
			vertices: verticesAt([3]float32{7, 8, 9}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			center, radius := boundingSphere(tt.vertices)

			for i, v := range tt.vertices {
				dx := float64(v.Position[0] - center[0])
				dy := float64(v.Position[1] - center[1])
				dz := float64(v.Position[2] - center[2])
				dist := gomath.Sqrt(dx*dx + dy*dy + dz*dz)
				if dist > float64(radius)+1e-4 {
					t.Errorf("vertex %d is %v from the center, outside radius %v", i, dist, radius)
				}
			}
		})
	}
}

func TestBoundingSphereOfNothing(t *testing.T) {
	center, radius := boundingSphere(nil)
	if radius != 0 || center != [3]float32{} {
		t.Errorf("boundingSphere(nil) = %v, %v; want zero", center, radius)
	}
}

// TestWorldBoundsFollowScale covers the reason the world sphere is computed
// separately from the local one: a scaled-up instance covers more ground than
// its mesh does, and culling it by the unscaled radius would clip it early.
func TestWorldBoundsFollowScale(t *testing.T) {
	mr := &ModelRenderer{mapWidth: 0, mapHeight: 0}
	mr.models = []*MapModel{{
		position:    [3]float32{0, 0, 0},
		rotation:    [3]float32{0, 0, 0},
		scale:       [3]float32{1, 3, 1}, // non-uniform: the tallest axis wins
		localCenter: [3]float32{0, 0, 0},
		localRadius: 10,
	}}

	mr.computeWorldBounds()

	if got := mr.models[0].worldRadius; got != 30 {
		t.Errorf("worldRadius = %v, want 30 (10 x the largest scale axis)", got)
	}
}

// TestWorldBoundsHandleNegativeScale guards a real case in the map data:
// instances are mirrored with a negative scale, and taking that at face value
// would give a negative radius, which culls the model from every angle.
func TestWorldBoundsHandleNegativeScale(t *testing.T) {
	mr := &ModelRenderer{}
	mr.models = []*MapModel{{
		scale:       [3]float32{-2, 1, 1},
		localRadius: 5,
	}}

	mr.computeWorldBounds()

	if got := mr.models[0].worldRadius; got != 10 {
		t.Errorf("worldRadius = %v, want 10; a mirrored instance is still 2x as wide", got)
	}
}

// TestWorldBoundsPlaceCenterOnTheMap checks the offset every model position is
// shifted by, since the RSW stores coordinates relative to the map center.
func TestWorldBoundsPlaceCenterOnTheMap(t *testing.T) {
	mr := &ModelRenderer{mapWidth: 400, mapHeight: 600}
	mr.models = []*MapModel{{
		position:    [3]float32{10, 0, 20},
		scale:       [3]float32{1, 1, 1},
		localCenter: [3]float32{0, 0, 0},
		localRadius: 1,
	}}

	mr.computeWorldBounds()

	// 10 + 400/2 = 210, 20 + 600/2 = 320.
	if got := mr.models[0].worldCenter; got[0] != 210 || got[2] != 320 {
		t.Errorf("worldCenter = %v, want x=210 z=320", got)
	}
}

// TestCullingKeepsWhatTheCameraLooksAt ties the two halves together: bounds
// built by computeWorldBounds, tested against a frustum built the way Render
// builds one.
func TestCullingKeepsWhatTheCameraLooksAt(t *testing.T) {
	mr := &ModelRenderer{mapWidth: 0, mapHeight: 0}
	mr.models = []*MapModel{
		{position: [3]float32{0, 0, 0}, scale: [3]float32{1, 1, 1}, localRadius: 5},
		{position: [3]float32{0, 0, -900}, scale: [3]float32{1, 1, 1}, localRadius: 5},
	}
	mr.computeWorldBounds()

	proj := math.Perspective(45*gomath.Pi/180, 16.0/9.0, 1, 500)
	view := math.LookAt(
		math.Vec3{X: 0, Y: 0, Z: 100},
		math.Vec3{X: 0, Y: 0, Z: 0},
		math.Vec3{X: 0, Y: 1, Z: 0},
	)
	frustum := FrustumFromViewProj(proj.Mul(view))

	// buildModelMatrix negates Y and leaves X/Z, so the near model sits at the
	// focal point and the far one well past the 500-unit far plane.
	if !frustum.ContainsSphere(mr.models[0].worldCenter, mr.models[0].worldRadius) {
		t.Error("culled the model the camera is pointed at")
	}
	if frustum.ContainsSphere(mr.models[1].worldCenter, mr.models[1].worldRadius) {
		t.Error("kept a model 1000 units away, past the far plane")
	}
}

// TestCullingOnATownSizedMap is the test that says whether any of this was
// worth doing. It reproduces the real camera — 45-degree field of view, the
// RO-style 48-degree downward pitch, the default zoom — over a map the size of
// Prontera, and sweeps it through a full rotation.
//
// Two things have to hold at every angle. Culling has to remove most of the
// map, or the work saved does not pay for the test. And it has to leave
// something standing, since a bug that culls everything shows up as an empty
// world, which no unit test on hand-picked points would catch.
func TestCullingOnATownSizedMap(t *testing.T) {
	// Prontera is 312x392 cells, and a cell is 5 world units.
	const mapW, mapH = 312 * 5.0, 392 * 5.0

	mr := &ModelRenderer{mapWidth: mapW, mapHeight: mapH}
	for x := float32(-mapW / 2); x < mapW/2; x += 40 {
		for z := float32(-mapH / 2); z < mapH/2; z += 40 {
			mr.models = append(mr.models, &MapModel{
				position:    [3]float32{x, 0, z},
				scale:       [3]float32{1, 1, 1},
				localRadius: 20, // roughly a wall segment
			})
		}
	}
	mr.computeWorldBounds()

	const (
		fov      = 0.785398 // matches Scene.RenderWithViewExtras
		near     = 1.0
		far      = 10000.0
		aspect   = 1280.0 / 720.0
		pitch    = 0.85 // NewThirdPersonCamera's RO-style top-down angle
		distance = 145  // DefaultCameraZoom
	)

	// Stand in the middle of the map, as a player in town would.
	targetX, targetZ := float32(mapW/2), float32(mapH/2)

	for step := 0; step < 8; step++ {
		yaw := float64(step) * gomath.Pi / 4

		offsetY := float32(distance * gomath.Sin(pitch))
		horiz := distance * gomath.Cos(pitch)
		pos := math.Vec3{
			X: targetX - float32(horiz*gomath.Sin(yaw)),
			Y: offsetY,
			Z: targetZ - float32(horiz*gomath.Cos(yaw)),
		}
		view := math.LookAt(pos,
			math.Vec3{X: targetX, Y: 30, Z: targetZ},
			math.Vec3{X: 0, Y: 1, Z: 0})
		frustum := FrustumFromViewProj(math.Perspective(fov, aspect, near, far).Mul(view))

		drawn := 0
		for _, model := range mr.models {
			if frustum.ContainsSphere(model.worldCenter, model.worldRadius) {
				drawn++
			}
		}

		total := len(mr.models)
		culled := float64(total-drawn) / float64(total)

		if drawn == 0 {
			t.Errorf("yaw %.2f: culled the entire map", yaw)
		}
		if culled < 0.85 {
			t.Errorf("yaw %.2f: only culled %.0f%% of %d models; the camera sees a "+
				"small part of a town, so most of it should be skipped", yaw, culled*100, total)
		}
		t.Logf("yaw %.2f: drew %d of %d (%.1f%% culled)", yaw, drawn, total, culled*100)
	}
}

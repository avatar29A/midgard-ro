package scene

import "testing"

// modelsAt builds instances at the given XZ positions.
func modelsAt(positions ...[2]float32) *ModelRenderer {
	mr := &ModelRenderer{}
	for _, p := range positions {
		mr.models = append(mr.models, &MapModel{position: [3]float32{p[0], 0, p[1]}})
	}
	return mr
}

// overlapping reports pairs close enough to share space, using the same rule
// assignDepthBiases does.
func overlapping(mr *ModelRenderer, a, b int) bool {
	dx := mr.models[a].position[0] - mr.models[b].position[0]
	dz := mr.models[a].position[2] - mr.models[b].position[2]
	return dx*dx+dz*dz < 30*30
}

// TestDepthBiasSeparatesOverlappingInstances is the property that matters:
// any two instances that can overlap must end up with different biases, or
// their coplanar faces have nothing to break the tie and flicker.
func TestDepthBiasSeparatesOverlappingInstances(t *testing.T) {
	tests := []struct {
		name      string
		positions [][2]float32
	}{
		{
			// A straight run of wall, as along Prontera's north face: a
			// 30-unit segment repeated every 28.3 units.
			name: "run of overlapping segments",
			positions: [][2]float32{
				{0, 0}, {20, 20}, {40, 40}, {60, 60}, {80, 80},
			},
		},
		{
			// A corner, where two runs meet. These come from different parts
			// of the world file, so their indices can be any distance apart —
			// which is what defeated deriving the bias from the index.
			name: "corner joining two runs",
			positions: [][2]float32{
				{0, 0}, {20, 20}, {40, 40}, // first run
				{200, 200}, {220, 220}, // unrelated, pushing indices apart
				{45, 35}, {50, 30}, // second run meeting the first
			},
		},
		{
			// Two instances at exactly the same coordinates. Prontera has
			// these, and no position-derived scheme can tell them apart.
			name: "duplicate instances in place",
			positions: [][2]float32{
				{100, 100}, {100, 100}, {100, 100},
			},
		},
		{
			// A dense cluster, the worst case for how many biases are needed.
			name: "dense cluster",
			positions: [][2]float32{
				{0, 0}, {5, 0}, {10, 0}, {0, 5}, {5, 5},
				{10, 5}, {0, 10}, {5, 10}, {10, 10}, {7, 7},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := modelsAt(tt.positions...)
			mr.assignDepthBiases()

			for a := range mr.models {
				if mr.models[a].depthBias < 0 {
					t.Fatalf("instance %d was left without a bias", a)
				}
				for b := a + 1; b < len(mr.models); b++ {
					if !overlapping(mr, a, b) {
						continue
					}
					if mr.models[a].depthBias == mr.models[b].depthBias {
						t.Errorf("instances %d and %d overlap but share bias %d — "+
							"nothing breaks the tie between their coplanar faces",
							a, b, mr.models[a].depthBias)
					}
				}
			}
		})
	}
}

// TestDepthBiasStaysSmall keeps the offsets in a range that cannot show. The
// bias multiplies the polygon's depth slope, so a large one can push a model
// behind geometry it should sit in front of.
func TestDepthBiasStaysSmall(t *testing.T) {
	// A cluster far denser than any real map corner.
	var positions [][2]float32
	for i := 0; i < 40; i++ {
		positions = append(positions, [2]float32{float32(i % 4), float32(i / 4)})
	}

	mr := modelsAt(positions...)
	mr.assignDepthBiases()

	maxBias := 0
	for _, m := range mr.models {
		if m.depthBias > maxBias {
			maxBias = m.depthBias
		}
	}

	// Greedy colouring never needs more than the worst overlap count plus one.
	if maxBias >= len(positions) {
		t.Errorf("largest bias %d, which is no better than giving every instance its own",
			maxBias)
	}
	if slope := float32(maxBias) * coplanarSlopeStep; slope > 16 {
		t.Errorf("largest slope offset %.1f is big enough to show", slope)
	}
}

// TestDepthBiasLeavesDistantInstancesAlone: instances that cannot overlap need
// no separation, and should mostly share the cheapest bias.
func TestDepthBiasLeavesDistantInstancesAlone(t *testing.T) {
	mr := modelsAt([2]float32{0, 0}, [2]float32{500, 500}, [2]float32{1000, 1000})
	mr.assignDepthBiases()

	for i, m := range mr.models {
		if m.depthBias != 0 {
			t.Errorf("isolated instance %d got bias %d, want 0", i, m.depthBias)
		}
	}
}

func TestDepthBiasHandlesNoModels(t *testing.T) {
	mr := &ModelRenderer{}
	mr.assignDepthBiases() // must not panic
}

package model

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// BuildMesh creates a model mesh from RSM data at the given animation time.
// Returns the mesh with vertices, indices, texture groups, and bounding box.
func BuildMesh(rsm *formats.RSM, opts BuildOptions) *Mesh {
	if len(rsm.Nodes) == 0 {
		return nil
	}

	var vertices []Vertex
	var indices []uint32
	texGroups := make(map[int][]uint32)

	// Track bounding box
	bounds := Bounds{
		Min: [3]float32{1e10, 1e10, 1e10},
		Max: [3]float32{-1e10, -1e10, -1e10},
	}

	// Process each node
	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]

		// Build node transform matrix with animation time
		nodeMatrix := BuildNodeMatrix(node, rsm, opts.AnimTimeMs)

		// Process faces
		for _, face := range node.Faces {
			if len(face.VertexIDs) < 3 {
				continue
			}

			// Bounds check vertex IDs
			validFace := true
			for _, vid := range face.VertexIDs {
				if int(vid) >= len(node.Vertices) {
					validFace = false
					break
				}
			}
			if !validFace {
				continue
			}

			// Calculate face normal from first 3 vertices
			v0 := node.Vertices[face.VertexIDs[0]]
			v1 := node.Vertices[face.VertexIDs[1]]
			v2 := node.Vertices[face.VertexIDs[2]]
			e1 := [3]float32{v1[0] - v0[0], v1[1] - v0[1], v1[2] - v0[2]}
			e2 := [3]float32{v2[0] - v0[0], v2[1] - v0[1], v2[2] - v0[2]}
			normalVec := Cross(e1, e2)

			// Degenerate triangle detection
			normalMag := float32(gomath.Sqrt(float64(normalVec[0]*normalVec[0] + normalVec[1]*normalVec[1] + normalVec[2]*normalVec[2])))
			if normalMag < 1e-5 {
				continue
			}
			normal := [3]float32{normalVec[0] / normalMag, normalVec[1] / normalMag, normalVec[2] / normalMag}

			isTwoSided := face.TwoSide != 0

			// Helper to add face vertices
			addFaceVertices := func(reverseOrder bool, flipNormal bool) uint32 {
				faceBaseIdx := uint32(len(vertices))
				faceNormal := normal
				if flipNormal {
					faceNormal = [3]float32{-normal[0], -normal[1], -normal[2]}
				}

				var vertIDs [3]uint16
				var texIDs [3]uint16
				if reverseOrder {
					vertIDs = [3]uint16{face.VertexIDs[2], face.VertexIDs[1], face.VertexIDs[0]}
					texIDs = [3]uint16{face.TexCoordIDs[2], face.TexCoordIDs[1], face.TexCoordIDs[0]}
				} else {
					vertIDs = face.VertexIDs
					texIDs = face.TexCoordIDs
				}

				for j := 0; j < 3; j++ {
					vid := vertIDs[j]
					v := node.Vertices[vid]

					// Transform vertex position by node matrix
					pos := TransformPoint(nodeMatrix, v)

					// Flip Y for RO coordinate system
					pos[1] = -pos[1]

					// Update bounding box
					updateBounds(&bounds, pos)

					// Get texture coordinates
					var uv [2]float32
					if int(texIDs[j]) < len(node.TexCoords) {
						tc := node.TexCoords[texIDs[j]]
						uv = [2]float32{tc.U, tc.V}
					}

					vertices = append(vertices, Vertex{
						Position: pos,
						Normal:   faceNormal,
						TexCoord: uv,
					})
				}
				return faceBaseIdx
			}

			// Add front face (with winding reversal if negative scale)
			faceBaseIdx := addFaceVertices(opts.ReverseWinding, false)

			// Get global texture index
			globalTexIdx := 0
			if int(face.TextureID) < len(node.TextureIDs) {
				globalTexIdx = int(node.TextureIDs[face.TextureID])
			}
			texGroups[globalTexIdx] = append(texGroups[globalTexIdx],
				faceBaseIdx, faceBaseIdx+1, faceBaseIdx+2)

			// If TwoSide or ForceAllTwoSided, add back face
			if isTwoSided || opts.ForceAllTwoSided {
				backFaceBaseIdx := addFaceVertices(!opts.ReverseWinding, true)
				texGroups[globalTexIdx] = append(texGroups[globalTexIdx],
					backFaceBaseIdx, backFaceBaseIdx+1, backFaceBaseIdx+2)
			}
		}
	}

	if len(vertices) == 0 {
		return nil
	}

	// Build texture groups and final index buffer
	var groups []TextureGroup
	for texIdx, idxs := range texGroups {
		if len(idxs) == 0 {
			continue
		}
		groups = append(groups, TextureGroup{
			TextureIdx: texIdx,
			StartIndex: int32(len(indices)),
			IndexCount: int32(len(idxs)),
		})
		indices = append(indices, idxs...)
	}

	// Smooth normals
	SmoothNormals(vertices)

	return &Mesh{
		Vertices: vertices,
		Indices:  indices,
		Groups:   groups,
		Bounds:   bounds,
	}
}

// CenterMeshXZ centers the mesh horizontally (X/Z) but preserves Y offset.
// Returns the centering offset applied.
func CenterMeshXZ(vertices []Vertex, bounds *Bounds) (centerX, centerZ float32) {
	centerX = (bounds.Min[0] + bounds.Max[0]) / 2
	centerZ = (bounds.Min[2] + bounds.Max[2]) / 2

	for i := range vertices {
		vertices[i].Position[0] -= centerX
		vertices[i].Position[2] -= centerZ
	}

	// Update bounds after centering
	bounds.Min[0] -= centerX
	bounds.Max[0] -= centerX
	bounds.Min[2] -= centerZ
	bounds.Max[2] -= centerZ

	return centerX, centerZ
}

// SmoothNormals averages normals at shared vertex positions.
// This reduces faceted appearance on models.
func SmoothNormals(vertices []Vertex) {
	const epsilon float32 = 0.001

	// Group vertices by quantized position for O(n) lookup
	posMap := make(map[[3]int32][]int)
	for i := range vertices {
		key := [3]int32{
			int32(vertices[i].Position[0] / epsilon),
			int32(vertices[i].Position[1] / epsilon),
			int32(vertices[i].Position[2] / epsilon),
		}
		posMap[key] = append(posMap[key], i)
	}

	// Average normals for vertices at same position
	for _, idxs := range posMap {
		if len(idxs) < 2 {
			continue
		}

		var sum [3]float32
		for _, idx := range idxs {
			sum[0] += vertices[idx].Normal[0]
			sum[1] += vertices[idx].Normal[1]
			sum[2] += vertices[idx].Normal[2]
		}

		avg := Normalize(sum)

		for _, idx := range idxs {
			vertices[idx].Normal = avg
		}
	}
}

// BuildNodeDebugInfo creates debug information for all nodes in an RSM.
func BuildNodeDebugInfo(rsm *formats.RSM) []NodeDebugInfo {
	info := make([]NodeDebugInfo, len(rsm.Nodes))
	for i, node := range rsm.Nodes {
		info[i] = NodeDebugInfo{
			Name:         node.Name,
			Parent:       node.Parent,
			Offset:       node.Offset,
			Position:     node.Position,
			Scale:        node.Scale,
			RotAngle:     node.RotAngle,
			RotAxis:      node.RotAxis,
			Matrix:       node.Matrix,
			HasRotKeys:   len(node.RotKeys) > 0,
			HasPosKeys:   len(node.PosKeys) > 0,
			HasScaleKeys: len(node.ScaleKeys) > 0,
			RotKeyCount:  len(node.RotKeys),
		}
		if len(node.RotKeys) > 0 {
			info[i].FirstRotQuat = node.RotKeys[0].Quaternion
		}
	}
	return info
}

// CountFaces returns total and two-sided face counts for an RSM.
func CountFaces(rsm *formats.RSM) (total, twoSided int) {
	for i := range rsm.Nodes {
		for _, face := range rsm.Nodes[i].Faces {
			total++
			if face.TwoSide != 0 {
				twoSided++
			}
		}
	}
	return total, twoSided
}

func updateBounds(b *Bounds, p [3]float32) {
	if p[0] < b.Min[0] {
		b.Min[0] = p[0]
	}
	if p[1] < b.Min[1] {
		b.Min[1] = p[1]
	}
	if p[2] < b.Min[2] {
		b.Min[2] = p[2]
	}
	if p[0] > b.Max[0] {
		b.Max[0] = p[0]
	}
	if p[1] > b.Max[1] {
		b.Max[1] = p[1]
	}
	if p[2] > b.Max[2] {
		b.Max[2] = p[2]
	}
}

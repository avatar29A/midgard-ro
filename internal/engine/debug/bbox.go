// Package debug provides debug visualization utilities.
package debug

// GenerateBBoxWireframeVertices creates line vertices for a wireframe bounding box.
// Returns 24 vertices (12 edges × 2 endpoints), format: [x, y, z] per vertex.
// minX, minY, minZ, maxX, maxY, maxZ define the box corners in world space.
func GenerateBBoxWireframeVertices(minX, minY, minZ, maxX, maxY, maxZ float32) []float32 {
	return []float32{
		// Bottom face (4 edges)
		minX, minY, minZ, maxX, minY, minZ,
		maxX, minY, minZ, maxX, minY, maxZ,
		maxX, minY, maxZ, minX, minY, maxZ,
		minX, minY, maxZ, minX, minY, minZ,
		// Top face (4 edges)
		minX, maxY, minZ, maxX, maxY, minZ,
		maxX, maxY, minZ, maxX, maxY, maxZ,
		maxX, maxY, maxZ, minX, maxY, maxZ,
		minX, maxY, maxZ, minX, maxY, minZ,
		// Vertical edges (4 edges)
		minX, minY, minZ, minX, maxY, minZ,
		maxX, minY, minZ, maxX, maxY, minZ,
		maxX, minY, maxZ, maxX, maxY, maxZ,
		minX, minY, maxZ, minX, maxY, maxZ,
	}
}

// GenerateBBoxWireframeFromAABB creates wireframe vertices from an AABB.
// bbox is [minX, minY, minZ, maxX, maxY, maxZ].
// position offsets the box in world space.
// scale scales each axis before translation.
// padding expands the box by the given amount on all sides.
func GenerateBBoxWireframeFromAABB(bbox [6]float32, position [3]float32, scale [3]float32, padding float32) []float32 {
	// Apply scale
	minX := bbox[0] * scale[0]
	minY := bbox[1] * scale[1]
	minZ := bbox[2] * scale[2]
	maxX := bbox[3] * scale[0]
	maxY := bbox[4] * scale[1]
	maxZ := bbox[5] * scale[2]

	// Handle negative scales
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	if minZ > maxZ {
		minZ, maxZ = maxZ, minZ
	}

	// Apply padding
	minX -= padding
	minY -= padding
	minZ -= padding
	maxX += padding
	maxY += padding
	maxZ += padding

	// Transform to world space
	minX += position[0]
	minY += position[1]
	minZ += position[2]
	maxX += position[0]
	maxY += position[1]
	maxZ += position[2]

	return GenerateBBoxWireframeVertices(minX, minY, minZ, maxX, maxY, maxZ)
}

// BBoxWireframeVertexCount is the number of vertices for a bbox wireframe (12 edges × 2).
const BBoxWireframeVertexCount = 24

// DefaultBBoxPadding is the default padding for selection boxes.
const DefaultBBoxPadding = 1.0

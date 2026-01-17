// Package model provides RSM model mesh building and animation utilities.
package model

// Vertex represents a model mesh vertex with position, normal, and texture coordinates.
type Vertex struct {
	Position [3]float32
	Normal   [3]float32
	TexCoord [2]float32
}

// TextureGroup groups triangles by texture index for batched rendering.
type TextureGroup struct {
	TextureIdx int
	StartIndex int32
	IndexCount int32
}

// Mesh holds the complete model mesh data ready for GPU upload.
type Mesh struct {
	Vertices []Vertex
	Indices  []uint32
	Groups   []TextureGroup
	Bounds   Bounds
}

// Bounds holds the axis-aligned bounding box of the model.
type Bounds struct {
	Min [3]float32
	Max [3]float32
}

// NodeDebugInfo stores debug information about an RSM node.
type NodeDebugInfo struct {
	Name         string
	Parent       string
	Offset       [3]float32
	Position     [3]float32
	Scale        [3]float32
	RotAngle     float32
	RotAxis      [3]float32
	Matrix       [9]float32
	HasRotKeys   bool
	HasPosKeys   bool
	HasScaleKeys bool
	FirstRotQuat [4]float32
	RotKeyCount  int
}

// BuildOptions contains options for mesh building.
type BuildOptions struct {
	// ReverseWinding reverses triangle winding order (for negative scale models).
	ReverseWinding bool
	// ForceAllTwoSided treats all faces as two-sided regardless of face flag.
	ForceAllTwoSided bool
	// AnimTimeMs is the animation time in milliseconds for animated models.
	AnimTimeMs float32
}

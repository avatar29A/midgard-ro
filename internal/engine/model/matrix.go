package model

import (
	gomath "math"

	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// BuildNodeMatrix builds the transformation matrix for an RSM node.
// Following roBrowser's approach: hierarchy matrix (inherited) + vertex transform (not inherited).
func BuildNodeMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32) math.Mat4 {
	// Get hierarchy matrix (parent * Position * Rotation * Scale)
	visited := make(map[string]bool)
	hierarchyMatrix := buildNodeHierarchyMatrix(node, rsm, animTimeMs, visited)

	// Add Offset and Mat3 for vertex transformation (NOT inherited by children)
	result := hierarchyMatrix
	result = result.Mul(math.Translate(node.Offset[0], node.Offset[1], node.Offset[2]))
	result = result.Mul(math.FromMat3x3(node.Matrix))

	return result
}

// buildNodeHierarchyMatrix returns the matrix that children inherit.
// This is: parent_hierarchy * Position * Rotation * Scale
// It does NOT include Offset or Mat3 (those are vertex-only transforms).
func buildNodeHierarchyMatrix(node *formats.RSMNode, rsm *formats.RSM, animTimeMs float32, visited map[string]bool) math.Mat4 {
	// Prevent infinite recursion
	if visited[node.Name] {
		return math.Identity()
	}
	visited[node.Name] = true

	// Check if node has rotation keyframes
	hasRotKeyframes := len(node.RotKeys) > 0

	// Build local hierarchy matrix: Position * Rotation * Scale
	localMatrix := math.Translate(node.Position[0], node.Position[1], node.Position[2])

	// Apply rotation (axis-angle OR keyframe, not both)
	if !hasRotKeyframes && node.RotAngle != 0 {
		axisLen := float32(gomath.Sqrt(float64(
			node.RotAxis[0]*node.RotAxis[0] +
				node.RotAxis[1]*node.RotAxis[1] +
				node.RotAxis[2]*node.RotAxis[2])))
		if axisLen > 1e-6 {
			normalizedAxis := [3]float32{
				node.RotAxis[0] / axisLen,
				node.RotAxis[1] / axisLen,
				node.RotAxis[2] / axisLen,
			}
			localMatrix = localMatrix.Mul(math.RotateAxis(normalizedAxis, node.RotAngle))
		}
	} else if hasRotKeyframes {
		rotQuat := InterpolateRotKeys(node.RotKeys, animTimeMs)
		localMatrix = localMatrix.Mul(rotQuat.ToMat4())
	}

	localMatrix = localMatrix.Mul(math.Scale(node.Scale[0], node.Scale[1], node.Scale[2]))

	// Apply animation scale if present
	if len(node.ScaleKeys) > 0 {
		scale := InterpolateScaleKeys(node.ScaleKeys, animTimeMs)
		localMatrix = localMatrix.Mul(math.Scale(scale[0], scale[1], scale[2]))
	}

	// If node has parent, get parent's hierarchy matrix first
	if node.Parent != "" && node.Parent != node.Name {
		parentNode := rsm.GetNodeByName(node.Parent)
		if parentNode != nil {
			parentHierarchy := buildNodeHierarchyMatrix(parentNode, rsm, animTimeMs, visited)
			return parentHierarchy.Mul(localMatrix)
		}
	}

	return localMatrix
}

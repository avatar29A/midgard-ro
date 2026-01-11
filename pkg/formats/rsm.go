// Package formats provides parsers for Ragnarok Online file formats.
// RSM (Resource Model) format parser for 3D models.
package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// RSM format errors.
var (
	ErrInvalidRSMMagic       = errors.New("invalid RSM magic: expected 'GRSM'")
	ErrUnsupportedRSMVersion = errors.New("unsupported RSM version")
	ErrTruncatedRSMData      = errors.New("truncated RSM data")
	ErrInvalidNodeCount      = errors.New("invalid RSM node count")
)

// RSMVersion represents the RSM file version.
type RSMVersion struct {
	Major uint8
	Minor uint8
}

// String returns the version as "Major.Minor".
func (v RSMVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// AtLeast returns true if version is >= major.minor.
func (v RSMVersion) AtLeast(major, minor uint8) bool {
	if v.Major > major {
		return true
	}
	if v.Major == major && v.Minor >= minor {
		return true
	}
	return false
}

// RSMShadingType represents the shading mode for rendering.
type RSMShadingType int32

const (
	RSMShadingNone   RSMShadingType = 0 // No shading
	RSMShadingFlat   RSMShadingType = 1 // Flat shading
	RSMShadingSmooth RSMShadingType = 2 // Smooth shading
)

// String returns a human-readable shading type name.
func (s RSMShadingType) String() string {
	switch s {
	case RSMShadingNone:
		return "None"
	case RSMShadingFlat:
		return "Flat"
	case RSMShadingSmooth:
		return "Smooth"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// RSMTexCoord represents a texture coordinate with optional vertex color.
type RSMTexCoord struct {
	Color [4]uint8  // RGBA vertex color (v1.2+)
	U, V  float32   // Texture coordinates
}

// RSMFace represents a triangle face in a mesh.
type RSMFace struct {
	VertexIDs   [3]uint16 // Indices into vertex array
	TexCoordIDs [3]uint16 // Indices into texcoord array
	TextureID   uint16    // Index into node's texture array
	Padding     uint16    // Padding (unused)
	TwoSide     int32     // Double-sided rendering flag
	SmoothGroup int32     // Smoothing group ID (v1.2+)
}

// RSMPosKeyframe represents a position animation keyframe.
type RSMPosKeyframe struct {
	Frame    int32      // Frame number
	Position [3]float32 // X, Y, Z position
}

// RSMRotKeyframe represents a rotation animation keyframe.
type RSMRotKeyframe struct {
	Frame      int32      // Frame number
	Quaternion [4]float32 // X, Y, Z, W quaternion
}

// RSMScaleKeyframe represents a scale animation keyframe.
type RSMScaleKeyframe struct {
	Frame int32      // Frame number
	Scale [3]float32 // X, Y, Z scale
}

// RSMNode represents a node in the model hierarchy.
type RSMNode struct {
	Name       string          // Node name
	Parent     string          // Parent node name (empty for root)
	TextureIDs []int32         // Indices into RSM.Textures array

	// Transform components
	Matrix   [9]float32  // 3x3 rotation matrix
	Offset   [3]float32  // Pivot point offset
	Position [3]float32  // Translation
	RotAngle float32     // Rotation angle (radians)
	RotAxis  [3]float32  // Rotation axis
	Scale    [3]float32  // Scale factors

	// Mesh data
	Vertices  [][3]float32 // Vertex positions
	TexCoords []RSMTexCoord // Texture coordinates
	Faces     []RSMFace     // Triangle faces

	// Animation keyframes
	PosKeys   []RSMPosKeyframe   // Position keyframes (v < 1.5)
	RotKeys   []RSMRotKeyframe   // Rotation keyframes
	ScaleKeys []RSMScaleKeyframe // Scale keyframes (v >= 1.5)
}

// RSMVolumeBox represents a bounding volume box.
type RSMVolumeBox struct {
	Size     [3]float32 // Box dimensions
	Position [3]float32 // Box center position
	Rotation [3]float32 // Box rotation (Euler angles)
	Flag     int32      // Box flag (v1.3+)
}

// RSM represents a parsed RSM (Resource Model) file.
type RSM struct {
	Version     RSMVersion     // File version
	AnimLength  int32          // Animation length in milliseconds
	Shading     RSMShadingType // Shading type
	Alpha       float32        // Global alpha (0-1)
	Textures    []string       // Texture file paths
	RootNode    string         // Root node name
	Nodes       []RSMNode      // Node hierarchy
	VolumeBoxes []RSMVolumeBox // Bounding volume boxes
}

// ParseRSM parses RSM data from a byte slice.
func ParseRSM(data []byte) (*RSM, error) {
	if len(data) < 14 {
		return nil, ErrTruncatedRSMData
	}

	r := bytes.NewReader(data)

	// Read magic
	magic := make([]byte, 4)
	if _, err := r.Read(magic); err != nil {
		return nil, ErrTruncatedRSMData
	}
	if string(magic) != "GRSM" {
		return nil, ErrInvalidRSMMagic
	}

	// Read version
	var verMajor, verMinor uint8
	binary.Read(r, binary.LittleEndian, &verMajor)
	binary.Read(r, binary.LittleEndian, &verMinor)

	rsm := &RSM{
		Version: RSMVersion{Major: verMajor, Minor: verMinor},
	}

	// Check supported versions (1.1 - 2.3)
	if rsm.Version.Major < 1 || rsm.Version.Major > 2 {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedRSMVersion, rsm.Version)
	}

	// Read animation length
	binary.Read(r, binary.LittleEndian, &rsm.AnimLength)

	// Read shading type
	binary.Read(r, binary.LittleEndian, &rsm.Shading)

	// Read alpha (v1.4+)
	if rsm.Version.AtLeast(1, 4) {
		var alpha uint8
		binary.Read(r, binary.LittleEndian, &alpha)
		rsm.Alpha = float32(alpha) / 255.0
	} else {
		rsm.Alpha = 1.0
	}

	// Skip reserved bytes (16 bytes for v < 2.2, variable for v >= 2.2)
	if rsm.Version.AtLeast(2, 2) {
		// v2.2+ has different header structure
		// Skip 16 bytes reserved
		r.Seek(16, 1)
	} else {
		// v1.x has 16 bytes reserved
		r.Seek(16, 1)
	}

	// Read texture count
	var textureCount int32
	binary.Read(r, binary.LittleEndian, &textureCount)

	// Read texture names
	rsm.Textures = make([]string, textureCount)
	for i := int32(0); i < textureCount; i++ {
		rsm.Textures[i] = readString(r, 40)
	}

	// Read root node name
	rsm.RootNode = readString(r, 40)

	// Read node count
	var nodeCount int32
	binary.Read(r, binary.LittleEndian, &nodeCount)

	if nodeCount < 0 || nodeCount > 10000 {
		return nil, ErrInvalidNodeCount
	}

	// Parse nodes
	rsm.Nodes = make([]RSMNode, nodeCount)
	for i := int32(0); i < nodeCount; i++ {
		node, err := parseRSMNode(r, rsm.Version)
		if err != nil {
			return nil, fmt.Errorf("parsing node %d: %w", i, err)
		}
		rsm.Nodes[i] = *node
	}

	// Parse volume boxes (if data remains)
	if r.Len() >= 4 {
		var boxCount int32
		binary.Read(r, binary.LittleEndian, &boxCount)

		if boxCount > 0 && boxCount < 1000 {
			rsm.VolumeBoxes = make([]RSMVolumeBox, boxCount)
			for i := int32(0); i < boxCount; i++ {
				box := &rsm.VolumeBoxes[i]
				binary.Read(r, binary.LittleEndian, &box.Size)
				binary.Read(r, binary.LittleEndian, &box.Position)
				binary.Read(r, binary.LittleEndian, &box.Rotation)

				// Flag (v1.3+)
				if rsm.Version.AtLeast(1, 3) {
					binary.Read(r, binary.LittleEndian, &box.Flag)
				}
			}
		}
	}

	return rsm, nil
}

// parseRSMNode parses a single node from the reader.
func parseRSMNode(r *bytes.Reader, version RSMVersion) (*RSMNode, error) {
	node := &RSMNode{}

	// Read node name and parent name
	node.Name = readString(r, 40)
	node.Parent = readString(r, 40)

	// Read texture indices
	var textureCount int32
	binary.Read(r, binary.LittleEndian, &textureCount)

	if textureCount > 0 && textureCount < 1000 {
		node.TextureIDs = make([]int32, textureCount)
		for i := int32(0); i < textureCount; i++ {
			binary.Read(r, binary.LittleEndian, &node.TextureIDs[i])
		}
	}

	// Read transform matrix (3x3, stored as 9 floats)
	for i := 0; i < 9; i++ {
		binary.Read(r, binary.LittleEndian, &node.Matrix[i])
	}

	// Read offset (pivot point)
	binary.Read(r, binary.LittleEndian, &node.Offset)

	// Read position
	binary.Read(r, binary.LittleEndian, &node.Position)

	// Read rotation (angle + axis)
	binary.Read(r, binary.LittleEndian, &node.RotAngle)
	binary.Read(r, binary.LittleEndian, &node.RotAxis)

	// Read scale
	binary.Read(r, binary.LittleEndian, &node.Scale)

	// Read vertices
	var vertexCount int32
	binary.Read(r, binary.LittleEndian, &vertexCount)

	if vertexCount > 0 && vertexCount < 100000 {
		node.Vertices = make([][3]float32, vertexCount)
		for i := int32(0); i < vertexCount; i++ {
			binary.Read(r, binary.LittleEndian, &node.Vertices[i])
		}
	}

	// Read texture coordinates
	var texCoordCount int32
	binary.Read(r, binary.LittleEndian, &texCoordCount)

	if texCoordCount > 0 && texCoordCount < 100000 {
		node.TexCoords = make([]RSMTexCoord, texCoordCount)
		for i := int32(0); i < texCoordCount; i++ {
			tc := &node.TexCoords[i]

			// Vertex color (v1.2+)
			if version.AtLeast(1, 2) {
				binary.Read(r, binary.LittleEndian, &tc.Color)
			} else {
				tc.Color = [4]uint8{255, 255, 255, 255}
			}

			binary.Read(r, binary.LittleEndian, &tc.U)
			binary.Read(r, binary.LittleEndian, &tc.V)
		}
	}

	// Read faces
	var faceCount int32
	binary.Read(r, binary.LittleEndian, &faceCount)

	if faceCount > 0 && faceCount < 100000 {
		node.Faces = make([]RSMFace, faceCount)
		for i := int32(0); i < faceCount; i++ {
			face := &node.Faces[i]
			binary.Read(r, binary.LittleEndian, &face.VertexIDs)
			binary.Read(r, binary.LittleEndian, &face.TexCoordIDs)
			binary.Read(r, binary.LittleEndian, &face.TextureID)
			binary.Read(r, binary.LittleEndian, &face.Padding)
			binary.Read(r, binary.LittleEndian, &face.TwoSide)

			// Smooth group (v1.2+)
			if version.AtLeast(1, 2) {
				binary.Read(r, binary.LittleEndian, &face.SmoothGroup)
			}
		}
	}

	// Read position keyframes (v < 1.5)
	if !version.AtLeast(1, 5) {
		var posKeyCount int32
		binary.Read(r, binary.LittleEndian, &posKeyCount)

		if posKeyCount > 0 && posKeyCount < 10000 {
			node.PosKeys = make([]RSMPosKeyframe, posKeyCount)
			for i := int32(0); i < posKeyCount; i++ {
				key := &node.PosKeys[i]
				binary.Read(r, binary.LittleEndian, &key.Frame)
				binary.Read(r, binary.LittleEndian, &key.Position)
			}
		}
	}

	// Read rotation keyframes
	var rotKeyCount int32
	binary.Read(r, binary.LittleEndian, &rotKeyCount)

	if rotKeyCount > 0 && rotKeyCount < 10000 {
		node.RotKeys = make([]RSMRotKeyframe, rotKeyCount)
		for i := int32(0); i < rotKeyCount; i++ {
			key := &node.RotKeys[i]
			binary.Read(r, binary.LittleEndian, &key.Frame)
			binary.Read(r, binary.LittleEndian, &key.Quaternion)
		}
	}

	// Read scale keyframes (v >= 1.5)
	if version.AtLeast(1, 5) {
		var scaleKeyCount int32
		binary.Read(r, binary.LittleEndian, &scaleKeyCount)

		if scaleKeyCount > 0 && scaleKeyCount < 10000 {
			node.ScaleKeys = make([]RSMScaleKeyframe, scaleKeyCount)
			for i := int32(0); i < scaleKeyCount; i++ {
				key := &node.ScaleKeys[i]
				binary.Read(r, binary.LittleEndian, &key.Frame)
				binary.Read(r, binary.LittleEndian, &key.Scale)
			}
		}
	}

	return node, nil
}

// ParseRSMFile parses an RSM file from disk.
func ParseRSMFile(path string) (*RSM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading RSM file: %w", err)
	}
	return ParseRSM(data)
}

// readString reads a fixed-length null-terminated string from a reader.
func readString(r *bytes.Reader, length int) string {
	buf := make([]byte, length)
	r.Read(buf)
	// Find null terminator
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// GetTotalVertexCount returns the total number of vertices across all nodes.
func (rsm *RSM) GetTotalVertexCount() int {
	total := 0
	for _, node := range rsm.Nodes {
		total += len(node.Vertices)
	}
	return total
}

// GetTotalFaceCount returns the total number of faces across all nodes.
func (rsm *RSM) GetTotalFaceCount() int {
	total := 0
	for _, node := range rsm.Nodes {
		total += len(node.Faces)
	}
	return total
}

// GetNodeByName returns a node by its name, or nil if not found.
func (rsm *RSM) GetNodeByName(name string) *RSMNode {
	for i := range rsm.Nodes {
		if rsm.Nodes[i].Name == name {
			return &rsm.Nodes[i]
		}
	}
	return nil
}

// GetRootNode returns the root node (first node matching RootNode name).
func (rsm *RSM) GetRootNode() *RSMNode {
	return rsm.GetNodeByName(rsm.RootNode)
}

// GetChildNodes returns all nodes that have the given parent name.
func (rsm *RSM) GetChildNodes(parentName string) []*RSMNode {
	var children []*RSMNode
	for i := range rsm.Nodes {
		if rsm.Nodes[i].Parent == parentName {
			children = append(children, &rsm.Nodes[i])
		}
	}
	return children
}

// HasAnimation returns true if the model has any animation keyframes.
func (rsm *RSM) HasAnimation() bool {
	for _, node := range rsm.Nodes {
		if len(node.PosKeys) > 0 || len(node.RotKeys) > 0 || len(node.ScaleKeys) > 0 {
			return true
		}
	}
	return false
}

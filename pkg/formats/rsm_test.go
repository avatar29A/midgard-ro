package formats

import (
	"encoding/binary"
	"testing"
)

func TestParseRSM_MagicValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name:    "valid magic",
			data:    makeRSMHeader("GRSM", 1, 5),
			wantErr: nil,
		},
		{
			name:    "invalid magic",
			data:    makeRSMHeader("XXXX", 1, 5),
			wantErr: ErrInvalidRSMMagic,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: ErrTruncatedRSMData,
		},
		{
			name:    "truncated data",
			data:    []byte{'G', 'R', 'S'},
			wantErr: ErrTruncatedRSMData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRSM(tt.data)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				}
			}
		})
	}
}

func TestParseRSM_VersionSupport(t *testing.T) {
	tests := []struct {
		name    string
		major   uint8
		minor   uint8
		wantErr bool
	}{
		{"v1.1", 1, 1, false},
		{"v1.2", 1, 2, false},
		{"v1.3", 1, 3, false},
		{"v1.4", 1, 4, false},
		{"v1.5", 1, 5, false},
		{"v2.1", 2, 1, false},
		{"v2.2", 2, 2, false},
		{"v2.3", 2, 3, false},
		{"v0.1 unsupported", 0, 1, true},
		{"v3.0 unsupported", 3, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := makeMinimalRSM(tt.major, tt.minor)
			_, err := ParseRSM(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("version %d.%d: got error=%v, wantErr=%v", tt.major, tt.minor, err, tt.wantErr)
			}
		})
	}
}

func TestRSMVersion_String(t *testing.T) {
	tests := []struct {
		version RSMVersion
		want    string
	}{
		{RSMVersion{1, 5}, "1.5"},
		{RSMVersion{2, 3}, "2.3"},
		{RSMVersion{1, 1}, "1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRSMVersion_AtLeast(t *testing.T) {
	tests := []struct {
		version RSMVersion
		major   uint8
		minor   uint8
		want    bool
	}{
		{RSMVersion{1, 5}, 1, 5, true},
		{RSMVersion{1, 5}, 1, 4, true},
		{RSMVersion{1, 5}, 1, 2, true},
		{RSMVersion{1, 5}, 1, 6, false},
		{RSMVersion{1, 5}, 2, 0, false},
		{RSMVersion{2, 3}, 1, 9, true},
		{RSMVersion{2, 3}, 2, 2, true},
		{RSMVersion{2, 3}, 2, 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.version.String(), func(t *testing.T) {
			if got := tt.version.AtLeast(tt.major, tt.minor); got != tt.want {
				t.Errorf("AtLeast(%d, %d) = %v, want %v", tt.major, tt.minor, got, tt.want)
			}
		})
	}
}

func TestRSMShadingType_String(t *testing.T) {
	tests := []struct {
		shading RSMShadingType
		want    string
	}{
		{RSMShadingNone, "None"},
		{RSMShadingFlat, "Flat"},
		{RSMShadingSmooth, "Smooth"},
		{RSMShadingType(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.shading.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRSM_V15_Structure(t *testing.T) {
	// Create a minimal v1.5 RSM file
	data := makeMinimalRSMWithNode(1, 5)

	rsm, err := ParseRSM(data)
	if err != nil {
		t.Fatalf("ParseRSM failed: %v", err)
	}

	if rsm.Version.Major != 1 || rsm.Version.Minor != 5 {
		t.Errorf("version mismatch: got %s", rsm.Version)
	}

	if len(rsm.Textures) != 1 {
		t.Errorf("texture count = %d, want 1", len(rsm.Textures))
	}

	if rsm.Textures[0] != "test.bmp" {
		t.Errorf("Textures[0] = %q, want %q", rsm.Textures[0], "test.bmp")
	}

	if rsm.RootNode != "root" {
		t.Errorf("RootNode = %q, want %q", rsm.RootNode, "root")
	}

	if len(rsm.Nodes) != 1 {
		t.Errorf("node count = %d, want 1", len(rsm.Nodes))
	}
}

func TestParseRSM_V14_Alpha(t *testing.T) {
	// v1.4+ has alpha byte
	data := makeMinimalRSMWithAlpha(1, 4, 128) // 50% alpha

	rsm, err := ParseRSM(data)
	if err != nil {
		t.Fatalf("ParseRSM failed: %v", err)
	}

	expectedAlpha := float32(128) / 255.0
	if rsm.Alpha < expectedAlpha-0.01 || rsm.Alpha > expectedAlpha+0.01 {
		t.Errorf("Alpha = %f, want ~%f", rsm.Alpha, expectedAlpha)
	}
}

func TestParseRSM_V13_NoAlpha(t *testing.T) {
	// v1.3 has no alpha byte, should default to 1.0
	data := makeMinimalRSM(1, 3)

	rsm, err := ParseRSM(data)
	if err != nil {
		t.Fatalf("ParseRSM failed: %v", err)
	}

	if rsm.Alpha != 1.0 {
		t.Errorf("Alpha = %f, want 1.0 (default for v1.3)", rsm.Alpha)
	}
}

func TestRSM_GetTotalVertexCount(t *testing.T) {
	rsm := &RSM{
		Nodes: []RSMNode{
			{Vertices: make([][3]float32, 10)},
			{Vertices: make([][3]float32, 20)},
			{Vertices: make([][3]float32, 5)},
		},
	}

	if got := rsm.GetTotalVertexCount(); got != 35 {
		t.Errorf("GetTotalVertexCount() = %d, want 35", got)
	}
}

func TestRSM_GetTotalFaceCount(t *testing.T) {
	rsm := &RSM{
		Nodes: []RSMNode{
			{Faces: make([]RSMFace, 10)},
			{Faces: make([]RSMFace, 20)},
		},
	}

	if got := rsm.GetTotalFaceCount(); got != 30 {
		t.Errorf("GetTotalFaceCount() = %d, want 30", got)
	}
}

func TestRSM_GetNodeByName(t *testing.T) {
	rsm := &RSM{
		Nodes: []RSMNode{
			{Name: "root"},
			{Name: "child1"},
			{Name: "child2"},
		},
	}

	node := rsm.GetNodeByName("child1")
	if node == nil {
		t.Fatal("GetNodeByName returned nil for existing node")
	}
	if node.Name != "child1" {
		t.Errorf("node.Name = %q, want %q", node.Name, "child1")
	}

	if rsm.GetNodeByName("nonexistent") != nil {
		t.Error("GetNodeByName returned non-nil for nonexistent node")
	}
}

func TestRSM_GetRootNode(t *testing.T) {
	rsm := &RSM{
		RootNode: "main",
		Nodes: []RSMNode{
			{Name: "other"},
			{Name: "main"},
		},
	}

	root := rsm.GetRootNode()
	if root == nil {
		t.Fatal("GetRootNode returned nil")
	}
	if root.Name != "main" {
		t.Errorf("root.Name = %q, want %q", root.Name, "main")
	}
}

func TestRSM_GetChildNodes(t *testing.T) {
	rsm := &RSM{
		Nodes: []RSMNode{
			{Name: "root", Parent: ""},
			{Name: "child1", Parent: "root"},
			{Name: "child2", Parent: "root"},
			{Name: "grandchild", Parent: "child1"},
		},
	}

	children := rsm.GetChildNodes("root")
	if len(children) != 2 {
		t.Errorf("got %d children, want 2", len(children))
	}

	children = rsm.GetChildNodes("child1")
	if len(children) != 1 {
		t.Errorf("got %d children of child1, want 1", len(children))
	}

	children = rsm.GetChildNodes("nonexistent")
	if len(children) != 0 {
		t.Errorf("got %d children of nonexistent, want 0", len(children))
	}
}

func TestRSM_HasAnimation(t *testing.T) {
	tests := []struct {
		name  string
		nodes []RSMNode
		want  bool
	}{
		{
			name:  "no animation",
			nodes: []RSMNode{{Name: "node"}},
			want:  false,
		},
		{
			name: "has rotation keys",
			nodes: []RSMNode{{
				Name:    "node",
				RotKeys: []RSMRotKeyframe{{Frame: 0}},
			}},
			want: true,
		},
		{
			name: "has position keys",
			nodes: []RSMNode{{
				Name:    "node",
				PosKeys: []RSMPosKeyframe{{Frame: 0}},
			}},
			want: true,
		},
		{
			name: "has scale keys",
			nodes: []RSMNode{{
				Name:      "node",
				ScaleKeys: []RSMScaleKeyframe{{Frame: 0}},
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsm := &RSM{Nodes: tt.nodes}
			if got := rsm.HasAnimation(); got != tt.want {
				t.Errorf("HasAnimation() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions for creating test data

func makeRSMHeader(magic string, major, minor uint8) []byte {
	// Create minimal header that passes magic check
	data := make([]byte, 200)
	copy(data[0:4], magic)
	data[4] = major
	data[5] = minor
	return data
}

func makeMinimalRSM(major, minor uint8) []byte {
	// Create a minimal valid RSM file
	data := make([]byte, 200)
	offset := 0

	// Magic
	copy(data[offset:], "GRSM")
	offset += 4

	// Version
	data[offset] = major
	offset++
	data[offset] = minor
	offset++

	// Animation length: 0
	offset += 4

	// Shading type: 0
	offset += 4

	// Alpha (v1.4+)
	if major > 1 || (major == 1 && minor >= 4) {
		data[offset] = 255 // fully opaque
		offset++
	}

	// Reserved: 16 bytes
	offset += 16

	// Texture count: 0
	binary.LittleEndian.PutUint32(data[offset:], 0)
	offset += 4

	// Root node name (40 bytes, empty)
	offset += 40

	// Node count: 0
	binary.LittleEndian.PutUint32(data[offset:], 0)
	offset += 4

	// Volume box count: 0
	binary.LittleEndian.PutUint32(data[offset:], 0)

	return data
}

func makeMinimalRSMWithNode(major, minor uint8) []byte {
	// Create RSM with one texture and one node
	data := make([]byte, 500)
	offset := 0

	// Magic
	copy(data[offset:], "GRSM")
	offset += 4

	// Version
	data[offset] = major
	offset++
	data[offset] = minor
	offset++

	// Animation length: 0
	offset += 4

	// Shading type: 2 (smooth)
	binary.LittleEndian.PutUint32(data[offset:], 2)
	offset += 4

	// Alpha (v1.4+)
	if major > 1 || (major == 1 && minor >= 4) {
		data[offset] = 255
		offset++
	}

	// Reserved: 16 bytes
	offset += 16

	// Texture count: 1
	binary.LittleEndian.PutUint32(data[offset:], 1)
	offset += 4

	// Texture name (40 bytes)
	copy(data[offset:], "test.bmp")
	offset += 40

	// Root node name (40 bytes)
	copy(data[offset:], "root")
	offset += 40

	// Node count: 1
	binary.LittleEndian.PutUint32(data[offset:], 1)
	offset += 4

	// Node data
	// Name (40 bytes)
	copy(data[offset:], "root")
	offset += 40

	// Parent name (40 bytes, empty for root)
	offset += 40

	// Texture count for node: 1
	binary.LittleEndian.PutUint32(data[offset:], 1)
	offset += 4

	// Texture ID: 0
	binary.LittleEndian.PutUint32(data[offset:], 0)
	offset += 4

	// Transform matrix (9 floats - identity matrix)
	// [1,0,0, 0,1,0, 0,0,1]
	binary.LittleEndian.PutUint32(data[offset:], 0x3f800000)   // 1.0
	binary.LittleEndian.PutUint32(data[offset+12:], 0x3f800000) // 1.0
	binary.LittleEndian.PutUint32(data[offset+24:], 0x3f800000) // 1.0
	offset += 36

	// Offset (3 floats)
	offset += 12

	// Position (3 floats)
	offset += 12

	// Rotation angle
	offset += 4

	// Rotation axis (3 floats)
	offset += 12

	// Scale (3 floats, all 1.0)
	binary.LittleEndian.PutUint32(data[offset:], 0x3f800000)   // 1.0
	binary.LittleEndian.PutUint32(data[offset+4:], 0x3f800000) // 1.0
	binary.LittleEndian.PutUint32(data[offset+8:], 0x3f800000) // 1.0
	offset += 12

	// Vertex count: 0
	offset += 4

	// Texture coord count: 0
	offset += 4

	// Face count: 0
	offset += 4

	// Rotation keyframe count: 0
	offset += 4

	// Scale keyframe count (v1.5+): 0
	if major > 1 || (major == 1 && minor >= 5) {
		offset += 4
	}

	// Volume box count: 0
	binary.LittleEndian.PutUint32(data[offset:], 0)

	return data
}

func makeMinimalRSMWithAlpha(major, minor, alpha uint8) []byte {
	data := makeMinimalRSM(major, minor)
	// Alpha is at offset 14 (after magic[4] + version[2] + animlen[4] + shading[4])
	if major > 1 || (major == 1 && minor >= 4) {
		data[14] = alpha
	}
	return data
}

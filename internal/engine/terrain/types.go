// Package terrain provides terrain mesh building and heightmap utilities for RO maps.
package terrain

// Vertex represents a terrain mesh vertex with all attributes.
type Vertex struct {
	Position   [3]float32
	Normal     [3]float32
	TexCoord   [2]float32
	LightmapUV [2]float32
	Color      [4]float32
}

// TextureGroup groups triangles by texture for batched rendering.
type TextureGroup struct {
	TextureID  int
	StartIndex int32
	IndexCount int32
}

// Mesh holds the complete terrain mesh data ready for GPU upload.
type Mesh struct {
	Vertices []Vertex
	Indices  []uint32
	Groups   []TextureGroup
	Bounds   Bounds
}

// Bounds holds the axis-aligned bounding box of the terrain.
type Bounds struct {
	Min [3]float32
	Max [3]float32
}

// LightmapAtlas holds lightmap atlas data and metadata.
type LightmapAtlas struct {
	Data        []byte // RGBA pixel data
	Size        int32  // Atlas size in pixels (square)
	TilesPerRow int32  // Number of lightmap tiles per row
	TileWidth   int    // Width of each lightmap tile
	TileHeight  int    // Height of each lightmap tile
}

// Heightmap provides terrain height lookup for a map.
type Heightmap struct {
	Altitudes [][]float32 // 2D array [x][z] of heights
	TilesX    int         // Number of tiles in X direction
	TilesZ    int         // Number of tiles in Z direction
	TileZoom  float32     // Size of each tile in world units
}

// TileGridVertex represents a vertex for tile grid debug visualization.
// Uses position and color (no texture coordinates).
type TileGridVertex struct {
	Position [3]float32
	Color    [4]float32
}

// TileGrid holds tile grid mesh data for debug visualization.
type TileGrid struct {
	Vertices []TileGridVertex
	Indices  []uint32
}

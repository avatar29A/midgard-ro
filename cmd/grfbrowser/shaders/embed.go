// Package shaders provides embedded GLSL shader sources.
package shaders

import _ "embed"

// TerrainVertexShader is the vertex shader for terrain rendering.
//
//go:embed terrain.vert
var TerrainVertexShader string

// TerrainFragmentShader is the fragment shader for terrain rendering.
//
//go:embed terrain.frag
var TerrainFragmentShader string

// ModelVertexShader is the vertex shader for model rendering.
//
//go:embed model.vert
var ModelVertexShader string

// ModelFragmentShader is the fragment shader for model rendering.
//
//go:embed model.frag
var ModelFragmentShader string

// WaterVertexShader is the vertex shader for water rendering.
//
//go:embed water.vert
var WaterVertexShader string

// WaterFragmentShader is the fragment shader for water rendering.
//
//go:embed water.frag
var WaterFragmentShader string

// BboxVertexShader is the vertex shader for bounding box rendering.
//
//go:embed bbox.vert
var BboxVertexShader string

// BboxFragmentShader is the fragment shader for bounding box rendering.
//
//go:embed bbox.frag
var BboxFragmentShader string

// SpriteVertexShader is the vertex shader for sprite rendering.
//
//go:embed sprite.vert
var SpriteVertexShader string

// SpriteFragmentShader is the fragment shader for sprite rendering.
//
//go:embed sprite.frag
var SpriteFragmentShader string

// ShadowVertexShader is the vertex shader for shadow map rendering.
//
//go:embed shadow.vert
var ShadowVertexShader string

// ShadowFragmentShader is the fragment shader for shadow map rendering.
//
//go:embed shadow.frag
var ShadowFragmentShader string

// TileGridVertexShader is the vertex shader for tile grid debug visualization.
//
//go:embed tilegrid.vert
var TileGridVertexShader string

// TileGridFragmentShader is the fragment shader for tile grid debug visualization.
//
//go:embed tilegrid.frag
var TileGridFragmentShader string

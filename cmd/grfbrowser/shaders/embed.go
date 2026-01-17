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

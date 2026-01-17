// Package shaders provides embedded GLSL shader sources.
package shaders

import _ "embed"

// Terrain shaders
//
//go:embed terrain.vert
var TerrainVertexShader string

//go:embed terrain.frag
var TerrainFragmentShader string

// Model shaders
//
//go:embed model.vert
var ModelVertexShader string

//go:embed model.frag
var ModelFragmentShader string

// Water shaders
//
//go:embed water.vert
var WaterVertexShader string

//go:embed water.frag
var WaterFragmentShader string

// Bounding box shaders
//
//go:embed bbox.vert
var BboxVertexShader string

//go:embed bbox.frag
var BboxFragmentShader string

// Sprite shaders
//
//go:embed sprite.vert
var SpriteVertexShader string

//go:embed sprite.frag
var SpriteFragmentShader string

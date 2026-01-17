// Constants for GRF Browser application.
package main

import "math"

// NumDirections is the number of directions for character sprites.
const NumDirections = 8

// Extended action types (entity package has ActionIdle and ActionWalk)
const (
	ActionSit    = 2
	ActionPickUp = 3
)

// Zoom and scale defaults
const (
	DefaultSpriteZoom   = 1.0
	DefaultMapZoom      = 1.0
	SpriteZoomIncrement = 0.5
	TileZoomIncrement   = 0.25
	MinZoom             = 0.5
	MaxZoom             = 8.0
)

// Rendering defaults
const (
	DefaultMaxModels         = 1500
	DefaultTerrainBrightness = 1.0
	DefaultFogNear           = 150.0
	DefaultFogFar            = 1400.0
	DefaultMoveSpeed         = 120.0 // Faster movement for responsive feel
	DefaultSpriteScale       = 0.45  // Larger sprite for better visibility
	DefaultShadowScale       = 0.5   // Larger shadow to match sprite
)

// Direction calculation constants
const (
	DirectionHysteresis = math.Pi / 16 // ~11 degrees dead zone
	SectorSize          = math.Pi / 4  // 45 degrees per sector
	SectorOffset        = math.Pi / 8  // 22.5 degrees offset for centering
)

// Sprite bounds calculation
const (
	BoundsMax = 100000
	BoundsMin = -100000
)

// Animation timing
const (
	DefaultAnimInterval = 150.0 // milliseconds per frame
	MinAnimInterval     = 50.0  // minimum interval to prevent too fast animation
)

// Billboard vertex offsets (normalized quad)
const (
	BillboardLeft   = -0.5
	BillboardRight  = 0.5
	BillboardBottom = 0.0
	BillboardTop    = 1.0
)

// Preview list limits
const (
	MaxPreviewListItems = 100
	MaxEffectListItems  = 50
	MaxSoundListItems   = 50
	MaxLightListItems   = 50
	MaxModelListItems   = 100
)

// Water rendering
const (
	WaterTextureFrames = 32
	WaterAnimSpeed     = 60.0 // milliseconds per frame
)

// DirectionMap converts geometric sectors (from atan2) to RO direction indices.
// Sectors go counter-clockwise from +Z, RO directions are ordered differently.
var DirectionMap = [NumDirections]int{0, 7, 6, 5, 4, 3, 2, 1}

// DirectionNames provides human-readable names for directions.
var DirectionNames = [NumDirections]string{"S", "SW", "W", "NW", "N", "NE", "E", "SE"}

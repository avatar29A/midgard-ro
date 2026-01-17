// Package character provides character animation and movement utilities.
package character

import (
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// CompositeFrame holds a pre-composited sprite frame (head + body merged).
type CompositeFrame struct {
	Texture uint32 // OpenGL texture ID
	Width   int    // Texture width in pixels
	Height  int    // Texture height in pixels
	OriginX int    // X offset from sprite origin to texture center
	OriginY int    // Y offset from sprite origin to texture center
}

// Player represents a player character with sprite data and rendering state.
// It embeds entity.Character for core game logic.
type Player struct {
	*entity.Character // Embedded character state (position, movement, animation)

	// Sprite data (body)
	SPR      *formats.SPR
	ACT      *formats.ACT
	Textures []uint32 // GPU textures for each SPR image

	// Head sprite data
	HeadSPR      *formats.SPR
	HeadACT      *formats.ACT
	HeadTextures []uint32 // GPU textures for head SPR images

	// Composite textures: [action*8+direction][frame] -> CompositeFrame
	// Pre-composited head+body for each animation frame
	CompositeFrames    map[int][]CompositeFrame
	UseComposite       bool // Whether to use composite rendering
	CompositeMaxWidth  int  // Max width across all composites (for consistent sizing)
	CompositeMaxHeight int  // Max height across all composites (for consistent sizing)

	// Billboard rendering
	VAO         uint32
	VBO         uint32
	SpriteScale float32 // Scale factor for sprite (default 1.0)

	// Shadow
	ShadowTex uint32 // Shadow texture (ellipse)
	ShadowVAO uint32
	ShadowVBO uint32
}

// TerrainQuery provides terrain information for character movement.
type TerrainQuery interface {
	// IsWalkable returns true if the given world position is walkable.
	IsWalkable(worldX, worldZ float32) bool
	// GetHeight returns the terrain height at the given world position.
	GetHeight(worldX, worldZ float32) float32
}

// Package states implements game state management.
package states

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/engine/playerrender"
	"github.com/Faultbox/midgard-ro/internal/engine/scene"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// InGameStateConfig contains configuration for the in-game state.
type InGameStateConfig struct {
	MapName   string
	SpawnX    int
	SpawnY    int
	SpawnDir  uint8
	CharID    uint32
	TexLoader func(string) ([]byte, error)
}

// InGameState handles the main gameplay state.
type InGameState struct {
	config  InGameStateConfig
	client  *network.Client
	manager *Manager

	// Rendering
	scene        *scene.Scene
	camera       *camera.ThirdPersonCamera
	gat          *formats.GAT // Walkability + minimap shape
	playerRender *playerrender.Renderer

	// Entities
	entityManager *entity.Manager
	player        *entity.Character

	// Map info
	MapName string
	TileX   int // Current tile X
	TileY   int // Current tile Y

	// Movement input
	moveInputX float32 // -1 to 1
	moveInputZ float32 // -1 to 1

	// Network timing
	lastMoveTick      uint32
	moveTickRate      time.Duration
	lastKeepAlive     time.Time
	keepAliveInterval time.Duration
	enterTime         time.Time // Used as the local epoch for ClientTick

	// State
	ErrorMsg   string
	StatusMsg  string
	MapLoaded  bool
	SceneReady bool
}

// NewInGameState creates a new in-game state.
func NewInGameState(cfg InGameStateConfig, client *network.Client, manager *Manager) *InGameState {
	return &InGameState{
		config:            cfg,
		client:            client,
		manager:           manager,
		entityManager:     entity.NewManager(),
		MapName:           cfg.MapName,
		TileX:             cfg.SpawnX,
		TileY:             cfg.SpawnY,
		moveTickRate:      100 * time.Millisecond, // Send move requests every 100ms max
		keepAliveInterval: 10 * time.Second,       // rAthena map server times out around 30s of silence
	}
}

// Enter is called when entering this state.
func (s *InGameState) Enter() error {
	logger.Info("entering InGameState",
		zap.String("map", s.MapName),
		zap.Int("spawnX", s.config.SpawnX),
		zap.Int("spawnY", s.config.SpawnY))

	s.ErrorMsg = ""
	s.StatusMsg = fmt.Sprintf("Loading %s...", s.MapName)

	// Create scene
	var err error
	s.scene, err = scene.New(scene.DefaultConfig())
	if err != nil {
		logger.Error("failed to create scene", zap.Error(err))
		s.ErrorMsg = fmt.Sprintf("Failed to create scene: %v", err)
		return err
	}

	// Load map data from GRF
	if err := s.loadMap(); err != nil {
		logger.Warn("failed to load map", zap.Error(err))
		// Continue without map - just show player position
		s.StatusMsg = fmt.Sprintf("Map not loaded: %v", err)
	} else {
		s.MapLoaded = true
		s.SceneReady = true
	}

	// Create player character at spawn position
	// Convert tile coords to world coords (RO uses 5 units per tile)
	tileSize := float32(5.0)
	worldX := float32(s.config.SpawnX) * tileSize
	worldZ := float32(s.config.SpawnY) * tileSize

	// Get terrain height at spawn position
	var worldY float32
	if s.scene != nil && s.MapLoaded {
		worldY = s.scene.GetTerrainHeight(worldX, worldZ)
	}

	s.player = entity.NewCharacter(worldX, worldY, worldZ)
	s.player.Direction = int(s.config.SpawnDir)

	logger.Debug("created player character",
		zap.Float32("worldX", worldX),
		zap.Float32("worldY", worldY),
		zap.Float32("worldZ", worldZ))

	// Create entity wrapper for the player
	playerEntity := entity.NewEntity(s.config.CharID, entity.TypePlayer)
	playerEntity.Position.X = worldX
	playerEntity.Position.Y = worldY
	playerEntity.Position.Z = worldZ
	s.entityManager.SetPlayer(playerEntity)

	// Create third-person camera following player (RO-style)
	s.camera = camera.NewThirdPersonCamera()
	s.camera.Distance = 145 // RO-style close distance (like grfbrowser PlayMode)
	s.camera.Yaw = 0

	// Build the player billboard renderer (procedural texture for now —
	// real Novice SPR/ACT composites land in a follow-up PR).
	if pr, prErr := playerrender.New(); prErr != nil {
		logger.Warn("failed to create player renderer", zap.Error(prErr))
	} else {
		s.playerRender = pr
	}

	s.StatusMsg = fmt.Sprintf("Entered %s", s.MapName)

	// Mark entry time — used as the local epoch for ClientTick and as the
	// gate for the keep-alive ticker (only run after we're actually in-game).
	s.enterTime = time.Now()
	s.lastKeepAlive = s.enterTime

	// Register packet handlers
	s.registerPacketHandlers()

	return nil
}

// loadMap loads the map data from GRF archives.
func (s *InGameState) loadMap() error {
	if s.manager.TexLoader == nil {
		return fmt.Errorf("no texture loader available")
	}

	// Get base map name (remove .gat extension)
	baseName := strings.TrimSuffix(s.MapName, ".gat")

	// Load GAT (walkability + minimap shape).  Non-fatal — log and continue.
	gatPath := "data\\" + baseName + ".gat"
	if gatData, gatErr := s.manager.TexLoader(gatPath); gatErr == nil {
		if gat, parseErr := formats.ParseGAT(gatData); parseErr == nil {
			s.gat = gat
		} else {
			logger.Warn("failed to parse GAT", zap.Error(parseErr))
		}
	} else {
		logger.Warn("failed to load GAT", zap.Error(gatErr))
	}

	// Load GND (terrain)
	gndPath := "data\\" + baseName + ".gnd"
	gndData, err := s.manager.TexLoader(gndPath)
	if err != nil {
		return fmt.Errorf("loading GND: %w", err)
	}
	gnd, err := formats.ParseGND(gndData)
	if err != nil {
		return fmt.Errorf("parsing GND: %w", err)
	}

	// Load RSW (map resources)
	rswPath := "data\\" + baseName + ".rsw"
	rswData, err := s.manager.TexLoader(rswPath)
	var rsw *formats.RSW
	if err == nil {
		rsw, err = formats.ParseRSW(rswData)
		if err != nil {
			logger.Warn("failed to parse RSW", zap.Error(err))
		}
	} else {
		logger.Warn("failed to load RSW", zap.Error(err))
	}

	// Load map into scene
	if err := s.scene.LoadMap(gnd, rsw, s.manager.TexLoader); err != nil {
		return fmt.Errorf("loading map into scene: %w", err)
	}

	logger.Info("map loaded successfully",
		zap.String("map", baseName),
		zap.Float32("width", s.scene.MapWidth),
		zap.Float32("height", s.scene.MapHeight))

	return nil
}

// Exit is called when leaving this state.
func (s *InGameState) Exit() error {
	if s.playerRender != nil {
		s.playerRender.Destroy()
		s.playerRender = nil
	}
	if s.scene != nil {
		s.scene.Destroy()
		s.scene = nil
	}
	return nil
}

// Update is called every frame.
func (s *InGameState) Update(dt float64) error {
	deltaMs := float32(dt * 1000)

	// Process network
	if err := s.client.Process(); err != nil {
		s.ErrorMsg = fmt.Sprintf("Network error: %v", err)
	}

	// Keep-alive: rAthena's map server drops the session after a few seconds
	// of silence. Send CZ_REQUEST_TIME at keepAliveInterval cadence.
	if !s.enterTime.IsZero() && time.Since(s.lastKeepAlive) >= s.keepAliveInterval {
		s.sendKeepAlive()
		s.lastKeepAlive = time.Now()
	}

	// Update player movement
	if s.player != nil {
		// Handle keyboard movement input
		if s.moveInputX != 0 || s.moveInputZ != 0 {
			s.player.UpdateWithVelocity(s.moveInputX, s.moveInputZ, deltaMs)
		} else {
			// Handle click-to-move
			s.player.Update(deltaMs)
		}

		// Update render interpolation
		s.player.UpdateRenderPosition(deltaMs)

		// Update tile position
		tileSize := float32(5.0)
		s.TileX = int(s.player.WorldX / tileSize)
		s.TileY = int(s.player.WorldZ / tileSize)
	}

	// Update all entities
	s.entityManager.Update(dt)

	return nil
}

// Render is called every frame to draw the state.
func (s *InGameState) Render() error {
	if s.scene == nil || s.camera == nil || !s.SceneReady || s.player == nil {
		return nil
	}

	// Player position for the camera to follow.
	x, y, z := s.player.RenderPosition()

	// Use the extras hook so the player billboard composites into the
	// scene framebuffer (after world rendering, before unbind).
	s.scene.RenderWithThirdPersonExtras(s.camera, x, y, z, func(viewProj math.Mat4) {
		if s.playerRender != nil {
			s.playerRender.Render(viewProj, s.player, s.camera.PosX, s.camera.PosZ)
		}
	})
	return nil
}

// GetSceneTexture returns the rendered scene texture ID for display.
func (s *InGameState) GetSceneTexture() uint32 {
	if s.scene != nil {
		return s.scene.ColorTexture()
	}
	return 0
}

// GetCamera returns the camera.
func (s *InGameState) GetCamera() *camera.ThirdPersonCamera {
	return s.camera
}

// GetScene returns the underlying scene (for diagnostics — exposes
// framebuffer dimensions, terrain Y query, etc).
func (s *InGameState) GetScene() *scene.Scene {
	return s.scene
}

// NetworkClient returns the underlying network client (for diagnostics).
func (s *InGameState) NetworkClient() *network.Client {
	return s.client
}

// ResizeScene resizes the scene framebuffer to match the window size.
func (s *InGameState) ResizeScene(width, height int32) {
	if s.scene != nil {
		logger.Debug("ResizeScene called", zap.Int32("width", width), zap.Int32("height", height))
		s.scene.Resize(width, height)
	}
}

// IsSceneReady returns whether the scene is ready for rendering.
func (s *InGameState) IsSceneReady() bool {
	return s.SceneReady
}

// HandleInput processes input events.
func (s *InGameState) HandleInput(event interface{}) error {
	// Input handling will be wired up by the game
	return nil
}

func (s *InGameState) registerPacketHandlers() {
	s.client.RegisterHandler(packets.ZC_NOTIFY_STANDENTRY, s.handleEntitySpawn)
	s.client.RegisterHandler(packets.ZC_NOTIFY_MOVEENTRY, s.handleEntityMove)
	s.client.RegisterHandler(packets.ZC_NPCACK_MAPMOVE, s.handleMapChange)
	s.client.RegisterHandler(packets.ZC_NOTIFY_PLAYERMOVE, s.handlePlayerMove)
}

// sendKeepAlive sends CZ_REQUEST_TIME so the map server doesn't time us out.
func (s *InGameState) sendKeepAlive() {
	pkt := &packets.TickSend{
		PacketID:   packets.CZ_REQUEST_TIME,
		ClientTick: uint32(time.Since(s.enterTime).Milliseconds()),
	}
	if err := s.client.Send(pkt.Encode()); err != nil {
		logger.Warn("keep-alive send failed", zap.Error(err))
	}
}

// handlePlayerMove processes ZC_NOTIFY_PLAYERMOVE — server confirms our
// own walk request. We trust the server-reported start/end tiles and
// re-target our local destination so the rendered position converges
// on the authoritative path.
func (s *InGameState) handlePlayerMove(data []byte) error {
	mv := packets.DecodePlayerMove(data)
	if mv == nil {
		return fmt.Errorf("invalid ZC_NOTIFY_PLAYERMOVE: %d bytes", len(data))
	}

	logger.Debug("player walk-OK",
		zap.Uint32("startTick", mv.StartTick),
		zap.Int("startX", mv.StartX),
		zap.Int("startY", mv.StartY),
		zap.Int("endX", mv.EndX),
		zap.Int("endY", mv.EndY))

	if s.player == nil {
		return nil
	}
	tileSize := float32(5.0)
	s.player.SetDestination(float32(mv.EndX)*tileSize, float32(mv.EndY)*tileSize)
	return nil
}

func (s *InGameState) handleEntitySpawn(data []byte) error {
	// Parse entity spawn packet (simplified)
	// Full implementation would extract entity ID, type, position, etc.
	return nil
}

func (s *InGameState) handleEntityMove(data []byte) error {
	// Parse entity movement packet
	return nil
}

func (s *InGameState) handleMapChange(data []byte) error {
	// Handle map change request from server
	// This would trigger a transition to loading state for the new map
	return nil
}

// SetMoveInput sets the movement input from keyboard.
func (s *InGameState) SetMoveInput(x, z float32) {
	s.moveInputX = x
	s.moveInputZ = z
}

// ScreenToTile maps a screen-space click (in viewport pixels) to a tile
// coordinate by ray-casting against the y=0 ground plane using the most
// recent view-projection matrix the scene rendered with.
//
// Returns ok=false if the scene hasn't rendered yet, or if the ray points
// away from the ground (e.g. clicking the sky).
func (s *InGameState) ScreenToTile(screenX, screenY, viewportW, viewportH float32) (tileX, tileY int, ok bool) {
	if s.scene == nil || viewportW <= 0 || viewportH <= 0 {
		return 0, 0, false
	}
	invViewProj := s.scene.LastViewProj().Inverse()
	ray := picking.ScreenToRay(screenX, screenY, viewportW, viewportH, invViewProj)
	worldX, worldZ, hit := ray.IntersectPlaneY(0)
	if !hit {
		return 0, 0, false
	}
	const tileSize = float32(5.0)
	return int(worldX / tileSize), int(worldZ / tileSize), true
}

// RequestMove sends a movement request to the server.
func (s *InGameState) RequestMove(tileX, tileY int) error {
	pkt := &packets.MoveRequest{
		PacketID: packets.CZ_REQUEST_MOVE,
	}
	pkt.SetDestination(tileX, tileY)

	if err := s.client.Send(pkt.Encode()); err != nil {
		return fmt.Errorf("send move request: %w", err)
	}

	// Also set local destination for immediate visual feedback
	if s.player != nil {
		tileSize := float32(5.0)
		s.player.SetDestination(float32(tileX)*tileSize, float32(tileY)*tileSize)
	}

	s.lastMoveTick = uint32(time.Now().UnixMilli() & 0xFFFFFFFF)
	return nil
}

// GetPlayer returns the player character.
func (s *InGameState) GetPlayer() *entity.Character {
	return s.player
}

// GetEntityManager returns the entity manager.
func (s *InGameState) GetEntityManager() *entity.Manager {
	return s.entityManager
}

// GetPlayerTilePosition returns the player's current tile position.
func (s *InGameState) GetPlayerTilePosition() (int, int) {
	return s.TileX, s.TileY
}

// GetPlayerWorldPosition returns the player's world position.
func (s *InGameState) GetPlayerWorldPosition() (float32, float32, float32) {
	if s.player != nil {
		return s.player.RenderPosition()
	}
	return 0, 0, 0
}

// GetStatusMessage returns the current status message.
func (s *InGameState) GetStatusMessage() string {
	return s.StatusMsg
}

// GetErrorMessage returns the current error message.
func (s *InGameState) GetErrorMessage() string {
	return s.ErrorMsg
}

// GetMapName returns the current map name.
func (s *InGameState) GetMapName() string {
	return s.MapName
}

// GetGAT returns the loaded GAT (walkability) data, or nil if unavailable.
func (s *InGameState) GetGAT() *formats.GAT {
	return s.gat
}

// GetPlayerEntity returns the player as an Entity (for UI).
func (s *InGameState) GetPlayerEntity() *entity.Entity {
	return s.entityManager.Player()
}

// CaptureScene captures the current rendered scene as RGBA pixel data.
// Returns pixels, width, height. Returns nil if no scene is available.
func (s *InGameState) CaptureScene() ([]byte, int32, int32) {
	if s.scene == nil {
		return nil, 0, 0
	}
	return s.scene.CaptureImage()
}

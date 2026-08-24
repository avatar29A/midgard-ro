// Package states implements game state management.
package states

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/picking"
	"github.com/Faultbox/midgard-ro/internal/engine/playerrender"
	"github.com/Faultbox/midgard-ro/internal/engine/scene"
	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/world"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
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

	// Walk pathing over the GAT, used to reproduce the route the server
	// walks us along (it only tells us the endpoints).
	pathFinder *world.PathFinder

	// Map info
	MapName string
	TileX   int // Current tile X
	TileY   int // Current tile Y

	// Network timing
	lastMoveTick      uint32
	lastMoveSent      time.Time
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

	// Pathfinder over the map's walkability grid. The server tells us only
	// where a walk starts and ends, so we re-derive the cells in between.
	s.pathFinder = world.NewPathFinder(s.gat)

	// Create player character at the spawn cell's center.
	worldX, worldZ := entity.CellToWorld(s.config.SpawnX, s.config.SpawnY)

	// Get terrain height at spawn position
	var worldY float32
	if s.scene != nil && s.MapLoaded {
		worldY = s.scene.GetTerrainHeight(worldX, worldZ)
	}

	s.player = entity.NewCharacter(worldX, worldY, worldZ)

	// The server numbers directions the opposite way round the compass from
	// the sprite sheets, so this needs converting rather than assigning.
	s.player.Direction = entity.DirectionFromServer(s.config.SpawnDir)

	// Walk timing comes from the character's `speed` stat (ms per cell).
	s.player.WalkSpeedMs = s.manager.Session.WalkSpeedMs()

	// Let the character follow the ground as it walks.
	if s.scene != nil && s.MapLoaded {
		scn := s.scene
		s.player.TerrainHeight = func(x, z float32) float32 {
			return scn.GetTerrainHeight(x, z)
		}
	}

	logger.Info("player walk speed",
		zap.Float32("msPerCell", s.player.WalkSpeedMs),
		zap.Bool("hasPathfinder", s.pathFinder != nil))

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

	// Build the player billboard renderer and load the character's sprites.
	// A sprite failure is not fatal: the renderer keeps drawing its
	// procedural marker so the player can still see where they are.
	if pr, prErr := playerrender.New(); prErr != nil {
		logger.Warn("failed to create player renderer", zap.Error(prErr))
	} else {
		s.playerRender = pr
		s.loadPlayerSprites()
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

// loadPlayerSprites loads the SPR/ACT art for the character we're playing and
// bakes it into the billboard renderer.
func (s *InGameState) loadPlayerSprites() {
	if s.manager.TexLoader == nil {
		logger.Warn("no asset loader; player will render as a placeholder")
		return
	}

	spec := s.manager.Session.SpriteSpec()
	if err := s.playerRender.LoadCharacter(charsprite.Loader(s.manager.TexLoader), spec); err != nil {
		logger.Warn("failed to load character sprites, using placeholder",
			zap.Int("job", spec.Job),
			zap.Bool("female", spec.Female),
			zap.Int("hairStyle", spec.HairStyle),
			zap.Error(err))
		return
	}

	logger.Info("character sprites loaded",
		zap.String("sprite", s.playerRender.SpritePath()),
		zap.Int("idleFrames", s.playerRender.FrameCount(entity.ActionIdle, entity.DirS)),
		zap.Int("walkFrames", s.playerRender.FrameCount(entity.ActionWalk, entity.DirS)))
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

	// Update player movement. Walking is server-authoritative: this advances
	// along the path the server acknowledged.
	if s.player != nil {
		s.player.Update(deltaMs)

		// Free movement needs a render catch-up pass; path walking already
		// interpolates on server timing and ignores this.
		s.player.UpdateRenderPosition(deltaMs)

		// Advance the sprite animation. Frame counts come from the loaded
		// sheet; with no sprites this parks on frame 0 harmlessly.
		frames := 0
		if s.playerRender != nil {
			action := entity.ActionIdle
			if s.player.IsMoving {
				action = entity.ActionWalk
			}
			frames = s.playerRender.FrameCount(action, s.player.Direction)
		}
		s.player.AdvanceAnimation(deltaMs, frames)

		// Update cell position
		s.TileX, s.TileY = s.player.CurrentCell()
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

// handlePlayerMove processes ZC_NOTIFY_PLAYERMOVE — the server confirming
// our own walk request. It sends only the start and end cells, because both
// sides are expected to derive the same route with the same rules: A* over
// the GAT, diagonals allowed only when both adjacent cells are open. We do
// exactly that and then walk it on the server's clock (`speed` ms per cell),
// so our position stays in step with the server's instead of beelining.
func (s *InGameState) handlePlayerMove(data []byte) error {
	mv := packets.DecodePlayerMove(data)
	if mv == nil {
		trace.Emit(trace.Move, "ack-undecodable",
			zap.Int("bytes", len(data)),
			zap.String("raw", fmt.Sprintf("% X", data)))
		return fmt.Errorf("invalid ZC_NOTIFY_PLAYERMOVE: %d bytes", len(data))
	}

	if trace.On(trace.Move) {
		curX, curY := 0, 0
		if s.player != nil {
			curX, curY = s.player.CurrentCell()
		}
		trace.Emit(trace.Move, "ack",
			zap.Uint32("startTick", mv.StartTick),
			zap.Int("startX", mv.StartX), zap.Int("startY", mv.StartY),
			zap.Int("endX", mv.EndX), zap.Int("endY", mv.EndY),
			zap.Int("weThinkCellX", curX), zap.Int("weThinkCellY", curY),
			zap.String("raw", fmt.Sprintf("% X", data)))
	}

	if s.player == nil {
		return nil
	}

	path := s.pathFinder.FindPath(mv.StartX, mv.StartY, mv.EndX, mv.EndY)
	if len(path) < 2 {
		// No GAT, or our walkability disagrees with the server's. Walk the
		// straight cell line instead — the route may clip a corner, but the
		// timing and endpoint still match what the server expects.
		logger.Debug("no A* path for server walk, using straight line",
			zap.Int("startX", mv.StartX), zap.Int("startY", mv.StartY),
			zap.Int("endX", mv.EndX), zap.Int("endY", mv.EndY))
		path = entity.CellLine(mv.StartX, mv.StartY, mv.EndX, mv.EndY)
	}

	logger.Debug("player walk-OK",
		zap.Uint32("startTick", mv.StartTick),
		zap.Int("startX", mv.StartX),
		zap.Int("startY", mv.StartY),
		zap.Int("endX", mv.EndX),
		zap.Int("endY", mv.EndY),
		zap.Int("cells", len(path)))

	if trace.On(trace.Move) {
		straight, diagonal := 0, 0
		for i := 1; i < len(path); i++ {
			if path[i][0] != path[i-1][0] && path[i][1] != path[i-1][1] {
				diagonal++
			} else {
				straight++
			}
		}
		expected := float32(straight)*s.player.WalkSpeedMs +
			float32(diagonal)*s.player.WalkSpeedMs*entity.DiagonalCostFactor
		trace.Emit(trace.Move, "path",
			zap.Int("cells", len(path)),
			zap.Int("straight", straight), zap.Int("diagonal", diagonal),
			zap.Float32("expectedMs", expected),
			zap.String("first", fmt.Sprintf("%v", path[0])),
			zap.String("last", fmt.Sprintf("%v", path[len(path)-1])))
	}

	s.player.FollowPath(path)

	if trace.On(trace.Move) {
		trace.Emit(trace.Move, "walk-start",
			zap.Int("facing", s.player.Direction),
			zap.Float32("renderX", s.player.RenderX),
			zap.Float32("renderZ", s.player.RenderZ),
			zap.Float32("worldX", s.player.WorldX),
			zap.Float32("worldZ", s.player.WorldZ))
	}
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

// StepToward asks the server to walk one cell in the direction of the given
// world-space vector, which the caller has already rotated into the camera's
// frame so "forward" means away from the viewer.
//
// Keyboard walking in RO is not free movement — it is a walk request per cell,
// same as a click. Issuing one step at a time keeps us on the server's path
// and stops the moment the key is released. Requests are skipped while a walk
// is already in flight so we don't spam the server mid-step.
func (s *InGameState) StepToward(dirX, dirZ float32) error {
	if s.player == nil || s.player.IsWalkingPath() {
		return nil
	}
	if dirX == 0 && dirZ == 0 {
		return nil
	}

	// Rate-limit. IsWalkingPath only goes true once the server's ack comes
	// back, so between sending a request and hearing about it we'd fire one
	// per frame — 40+ walk requests a second, each restarting the walk server
	// side so the character never finishes a step. One request per
	// moveTickRate is plenty to walk continuously at 150ms per cell.
	if time.Since(s.lastMoveSent) < s.moveTickRate {
		return nil
	}

	dir := entity.CalculateDirection(dirX, dirZ)
	dx, dy := entity.CellDeltaForDirection(dir)
	if dx == 0 && dy == 0 {
		return nil
	}

	cellX, cellY := s.player.CurrentCell()
	targetX, targetY := cellX+dx, cellY+dy

	// Don't bother the server with a step into a wall.
	if s.pathFinder != nil && !s.pathFinder.IsWalkable(targetX, targetY) {
		trace.Emit(trace.Move, "step-blocked",
			zap.Int("cellX", cellX), zap.Int("cellY", cellY),
			zap.Int("targetX", targetX), zap.Int("targetY", targetY),
			zap.Int("facing", dir))
		return nil
	}

	trace.Emit(trace.Move, "step",
		zap.Float32("inputX", dirX), zap.Float32("inputZ", dirZ),
		zap.Int("facing", dir),
		zap.Int("cellX", cellX), zap.Int("cellY", cellY),
		zap.Int("targetX", targetX), zap.Int("targetY", targetY))

	return s.RequestMove(targetX, targetY)
}

// ScreenToTile maps a screen-space click (in viewport pixels) to a tile
// coordinate by ray-casting against the y=0 ground plane using the most
// recent view-projection matrix the scene rendered with.
//
// Returns ok=false if the scene hasn't rendered yet, or if the ray points
// away from the ground (e.g. clicking the sky).
func (s *InGameState) ScreenToTile(screenX, screenY, viewportW, viewportH float32) (tileX, tileY int, ok bool) {
	if s.scene == nil || viewportW <= 0 || viewportH <= 0 {
		trace.Emit(trace.Pick, "reject", zap.Bool("hasScene", s.scene != nil),
			zap.Float32("viewportW", viewportW), zap.Float32("viewportH", viewportH))
		return 0, 0, false
	}
	invViewProj := s.scene.LastViewProj().Inverse()
	ray := picking.ScreenToRay(screenX, screenY, viewportW, viewportH, invViewProj)
	groundY := float32(0)
	if s.player != nil {
		groundY = s.player.RenderY
	}
	worldX, worldZ, hit := ray.IntersectPlaneY(groundY)

	if trace.On(trace.Pick) {
		trace.Emit(trace.Pick, "ray",
			zap.Float32("screenX", screenX), zap.Float32("screenY", screenY),
			zap.Float32("viewportW", viewportW), zap.Float32("viewportH", viewportH),
			zap.Float32("originX", ray.Origin[0]), zap.Float32("originY", ray.Origin[1]),
			zap.Float32("originZ", ray.Origin[2]),
			zap.Float32("dirX", ray.Direction[0]), zap.Float32("dirY", ray.Direction[1]),
			zap.Float32("dirZ", ray.Direction[2]),
			zap.Float32("groundY", groundY),
			zap.Bool("hit", hit))
	}

	if !hit {
		return 0, 0, false
	}
	cellX, cellY := entity.WorldToCell(worldX, worldZ)

	if trace.On(trace.Pick) {
		playerCellX, playerCellY := 0, 0
		if s.player != nil {
			playerCellX, playerCellY = s.player.CurrentCell()
		}
		walkable := s.pathFinder == nil || s.pathFinder.IsWalkable(cellX, cellY)
		trace.Emit(trace.Pick, "hit",
			zap.Float32("worldX", worldX), zap.Float32("worldZ", worldZ),
			zap.Int("cellX", cellX), zap.Int("cellY", cellY),
			zap.Int("playerCellX", playerCellX), zap.Int("playerCellY", playerCellY),
			zap.Bool("walkable", walkable))
	}
	return cellX, cellY, true
}

// RequestMove sends a movement request to the server.
func (s *InGameState) RequestMove(tileX, tileY int) error {
	pkt := &packets.MoveRequest{
		PacketID: packets.CZ_REQUEST_MOVE,
	}
	pkt.SetDestination(tileX, tileY)

	if trace.On(trace.Move) {
		fromX, fromY := 0, 0
		if s.player != nil {
			fromX, fromY = s.player.CurrentCell()
		}
		trace.Emit(trace.Move, "request",
			zap.Int("fromCellX", fromX), zap.Int("fromCellY", fromY),
			zap.Int("toCellX", tileX), zap.Int("toCellY", tileY),
			zap.String("packet", fmt.Sprintf("0x%04X", packets.CZ_REQUEST_MOVE)),
			zap.String("bytes", fmt.Sprintf("% X", pkt.Encode())))
	}

	if err := s.client.Send(pkt.Encode()); err != nil {
		trace.Emit(trace.Move, "request-failed", zap.Error(err))
		return fmt.Errorf("send move request: %w", err)
	}

	// No local prediction: we start walking when ZC_NOTIFY_PLAYERMOVE comes
	// back, so the route and its timing are the server's, not a guess we'd
	// have to reconcile. The server also gets to refuse the move outright.
	s.lastMoveTick = uint32(time.Now().UnixMilli() & 0xFFFFFFFF)
	s.lastMoveSent = time.Now()
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

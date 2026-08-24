// Package states implements game state management.
package states

import (
	"fmt"
	gomath "math"
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
	lastMoveTick    uint32
	lastMoveSent    time.Time
	lastWalkEnded   time.Time
	wasWalking      bool
	keepAliveSentAt time.Time
	pingMs          float64

	// Click-to-move destination. The server will only path a limited distance
	// per request, so a far click is walked in stages toward this.
	destCellX, destCellY   int
	hasDest                bool
	chainCellX, chainCellY int
	moveTickRate           time.Duration
	lastKeepAlive          time.Time
	keepAliveInterval      time.Duration
	enterTime              time.Time // Used as the local epoch for ClientTick

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

		// Note when a walk finishes so the next one can report how long the
		// character stood idle in between.
		walking := s.player.IsWalkingPath()
		if s.wasWalking && !walking {
			s.lastWalkEnded = time.Now()
			trace.Emit(trace.Move, "walk-end")
			s.continueToDestination()
		}
		s.wasWalking = walking

		// Advance the sprite animation. Frame counts come from the loaded
		// sheet; with no sprites this parks on frame 0 harmlessly.
		idleFrames, walkFrames := 0, 0
		if s.playerRender != nil {
			idleFrames = s.playerRender.FrameCount(entity.ActionIdle, s.player.Direction)
			walkFrames = s.playerRender.FrameCount(entity.ActionWalk, s.player.Direction)
		}
		s.player.AdvanceAnimation(deltaMs, idleFrames, walkFrames)

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
	s.client.RegisterHandler(packets.ZC_NOTIFY_TIME, s.handleServerTick)
}

// sendKeepAlive sends CZ_REQUEST_TIME so the map server doesn't time us out.
func (s *InGameState) sendKeepAlive() {
	pkt := &packets.TickSend{
		PacketID:   packets.CZ_REQUEST_TIME,
		ClientTick: uint32(time.Since(s.enterTime).Milliseconds()),
	}
	s.keepAliveSentAt = time.Now()
	if err := s.client.Send(pkt.Encode()); err != nil {
		logger.Warn("keep-alive send failed", zap.Error(err))
	}
}

// handleServerTick measures the round trip on ZC_NOTIFY_TIME. The keep-alive
// is the only exchange with an unambiguous request/response pairing, so it is
// the honest place to measure latency — walk acknowledgements can't be matched
// to their request when several are in flight.
func (s *InGameState) handleServerTick(_ []byte) error {
	if s.keepAliveSentAt.IsZero() {
		return nil
	}
	rtt := float64(time.Since(s.keepAliveSentAt).Microseconds()) / 1000
	s.pingMs = rtt
	trace.Emit(trace.Move, "ping", zap.Float64("rttMs", rtt))
	return nil
}

// PingMs returns the last measured round-trip time in milliseconds.
func (s *InGameState) PingMs() float64 {
	return s.pingMs
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
		// Round trip as the player experiences it: from putting the request on
		// the wire to acting on the reply. Includes our own frame quantisation,
		// which on a local server dominates the network itself.
		rtt := float64(0)
		if !s.lastMoveSent.IsZero() {
			rtt = float64(time.Since(s.lastMoveSent).Microseconds()) / 1000
		}
		trace.Emit(trace.Move, "ack",
			zap.Float64("rttMs", rtt),
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

	stallMs := float64(0)
	if !s.lastWalkEnded.IsZero() {
		stallMs = float64(time.Since(s.lastWalkEnded).Microseconds()) / 1000
	}

	s.player.FollowPath(path)

	if trace.On(trace.Move) {
		// How long the character stood still between the previous walk ending
		// and this one starting. Anything above a frame is visible as a hitch.
		trace.Emit(trace.Move, "stall", zap.Float64("idleMs", stallMs))
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

	// Taking manual control abandons wherever the last click was headed.
	s.hasDest = false

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
// MaxWalkRequestCells is the furthest a single walk request may reach.
//
// rAthena's nominal ceiling is MAX_WALKPATH, 32 cells. The real one is lower:
// path_search keeps known nodes in a fixed 1024-entry table indexed by
// (x + 32*y) & 1023, and the FIXME above that array in path.cpp admits it is
// "too small to ensure all paths shorter than MAX_WALKPATH can be found
// without node collision". Searching much past ~17 cells explores more cells
// than the table holds, collisions become certain and the search gives up —
// and a refused walk is answered with no packet at all, so the client simply
// appears to ignore the click.
//
// Measured against this server: requests up to 17 cells were acknowledged,
// every one of 18 or more was dropped. We stay well under that, because the
// limit is on path length and a route around an obstacle is longer than the
// straight-line distance suggests. Longer walks are chained, one request at a
// time, which is what the official client does.
const MaxWalkRequestCells = 12

// clampWalkRequest pulls a destination back along the line from the character
// until it is close enough for the server to path to, preserving the direction
// so the character still sets off towards where the player clicked.
func clampWalkRequest(fromX, fromY, toX, toY int) (int, int) {
	dx, dy := toX-fromX, toY-fromY

	reach := abs(dx)
	if abs(dy) > reach {
		reach = abs(dy)
	}
	if reach <= MaxWalkRequestCells {
		return toX, toY
	}

	scale := float64(MaxWalkRequestCells) / float64(reach)
	return fromX + int(gomath.Round(float64(dx)*scale)),
		fromY + int(gomath.Round(float64(dy)*scale))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// RequestMove asks the server to walk to a cell, remembering it as the
// player's actual intent so a destination beyond one request's reach can be
// walked in stages.
func (s *InGameState) RequestMove(tileX, tileY int) error {
	s.destCellX, s.destCellY = tileX, tileY
	s.hasDest = true
	s.chainCellX, s.chainCellY = -1, -1
	return s.sendWalkRequest(tileX, tileY)
}

// sendWalkRequest sends one walk packet toward a cell, clamped to a distance
// the server will actually path.
func (s *InGameState) sendWalkRequest(tileX, tileY int) error {
	fromX, fromY := 0, 0
	if s.player != nil {
		fromX, fromY = s.player.CurrentCell()
	}

	reqX, reqY := clampWalkRequest(fromX, fromY, tileX, tileY)

	pkt := &packets.MoveRequest{
		PacketID: packets.CZ_REQUEST_MOVE,
	}
	pkt.SetDestination(reqX, reqY)

	if trace.On(trace.Move) {
		trace.Emit(trace.Move, "request",
			zap.Int("fromCellX", fromX), zap.Int("fromCellY", fromY),
			zap.Int("toCellX", reqX), zap.Int("toCellY", reqY),
			zap.Int("wantCellX", tileX), zap.Int("wantCellY", tileY),
			zap.Bool("clamped", reqX != tileX || reqY != tileY),
			zap.String("packet", fmt.Sprintf("0x%04X", packets.CZ_REQUEST_MOVE)),
			zap.String("bytes", fmt.Sprintf("% X", pkt.Encode())))
	}

	if err := s.client.Send(pkt.Encode()); err != nil {
		trace.Emit(trace.Move, "request-failed", zap.Error(err))
		return fmt.Errorf("send move request: %w", err)
	}

	s.lastMoveTick = uint32(time.Now().UnixMilli() & 0xFFFFFFFF)
	s.lastMoveSent = time.Now()
	return nil
}

// continueToDestination is called when a walk finishes. If the player asked to
// go further than one request could carry them, this sends the next leg.
func (s *InGameState) continueToDestination() {
	if !s.hasDest || s.player == nil {
		return
	}

	cellX, cellY := s.player.CurrentCell()
	if cellX == s.destCellX && cellY == s.destCellY {
		s.hasDest = false
		return
	}

	// If the last leg left us exactly where it started, the destination is one
	// the server will not walk us to. Stop rather than asking forever.
	if cellX == s.chainCellX && cellY == s.chainCellY {
		trace.Emit(trace.Move, "chain-stalled",
			zap.Int("cellX", cellX), zap.Int("cellY", cellY),
			zap.Int("destX", s.destCellX), zap.Int("destY", s.destCellY))
		s.hasDest = false
		return
	}

	s.chainCellX, s.chainCellY = cellX, cellY
	trace.Emit(trace.Move, "chain",
		zap.Int("cellX", cellX), zap.Int("cellY", cellY),
		zap.Int("destX", s.destCellX), zap.Int("destY", s.destCellY))

	if err := s.sendWalkRequest(s.destCellX, s.destCellY); err != nil {
		logger.Warn("chained walk request failed", zap.Error(err))
		s.hasDest = false
	}
}

// ClearDestination forgets the click-to-move destination, so a keyboard step
// or a new click doesn't get overridden by the previous walk continuing.
func (s *InGameState) ClearDestination() {
	s.hasDest = false
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

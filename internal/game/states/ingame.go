// Package states implements game state management.
package states

import (
	"fmt"
	gomath "math"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/config"
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

	// chat is the scrollback the chat box shows.
	chat ChatLog

	// unitTraceAt rate limits the unit render statistics.
	unitTraceAt time.Time

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

	// The walk we started on our own authority, so its acknowledgement can be
	// recognized and ignored rather than restarting the walk.
	predictStartX, predictStartY int
	predictEndX, predictEndY     int
	hasPrediction                bool
	predictions, predictionHits  int
	moveTickRate                 time.Duration
	lastKeepAlive                time.Time
	keepAliveInterval            time.Duration
	enterTime                    time.Time // Used as the local epoch for ClientTick

	// The player's own numbers, seeded from the character list and kept
	// current by the server's parameter packets.
	stats PlayerStats

	// The conversation in progress, if any.
	dialog NPCDialog

	// Parameter ids we do not track, remembered so each is reported once
	// rather than on every update the server sends for it.
	unknownStats map[uint16]bool

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

	// The map is up, so its background music replaces the title theme and
	// repeats until the player leaves for another map.
	s.manager.PlayLocationBGM(s.MapName)

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
		s.player.TerrainHeight = s.terrainHeight
	}

	logger.Info("player walk speed",
		zap.Float32("msPerCell", s.player.WalkSpeedMs),
		zap.Bool("hasPathfinder", s.pathFinder != nil))

	s.stats = PlayerStatsFromChar(s.CharInfo())
	s.traceInitialStats()

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
	s.camera.Distance = DefaultCameraZoom
	s.camera.Yaw = 0

	// Pick up where the last session left the zoom, if it was remembered and
	// is still a distance the camera accepts — a stale file should not be able
	// to put the camera somewhere unusable.
	if z := config.LoadUIState().CameraZoom; z >= s.camera.MinDistance && z <= s.camera.MaxDistance {
		s.camera.Distance = z
		logger.Debug("restored camera zoom", zap.Float32("distance", z))
	}

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
	s.SaveUIState()

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

	// Map models with animation — windmills, clock towers — are rebuilt here.
	if s.scene != nil && s.MapLoaded {
		s.scene.AdvanceModels(deltaMs)
	}

	// Update all entities
	s.entityManager.Update(dt)
	updateUnits(s.entityManager, deltaMs, s.unitAnim)

	return nil
}

// unitAnim reports a unit's animation length and frame rate, so it loops at
// the length of its own sprite and runs at the rate its own ACT specifies
// rather than the player's fixed one. Zero until the sheet for that appearance
// has been baked, which parks it on frame 0.
func (s *InGameState) unitAnim(e *entity.Entity, action, direction int) (int, float32) {
	if s.playerRender == nil || !unitIsDrawable(e) {
		return 0, 0
	}
	spec := unitSpec(e)
	return s.playerRender.UnitFrameCount(spec, action, direction),
		s.playerRender.UnitFrameInterval(spec, action)
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
		if s.playerRender == nil {
			return
		}
		s.playerRender.Render(viewProj, s.player, s.camera.PosX, s.camera.PosZ)
		s.renderUnits(viewProj)
	})
	return nil
}

// renderUnits draws the other units on the map, sharing the player's billboard
// renderer and its sheet cache.
func (s *InGameState) renderUnits(viewProj math.Mat4) {
	if s.entityManager == nil || s.manager == nil {
		return
	}

	load := charsprite.Loader(s.manager.TexLoader)
	tracked, drawn := 0, 0
	for _, e := range s.entityManager.All() {
		if e.Body == nil {
			continue
		}
		tracked++
		if !unitIsDrawable(e) {
			continue
		}
		drawn++
		s.playerRender.RenderUnit(viewProj, e.Body, s.camera.PosX, s.camera.PosZ, load, unitSpec(e), e.Alpha())
	}

	s.traceUnitStats(tracked, drawn)
}

// traceUnitStats reports how many units we hold against how many we draw, once
// a second.
//
// The gap between the two is the only way to tell a unit we failed to draw
// from one the server never mentioned: it only sends what is within its
// area_size — 14 cells by default — while the camera can see several times
// that. A town looking sparse is usually that limit, not a bug, and these two
// numbers are what distinguish them.
func (s *InGameState) traceUnitStats(tracked, drawn int) {
	if !trace.On(trace.Render) || time.Since(s.unitTraceAt) < time.Second {
		return
	}
	s.unitTraceAt = time.Now()

	sheets := 0
	if s.playerRender != nil {
		sheets = s.playerRender.CachedUnitSheets()
	}
	trace.Emit(trace.Render, "units",
		zap.Int("tracked", tracked),
		zap.Int("drawn", drawn),
		zap.Int("undrawable", tracked-drawn),
		zap.Int("sheets", sheets))
}

// GetSceneTexture returns the rendered scene texture ID for display.
func (s *InGameState) GetSceneTexture() uint32 {
	if s.scene != nil {
		return s.scene.ColorTexture()
	}
	return 0
}

// DefaultCameraZoom is the starting third-person distance: RO-style and close
// in, matching grfbrowser's play mode.
const DefaultCameraZoom = 145

// CameraZoom returns the current camera distance.
func (s *InGameState) CameraZoom() float32 {
	if s.camera == nil {
		return DefaultCameraZoom
	}
	return s.camera.Distance
}

// SaveUIState records the parts of the session worth remembering. Called on
// the way out; a failure here is worth a line in the log and nothing more.
func (s *InGameState) SaveUIState() {
	if s.camera == nil {
		return
	}
	if err := config.SaveUIState(config.UIState{CameraZoom: s.camera.Distance}); err != nil {
		logger.Warn("could not save ui state", zap.Error(err))
	}
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

// traceInitialStats records the stats we enter the map with. Everything the
// server pushes afterwards is a delta against these, so a trace that starts
// without them cannot be read: a later HP of 40 means nothing unless you know
// whether it went up or down.
func (s *InGameState) traceInitialStats() {
	if !trace.On(trace.Status) {
		return
	}

	if s.CharInfo() == nil {
		trace.Emit(trace.Status, "initial", zap.String("source", "none"))
		return
	}

	trace.Emit(trace.Status, "initial",
		zap.String("source", "charinfo"),
		zap.Int("hp", s.stats.HP),
		zap.Int("maxHP", s.stats.MaxHP),
		zap.Int("sp", s.stats.SP),
		zap.Int("maxSP", s.stats.MaxSP),
		zap.Int("baseLevel", s.stats.BaseLevel),
		zap.Int("jobLevel", s.stats.JobLevel),
		zap.Int("class", s.stats.Class))
}

// Stats returns the player's current numbers.
func (s *InGameState) Stats() PlayerStats {
	if s == nil {
		return PlayerStats{}
	}

	return s.stats
}

// handleStatusChange applies one parameter update from the server.
//
// These arrive constantly — every point of HP regenerated is one — so the
// handler stays cheap and says nothing at info level; --trace=status is how
// you watch them.
func (s *InGameState) handleStatusChange(data []byte) error {
	change := packets.DecodeStatusChange(data)
	if change == nil {
		logger.Warn("malformed status packet", zap.Int("bytes", len(data)))
		return nil
	}

	if !s.stats.Apply(change.VarID, change.Value) {
		s.reportUnknownStat(change.VarID, change.Value)
		return nil
	}

	trace.Emit(trace.Status, "change",
		zap.Uint16("varID", change.VarID),
		zap.Int64("value", change.Value))

	return nil
}

// reportUnknownStat notes a parameter the client does not track, once per id.
//
// Most are genuinely not ours — the server sends every combat stat down the
// same pipe. The point of saying it at all is that a parameter which *should*
// be on the panel would otherwise go missing without a word.
func (s *InGameState) reportUnknownStat(varID uint16, value int64) {
	if s.unknownStats == nil {
		s.unknownStats = make(map[uint16]bool)
	}

	if s.unknownStats[varID] {
		return
	}
	s.unknownStats[varID] = true

	trace.Emit(trace.Status, "unknown",
		zap.Uint16("varID", varID),
		zap.Int64("value", value))
}

// CharInfo returns the character being played as character select last
// described it. Until the status packets are parsed this is the only source of
// the player's stats, and afterwards it is what those packets are checked
// against — the two disagreeing means the CharInfo struct is being read at the
// wrong offsets.
func (s *InGameState) CharInfo() *packets.CharInfo {
	if s == nil || s.manager == nil {
		return nil
	}

	return s.manager.Session.Char
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
	s.client.RegisterHandler(packets.ZC_NOTIFY_STANDENTRY, s.handleEntityIdle)
	s.client.RegisterHandler(packets.ZC_NOTIFY_NEWENTRY, s.handleEntitySpawn)
	s.client.RegisterHandler(packets.ZC_NOTIFY_MOVEENTRY, s.handleEntityMove)
	s.client.RegisterHandler(packets.ZC_NOTIFY_VANISH, s.handleEntityVanish)
	s.client.RegisterHandler(packets.ZC_NPCACK_MAPMOVE, s.handleMapChange)
	s.client.RegisterHandler(packets.ZC_NOTIFY_PLAYERMOVE, s.handlePlayerMove)
	s.client.RegisterHandler(packets.ZC_NOTIFY_TIME, s.handleServerTick)
	s.client.RegisterHandler(packets.ZC_NOTIFY_CHAT, s.handleChat)
	s.client.RegisterHandler(packets.ZC_NOTIFY_PLAYERCHAT, s.handlePlayerChat)
	s.client.RegisterHandler(packets.ZC_BROADCAST, s.handleBroadcast)
	s.client.RegisterHandler(packets.ZC_PAR_CHANGE, s.handleStatusChange)
	s.client.RegisterHandler(packets.ZC_LONGPAR_CHANGE, s.handleStatusChange)
	s.client.RegisterHandler(packets.ZC_LONGLONGPAR_CHANGE, s.handleStatusChange)
	s.client.RegisterHandler(packets.ZC_SAY_DIALOG, s.handleSayDialog)
	s.client.RegisterHandler(packets.ZC_WAIT_DIALOG, s.handleWaitDialog)
	s.client.RegisterHandler(packets.ZC_CLOSE_DIALOG, s.handleCloseDialog)
	s.client.RegisterHandler(packets.ZC_MENU_LIST, s.handleMenuList)
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

	// If this is the walk we already started ourselves, we are on it — leave
	// it alone. Re-issuing an identical path would restart the current step
	// and undo the point of predicting.
	if s.hasPrediction && s.ackMatchesPrediction(mv) {
		s.hasPrediction = false
		s.predictionHits++
		trace.Emit(trace.Move, "ack-confirms-prediction",
			zap.Int("startDelta", cellsApart(mv.StartX, mv.StartY, s.predictStartX, s.predictStartY)),
			zap.Int("hits", s.predictionHits), zap.Int("total", s.predictions))
		return nil
	}

	if s.hasPrediction {
		trace.Emit(trace.Move, "ack-corrects-prediction",
			zap.Int("startDelta", cellsApart(mv.StartX, mv.StartY, s.predictStartX, s.predictStartY)),
			zap.Bool("endDiffers", mv.EndX != s.predictEndX || mv.EndY != s.predictEndY),
			zap.Int("predictedStartX", s.predictStartX), zap.Int("predictedStartY", s.predictStartY),
			zap.Int("predictedEndX", s.predictEndX), zap.Int("predictedEndY", s.predictEndY),
			zap.Int("serverStartX", mv.StartX), zap.Int("serverStartY", mv.StartY),
			zap.Int("serverEndX", mv.EndX), zap.Int("serverEndY", mv.EndY))
	}
	s.hasPrediction = false

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

// handleEntityIdle handles a unit standing still (ZC_NOTIFY_STANDENTRY).
func (s *InGameState) handleEntityIdle(data []byte) error {
	return s.applyUnit(packets.DecodeEntityIdle(data), "idle")
}

// handleEntitySpawn handles a unit appearing (ZC_NOTIFY_NEWENTRY).
func (s *InGameState) handleEntitySpawn(data []byte) error {
	return s.applyUnit(packets.DecodeEntitySpawn(data), "spawn")
}

// handleEntityMove handles a unit that is walking (ZC_NOTIFY_MOVEENTRY).
func (s *InGameState) handleEntityMove(data []byte) error {
	return s.applyUnit(packets.DecodeEntityWalk(data), "walk")
}

// handleEntityVanish removes a unit that has left our view, died, logged out,
// teleported or is playing dead.
func (s *InGameState) handleEntityVanish(data []byte) error {
	aid, reason, ok := packets.DecodeEntityVanish(data)
	if !ok {
		return nil
	}
	if aid == s.selfAID() {
		// The server reports our own death and teleports through this packet
		// too. Removing ourselves would delete the character being played, so
		// only note it.
		trace.Emit(trace.Net, "vanish-self", zap.Uint8("reason", reason))
		return nil
	}

	removeUnit(s.entityManager, aid)
	trace.Emit(trace.Net, "vanish",
		zap.Uint32("aid", aid),
		zap.Uint8("reason", reason),
		zap.Int("units", s.entityManager.Count()))
	return nil
}

// applyUnit folds one decoded unit report into the entity registry.
//
// Reports about our own character are dropped: the local player is driven by
// its own prediction and acknowledgement path, and letting a unit report move
// it would fight that.
func (s *InGameState) applyUnit(u *packets.Entity, kind string) error {
	if u == nil {
		return nil
	}
	if u.AID == s.selfAID() {
		return nil
	}

	e := upsertUnit(s.entityManager, u, s.unitPath)
	if e == nil {
		return nil
	}
	if e.Body != nil {
		e.Body.TerrainHeight = s.terrainHeight
	}

	// sheets says whether the appearance actually baked, which is what
	// separates "the packet never arrived" from "it arrived and nothing was
	// drawn" when a unit is missing from the map.
	sheets := 0
	if s.playerRender != nil {
		sheets = s.playerRender.CachedUnitSheets()
	}

	trace.Emit(trace.Net, "unit",
		zap.String("kind", kind),
		zap.Uint32("aid", u.AID),
		zap.Uint8("objectType", uint8(u.Kind)),
		// The job id is what names the sprite, so it is the first thing needed
		// when a unit turns up undrawable.
		zap.Int16("job", u.Job),
		zap.String("name", u.Name),
		zap.Int("x", u.X), zap.Int("y", u.Y),
		zap.Bool("moving", u.Moving),
		zap.Bool("drawable", unitIsDrawable(e)),
		zap.Int("units", s.entityManager.Count()),
		zap.Int("sheets", sheets))
	return nil
}

// handleChat handles a line someone else spoke.
func (s *InGameState) handleChat(data []byte) error {
	return s.addChat(packets.DecodeChat(data))
}

// handlePlayerChat handles our own line echoed back, and the messages the
// server sends us directly — rAthena's welcome lines arrive this way.
func (s *InGameState) handlePlayerChat(data []byte) error {
	return s.addChat(packets.DecodePlayerChat(data))
}

// handleBroadcast handles a server-wide announcement.
func (s *InGameState) handleBroadcast(data []byte) error {
	return s.addChat(packets.DecodeBroadcast(data))
}

// addChat folds one decoded message into the scrollback.
func (s *InGameState) addChat(msg *packets.ChatMessage) error {
	if msg == nil {
		return nil
	}

	s.chat.Add(msg)

	trace.Emit(trace.HUD, "chat",
		zap.Uint8("kind", uint8(msg.Kind)),
		zap.String("speaker", msg.Speaker),
		zap.Int("bytes", len(msg.Text)),
		zap.Int("lines", s.chat.Len()))

	return nil
}

// ChatLines returns the chat scrollback for the UI, oldest first.
func (s *InGameState) ChatLines() []ChatLine {
	return s.chat.Lines()
}

// selfAID returns our own account id, which is what identifies our character
// among the units the server reports.
func (s *InGameState) selfAID() uint32 {
	if s.client == nil {
		return 0
	}
	accountID, _, _, _ := s.client.Session()
	return accountID
}

// terrainHeight returns the ground altitude at a world position, so units
// follow the terrain as they walk rather than sinking through hills. Returns
// zero before the map is loaded, which is what a flat map would give.
func (s *InGameState) terrainHeight(worldX, worldZ float32) float32 {
	if s.scene == nil || !s.MapLoaded {
		return 0
	}
	return s.scene.GetTerrainHeight(worldX, worldZ)
}

// unitPath reproduces the route a unit walks. The server sends only the
// endpoints, so units have to be walked over the same cells the server thinks
// they are walking or the two drift apart around obstacles.
func (s *InGameState) unitPath(fromX, fromY, toX, toY int) [][2]int {
	if s.pathFinder == nil {
		return nil
	}
	return s.pathFinder.FindPath(fromX, fromY, toX, toY)
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

// ClickWorld handles a left click that landed on the world rather than on the
// interface.
//
// It exists so there is one place that decides what a click means. Today that
// decision is only "walk there"; entity picking goes in front of it, and
// having the decision in the state — where the entities and the connection
// already are — is what lets it, without the game loop growing a second copy
// of the ray cast.
func (s *InGameState) ClickWorld(mouseX, mouseY, viewportW, viewportH float32) {
	// An NPC under the pointer takes the click. Walking there instead would be
	// the wrong thing twice over: the conversation would not start, and the
	// server would refuse a step into the cell the NPC is standing on.
	if npc := s.PickEntity(mouseX, mouseY, viewportW, viewportH); npc != nil {
		trace.Emit(trace.NPC, "click",
			zap.Uint32("npcID", npc.ID), zap.String("name", npc.Name),
			zap.Float32("screenX", mouseX), zap.Float32("screenY", mouseY))

		s.ContactNPC(npc)

		return
	}

	tileX, tileY, ok := s.ScreenToTile(mouseX, mouseY, viewportW, viewportH)
	if !ok {
		trace.Emit(trace.Pick, "miss")
		return
	}

	if err := s.RequestMove(tileX, tileY); err != nil {
		logger.Warn("click-to-move RequestMove failed", zap.Error(err))
	}
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

	s.predictWalk(fromX, fromY, reqX, reqY)
	return nil
}

// predictWalk starts walking immediately rather than waiting for the server to
// agree.
//
// Both sides derive the route the same way — A* over the same GAT, diagonals
// only where both neighbors are open — and walk it at the same ms per cell,
// so the prediction is normally exactly what comes back. Waiting for the
// acknowledgement instead costs a round trip on every walk, which the
// character can never make up: it renders permanently behind the server and
// each correction drags it forward.
//
// When the acknowledgement matches, there is nothing to do. When it doesn't,
// the server wins and the difference is absorbed by the visual offset, so a
// wrong guess costs a short catch-up rather than a jump.
func (s *InGameState) predictWalk(fromX, fromY, toX, toY int) {
	s.hasPrediction = false
	if s.player == nil {
		return
	}

	path := s.pathFinder.FindPath(fromX, fromY, toX, toY)
	if len(path) < 2 {
		// Nothing we can walk; wait and see what the server says.
		return
	}

	s.predictStartX, s.predictStartY = fromX, fromY
	s.predictEndX, s.predictEndY = toX, toY
	s.hasPrediction = true
	s.predictions++

	s.player.FollowPath(path)

	trace.Emit(trace.Move, "predict",
		zap.Int("fromX", fromX), zap.Int("fromY", fromY),
		zap.Int("toX", toX), zap.Int("toY", toY),
		zap.Int("cells", len(path)))
}

// PredictionStartTolerance is how far the server's idea of where a walk began
// may sit from ours before the prediction counts as wrong, in cells.
//
// Prediction inherently runs ahead of the server: we set off on the input, the
// server sets off when the packet lands, so by the time it answers we have
// already taken the step it is only now starting. The lead is latency
// expressed in cells — a cell is 150ms and a round trip plus a frame is a
// good fraction of that.
//
// Two, not one, and measured rather than guessed. Over 85 acknowledgements
// from a live session the lead was 0 or 1 in 48 cases and 2 in another 28,
// with 3 or more in only nine. It does not accumulate: regressing the lead
// against how long the character had been walking gives a slope of 0.00 cells
// per second, across walks from half a second to seven minutes. Legs chained
// within one walk are exact every time — all eleven of them — because they are
// issued the moment a walk ends, when both sides agree on where we are.
//
// Anything further apart is a real disagreement and the server wins. That
// costs a restarted step, which is smooth now, so erring low is cheap.
const PredictionStartTolerance = 2

// ackMatchesPrediction reports whether an acknowledgement describes the walk we
// already started.
//
// The destination has to match exactly: it is what we asked for and what
// decides where the walk ends. The start is allowed the tolerance above.
func (s *InGameState) ackMatchesPrediction(mv *packets.PlayerMove) bool {
	if mv.EndX != s.predictEndX || mv.EndY != s.predictEndY {
		return false
	}
	return cellsApart(mv.StartX, mv.StartY, s.predictStartX, s.predictStartY) <= PredictionStartTolerance
}

// cellsApart returns the Chebyshev distance between two cells, which is the
// number of steps between them for eight-way movement.
func cellsApart(ax, ay, bx, by int) int {
	dx, dy := abs(ax-bx), abs(ay-by)
	if dx > dy {
		return dx
	}
	return dy
}

// PredictionAccuracy returns how many predicted walks the server confirmed
// unchanged, out of how many were made. Exposed for diagnostics.
func (s *InGameState) PredictionAccuracy() (hits, total int) {
	return s.predictionHits, s.predictions
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

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
	"github.com/Faultbox/midgard-ro/internal/engine/character"
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

	// LoadingImage is which loading screen the handshake already showed, so
	// the load that follows keeps it rather than cutting to another. Zero
	// lets the loader pick.
	LoadingImage int
}

// spawnPoint is where the server put us on a map: a cell, and a facing in
// the server's numbering.
type spawnPoint struct {
	x, y int
	dir  uint8
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

	// portals draws the warp portal effect for every class-45 unit. Like
	// playerRender it outlives the map, so a warp costs no reload.
	portals *scene.PortalRenderer

	// marker is the cell highlight under the cursor. hoverCellX/Y is the cell
	// it sits on and hoverValid whether the cursor is over the ground at all;
	// markerPulse counts down the flourish a click sets off.
	marker     *scene.GroundMarker
	castAura   *scene.GroundMarker
	hoverCellX int
	hoverCellY int
	hoverValid bool

	// hoverEntity is whatever the pointer is over this frame, set by the
	// cursor pass so the label and the cursor cannot disagree.
	hoverEntity *entity.Entity

	// pendingPickup is a ground item being walked to, picked up once the
	// character is close enough. pendingPickupIdleMs counts how long it has
	// stood still on the way, which is how an unreachable item is given up on.
	pendingPickup       uint32
	pendingPickupIdleMs float32

	// targetID is the unit being attacked, and pendingAttack one being walked
	// to before the first blow. pendingAttackIdleMs counts how long the
	// character has stood still on the way, which is how an unreachable
	// target is given up on.
	// targetID is the unit being fought, kept until it dies, leaves, or
	// another click countermands it. attacking records whether the server has
	// been told to swing, and the two timers throttle chasing it and
	// reissuing the blow.
	// attackRange is the reach of the equipped weapon, as the server reports
	// it. Zero until the first ZC_ATTACK_RANGE arrives.
	attackRange int

	// damageNumbers are the figures floating up from recent blows.
	damageNumbers []floatingDamage

	// effects are the STR animations playing over the world, and effectCache
	// the files they were parsed from.
	effects     []*activeEffect
	effectCache map[string]*formats.STR

	// celebrations are level-ups waiting to be shown, and celebrationWaitMs
	// how long before the next may start.
	// groundTraceMs counts frames for the ground trace's throttle, and
	// gridTraced marks the one-off height grid as already printed.
	groundTraceMs int
	gridTraced    bool

	// showEquipment is the server's word on whether other players may look at
	// what this character is wearing.
	showEquipment bool

	// delayedEffects are the ones waiting for their moment — a bolt's shot
	// lands a while after the volley starts.
	delayedEffects []delayedEffect

	// skillUnits are the ground skills standing on the map, by the id their
	// blows arrive from. What is kept is the packet: who placed it, for the
	// battle log, and where and what it is, for drawing it.
	skillUnits map[uint32]packets.SkillUnit

	// spriteEffects are the ones the archive draws frame by frame, and the
	// two caches are the art they are read from.
	spriteEffects []*spriteEffect
	effectACTs    map[string]*formats.ACT
	effectSPRs    map[string]*formats.SPR

	// ambient is the map's own sound sources, each counting down to its next
	// turn.
	ambient []*ambient

	// playerBodyState is the state the character being played is drawn in.
	// Kept here rather than on an entity because the character is not in the
	// registry — it is driven by its own prediction.
	playerBodyState uint16

	// bursts are the particle effects playing — the ones the original draws
	// in code rather than from a file.
	bursts []*activeBurst

	// sounds are what the world wants played this frame.
	sounds []Sound

	celebrations      int
	celebrationWaitMs float32

	// pendingLevelUp and pendingJobLevelUp are levels reached and not yet
	// acknowledged, which the buttons at the foot of the screen offer.
	pendingLevelUp    bool
	pendingJobLevelUp bool

	// unknownJobs are unit job ids the sprite table has no name for, kept so
	// each is complained about once rather than on every sighting.
	unknownJobs map[int]bool

	// placingSkill is a skill chosen and waiting to be aimed, zero when none
	// is, and placingLevel the level it will go off at. placingAtUnit says
	// which it is waiting for: a unit, or a cell.
	placingSkill  uint16
	placingLevel  int
	placingAtUnit bool

	// The cast bar: which skill, how long it takes, and how much is left.
	castSkill   uint16
	castTotalMs float32
	castLeftMs  float32

	// pendingSkill is a cast waiting for the character to walk into range.
	pendingSkill *pendingSkillCast

	// castAuras are the rings under whoever is casting, and holdCastAura keeps
	// one there for --cast-aura.
	castAuras    []castingAura
	holdCastAura bool

	// skillLabels are skill names floating over whoever they were cast on.
	skillLabels []floatingSkillName

	// pendingBlows are swings whose outcome is still traveling: the figure,
	// the flinch and the death wait for the frame the blade lands on.
	pendingBlows []pendingBlow

	targetID    uint32
	attacking   bool
	repathMs    float32
	resendMs    float32
	markerPulse float32

	// markerTraceAt rate limits the marker diagnostics.
	markerTraceAt time.Time
	effectTimeMs  float32

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

	// lastCommand is what became of the most recent command, for the F3
	// overlay. See LastCommand in commands.go for why it is worth keeping.
	lastCommand lastCommand

	// pendingWhisper is the private message waiting on its acknowledgement,
	// which is what says whether it reached anyone. One at a time is enough:
	// the server answers each before the box can send another.
	pendingWhisper pendingWhisper

	// quit ends the process, once the server has agreed to let us go.
	quit QuitFunc

	// skills is what the character can do, as the server listed it, and
	// inventory what it is carrying — the two lists appended together.
	skills    []packets.Skill
	inventory []packets.InventoryItem

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

	// The map being loaded, when one is — see beginMapLoad. While it is set
	// there is no scene, and the loading screen is what the player sees.
	mapLoader     *MapLoader
	loadingScene  *scene.Scene
	mapLoadOrigin string
	pendingSpawn  spawnPoint
	lastLoadMs    float64
	lastLoadPhase string

	// What the current map asks of the camera; see applyMapRules.
	mapCamera MapCamera

	// OnMapChanged is called once a map is up — the first, and every one a
	// warp brings — with its name. The per-map pieces of the HUD hang on it.
	OnMapChanged func(mapName string)

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
//
// Everything that does not depend on the map is set up here: the camera, the
// unit renderer, the player's stats, the packet handlers. The map itself is
// loaded over the following frames by a MapLoader — see beginMapLoad and
// finishMapLoad — so the loading screen can draw between its phases instead
// of the last frame of the previous screen freezing for a second.
func (s *InGameState) Enter() error {
	logger.Info("entering InGameState",
		zap.String("map", s.MapName),
		zap.Int("spawnX", s.config.SpawnX),
		zap.Int("spawnY", s.config.SpawnY))

	s.ErrorMsg = ""

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

	s.loadPortalRenderer()

	s.stats = PlayerStatsFromChar(s.CharInfo())
	s.traceInitialStats()

	// Mark entry time — used as the local epoch for ClientTick and as the
	// gate for the keep-alive ticker (only run after we're actually in-game).
	s.enterTime = time.Now()
	s.lastKeepAlive = s.enterTime

	// The load starts before the handlers exist: registering them delivers
	// whatever was held for us, and the login-time ZC_NPCACK_MAPMOVE is in
	// there. It must find the map already loading, or it reads as a teleport.
	s.beginMapLoad(s.config.MapName, spawnPoint{s.config.SpawnX, s.config.SpawnY, s.config.SpawnDir}, "login")

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
		zap.Int("weaponLook", spec.Weapon),
		zap.String("weapon", s.playerRender.WeaponPath()),
		zap.Int("idleFrames", s.playerRender.FrameCount(entity.ActionIdle, entity.DirS)),
		zap.Int("walkFrames", s.playerRender.FrameCount(entity.ActionWalk, entity.DirS)))
}

// loadPortalRenderer builds the warp portal effect. It needs nothing from
// the archive — the effect is generated — so the only way it fails is the
// shader, and then the warps are still there to walk into.
func (s *InGameState) loadPortalRenderer() {
	pr, err := scene.NewPortalRenderer()
	if err != nil {
		logger.Warn("no warp portal effect", zap.Error(err))
		return
	}
	s.portals = pr

	m, err := scene.NewGroundMarker()
	if err != nil {
		logger.Warn("no click marker", zap.Error(err))

		return
	}
	s.marker = m

	if err := s.marker.LoadTexture(s.manager.TexLoader); err != nil {
		// Warned, not fatal: everything else still works, and a click just
		// goes back to having no feedback.
		logger.Warn("no click marker texture", zap.Error(err))
	}

	aura, err := scene.NewTube(scene.CastAuraTexture, scene.CastAuraTint, scene.CastAuraSides)
	if err != nil {
		logger.Warn("no casting aura", zap.Error(err))

		return
	}
	s.castAura = aura

	if err := s.castAura.LoadTexture(s.manager.TexLoader); err != nil {
		logger.Warn("no casting aura texture", zap.Error(err))
	}
}

// beginMapLoad starts loading a map and drops the one we were on.
//
// The old map's units, scene and any walk in progress go now rather than when
// the new one is ready: the server has already moved us, and nothing about
// where we were is true any more. The character itself survives — it is put
// down again by finishMapLoad — as do the stats, the camera and the packet
// handlers, which is what keeps a warp from being a login.
func (s *InGameState) beginMapLoad(mapName string, at spawnPoint, origin string) {
	from := s.MapName
	if origin == "login" {
		from = ""
	}
	s.MapName = mapName
	s.pendingSpawn = at
	s.mapLoadOrigin = origin
	s.MapLoaded = false
	s.SceneReady = false
	s.StatusMsg = fmt.Sprintf("Loading %s...", packets.MapBaseName(mapName))

	if s.mapLoader != nil {
		// A change ordered while another was still loading: the server's
		// later word wins, and the half-built scene goes with the earlier one.
		s.abortMapLoad()
	}
	if s.scene != nil {
		s.scene.Destroy()
		s.scene = nil
	}
	s.gat = nil
	s.pathFinder = nil
	s.dropDialog()
	s.cancelWalk()
	s.entityManager.Clear()
	s.forgetPendingBlows()

	trace.Emit(trace.Map, "change",
		zap.String("from", from), zap.String("to", mapName),
		zap.Int("x", at.x), zap.Int("y", at.y),
		zap.String("origin", origin), zap.Bool("same", false))

	if s.manager.TexLoader == nil {
		logger.Warn("no asset loader; the map cannot be loaded")
		s.ErrorMsg = "Map not loaded: no asset loader"
		s.placePlayer(at)
		s.sendLoadingComplete()
		return
	}

	sc, err := scene.New(scene.DefaultConfig())
	if err != nil {
		logger.Error("failed to create scene", zap.Error(err))
		s.ErrorMsg = fmt.Sprintf("Failed to create scene: %v", err)
		s.placePlayer(at)
		s.sendLoadingComplete()
		return
	}
	s.loadingScene = sc
	s.mapLoader = NewMapLoader(mapName, s.manager.TexLoader, sc)
	if s.config.LoadingImage > 0 {
		s.mapLoader.ImageIndex = s.config.LoadingImage
		s.config.LoadingImage = 0
	}
}

// abortMapLoad drops a load in progress and the scene it was building.
func (s *InGameState) abortMapLoad() {
	if s.loadingScene != nil {
		s.loadingScene.Destroy()
		s.loadingScene = nil
	}
	s.mapLoader = nil
}

// finishMapLoad installs the loaded map, puts the character on it and tells
// the server we are ready.
//
// On failure the map is simply absent: the character stands on nothing at
// the given cell so the error can be read, and the server is still answered,
// since leaving it waiting would time the session out on top of the error.
func (s *InGameState) finishMapLoad() {
	l := s.mapLoader
	sc := s.loadingScene
	s.mapLoader, s.loadingScene = nil, nil
	at := s.pendingSpawn

	if err := l.Err(); err != nil {
		logger.Error("failed to load map", zap.String("map", l.Name), zap.Error(err))
		s.ErrorMsg = fmt.Sprintf("Map not loaded: %v", err)
		if sc != nil {
			sc.Destroy()
		}
		s.placePlayer(at)
		s.sendLoadingComplete()
		return
	}

	s.scene = sc
	s.gat = l.GAT()
	s.pathFinder = world.NewPathFinder(s.gat)
	s.MapLoaded = true
	s.SceneReady = true
	s.lastLoadMs = l.TotalMs()
	s.lastLoadPhase = l.TimingSummary()

	logger.Info("map loaded",
		zap.String("map", l.Name),
		zap.Float64("ms", s.lastLoadMs),
		zap.Int("models", s.scene.LoadedModels()),
		zap.Int("terrainGroups", s.scene.TerrainGroups()),
		zap.Float32("width", s.scene.MapWidth),
		zap.Float32("height", s.scene.MapHeight),
		zap.Bool("hasGAT", s.gat != nil),
		zap.String("phases", s.lastLoadPhase))
	trace.Emit(trace.Map, "loaded",
		zap.String("map", l.Name),
		zap.Float64("ms", s.lastLoadMs),
		zap.Int("models", s.scene.LoadedModels()),
		zap.String("phases", s.lastLoadPhase))

	// And its own noises, which are the world file's rather than the music's.
	s.loadAmbientSounds()

	// The map is up, so its background music replaces whatever was playing
	// and repeats until the player leaves for another map.
	s.manager.PlayLocationBGM(s.MapName)

	s.applyMapRules()
	trace.Emit(trace.Map, "water",
		zap.String("map", l.Name), zap.Int("cells", s.scene.WaterCells()))

	s.placePlayer(at)
	s.StatusMsg = fmt.Sprintf("Entered %s", s.MapName)

	s.sendLoadingComplete()
	trace.Emit(trace.Map, "ready",
		zap.String("map", l.Name),
		zap.String("origin", s.mapLoadOrigin),
		zap.Float64("loadMs", s.lastLoadMs))

	if s.OnMapChanged != nil {
		s.OnMapChanged(s.MapName)
	}
}

// applyMapRules gives the camera the map's rules — no orbiting indoors, an
// arc where the presets say so — and turns it to the map's entry angle, as
// the original does on every map change.
func (s *InGameState) applyMapRules() {
	s.mapCamera = s.manager.MapRules().For(s.MapName)
	if s.camera != nil {
		s.camera.SetLimits(s.mapCamera.Limits)
		s.camera.SetYaw(s.mapCamera.YawIn)
	}
	if s.scene != nil {
		// Beyond an indoor map's rooms the original shows black, not sky.
		if s.mapCamera.Indoor {
			s.scene.SetClearColor(scene.IndoorClearColor)
		} else {
			s.scene.SetClearColor(scene.SkyClearColor)
		}
	}
	trace.Emit(trace.Map, "indoor",
		zap.String("map", packets.MapBaseName(s.MapName)),
		zap.Bool("indoor", s.mapCamera.Indoor),
		zap.Bool("yawLocked", s.mapCamera.Limits.YawLocked),
		zap.Bool("zoomLocked", s.mapCamera.Limits.ZoomLocked),
		zap.Bool("arc", s.mapCamera.Limits.Arc),
		zap.Float32("yawIn", s.mapCamera.YawIn))
}

// IsIndoor reports whether the current map is one the original treats as
// indoor.
func (s *InGameState) IsIndoor() bool {
	return s.mapCamera.Indoor
}

// CameraRules is what the current map asks of the camera.
func (s *InGameState) CameraRules() MapCamera {
	return s.mapCamera
}

// WaterCells is how many cells of the current map carry water.
func (s *InGameState) WaterCells() int {
	if s.scene == nil {
		return 0
	}
	return s.scene.WaterCells()
}

// placePlayer puts the character on a cell of the current map, creating it
// the first time.
func (s *InGameState) placePlayer(at spawnPoint) {
	worldX, worldZ := entity.CellToWorld(at.x, at.y)
	worldY := s.terrainHeight(worldX, worldZ)

	if s.player == nil {
		s.player = entity.NewCharacter(worldX, worldY, worldZ)

		// Walk timing comes from the character's `speed` stat (ms per cell).
		s.player.WalkSpeedMs = s.manager.Session.WalkSpeedMs()
		logger.Info("player walk speed",
			zap.Float32("msPerCell", s.player.WalkSpeedMs),
			zap.Bool("hasPathfinder", s.pathFinder != nil))

		// Create entity wrapper for the player
		s.entityManager.SetPlayer(entity.NewEntity(s.config.CharID, entity.TypePlayer))
	} else {
		// A correction, not a step: any walk is abandoned and the drawn
		// position comes along.
		s.player.SetPosition(worldX, worldY, worldZ)
	}

	// Let the character follow the ground as it walks.
	s.player.TerrainHeight = s.terrainHeight

	// The server numbers directions the opposite way round the compass from
	// the sprite sheets, so this needs converting rather than assigning.
	s.player.Direction = entity.DirectionFromServer(at.dir)

	if pe := s.entityManager.Player(); pe != nil {
		pe.Position = math.Vec3{X: worldX, Y: worldY, Z: worldZ}
	}
	s.TileX, s.TileY = at.x, at.y

	logger.Debug("placed player",
		zap.Int("cellX", at.x), zap.Int("cellY", at.y),
		zap.Float32("worldX", worldX),
		zap.Float32("worldY", worldY),
		zap.Float32("worldZ", worldZ))
}

// sendLoadingComplete tells the server the map is up. It answers with
// everything in view, so this must not go out before the handlers exist and
// there is a map to put the units on.
func (s *InGameState) sendLoadingComplete() {
	pkt := &packets.LoadingComplete{PacketID: packets.CZ_NOTIFY_ACTORINIT}
	if err := s.client.Send(pkt.Encode()); err != nil {
		logger.Warn("could not report the map loaded", zap.Error(err))
	}
}

// dropDialog closes a conversation the server has already ended by moving us.
// Nothing is sent: the script is over on its side.
func (s *InGameState) dropDialog() {
	if s.dialog.Phase == 0 {
		return
	}
	trace.Emit(trace.NPC, "dropped", zap.Uint32("npcID", s.dialog.NPCID), zap.String("reason", "map change"))
	s.dialog = NPCDialog{}
}

// cancelWalk forgets where we were going. The server has moved us, so the
// destination, the prediction and the path are all about a place we are no
// longer in.
func (s *InGameState) cancelWalk() {
	s.hasDest = false
	s.hasPrediction = false
	s.chainCellX, s.chainCellY = -1, -1
	if s.player != nil {
		s.player.StopWalking()
		s.player.ClearDestination()
	}
}

// IsLoadingMap reports whether a map is being loaded — when the loading
// screen stands in for the scene.
func (s *InGameState) IsLoadingMap() bool {
	return s.mapLoader != nil
}

// MapReady reports whether the map is up and the character is standing on it.
func (s *InGameState) MapReady() bool {
	return s.mapLoader == nil && s.MapLoaded && s.player != nil
}

// MapLoadProgress is how far the load in progress has got, 0 to 1.
func (s *InGameState) MapLoadProgress() float32 {
	if s.mapLoader == nil {
		return 1
	}
	return s.mapLoader.Progress()
}

// MapLoadPhase names the phase in progress, for the overlay.
func (s *InGameState) MapLoadPhase() string {
	if s.mapLoader == nil {
		return ""
	}
	return s.mapLoader.Phase().String()
}

// MapLoadImage is which loading screen the load in progress shows, 1-based,
// or zero when nothing is loading.
func (s *InGameState) MapLoadImage() int {
	if s.mapLoader == nil {
		return 0
	}
	return s.mapLoader.ImageIndex
}

// LastMapLoad reports how long the last map took and where the time went.
func (s *InGameState) LastMapLoad() (ms float64, phases string) {
	return s.lastLoadMs, s.lastLoadPhase
}

// Exit is called when leaving this state.
func (s *InGameState) Exit() error {
	s.SaveUIState()

	if s.mapLoader != nil {
		s.abortMapLoad()
	}
	if s.playerRender != nil {
		s.playerRender.Destroy()
		s.playerRender = nil
	}
	if s.portals != nil {
		s.portals.Destroy()
		s.portals = nil
	}
	if s.castAura != nil {
		s.castAura.Destroy()
		s.castAura = nil
	}

	if s.marker != nil {
		s.marker.Destroy()
		s.marker = nil
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

	// Bring the map in, a phase or a few models per frame. Nothing else
	// moves until it is up: the units went with the old map, and the
	// character is put down when this finishes.
	if s.mapLoader != nil {
		stepStart := time.Now()
		done := s.mapLoader.Step()
		if trace.On(trace.Map) {
			trace.Emit(trace.Map, "step",
				zap.String("phase", s.mapLoader.Phase().String()),
				zap.Float64("ms", msSinceStart(stepStart)),
				zap.Float32("progress", s.mapLoader.Progress()))
		}
		if done {
			s.finishMapLoad()
		}
		return nil
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

		s.updatePendingPickup(deltaMs, walking)
		s.updateCombat(deltaMs, walking)
		s.advancePendingBlows(deltaMs)
		s.advanceCast(deltaMs)
		s.advanceSkillLabels(deltaMs)
		s.advanceCastAuras(deltaMs)
		s.advancePendingSkill(deltaMs)
		s.advanceBursts(deltaMs)
		s.advanceDelayedEffects(deltaMs)
		s.advanceUnitSounds()
		s.advanceAmbientSounds(deltaMs)
		s.advanceSpriteEffects(deltaMs)
		s.updateDamageNumbers(deltaMs)
		s.updateEffects(deltaMs)
		s.updateCelebrations(deltaMs)

		// Advance the sprite animation. Frame counts come from the loaded
		// sheet; with no sprites this parks on frame 0 harmlessly.
		// The armed stance is worn while there is something to stand ready
		// against, and dropped the moment the fight is over. Not while
		// seated: standing ready is standing.
		s.player.Ready = s.targetID != 0 && !s.player.Sitting

		idleFrames, walkFrames, onceFrames, standbyFrames, sitFrames := 0, 0, 0, 0, 0
		if s.playerRender != nil {
			idleFrames = s.playerRender.FrameCount(entity.ActionIdle, s.player.Direction)
			walkFrames = s.playerRender.FrameCount(entity.ActionWalk, s.player.Direction)

			if playing := s.player.PlayingAction(); playing >= 0 {
				onceFrames = s.playerRender.FrameCount(playing, s.player.Direction)

				// The sprite's own rate, the way a unit's is taken. The pick-up
				// was running at a rate picked by hand and was first too slow
				// and then too fast; the ACT has said all along.
				s.player.AnimIntervalMs[playing] = s.playerRender.FrameInterval(playing)
			}
			if s.player.Ready {
				standbyFrames = s.playerRender.FrameCount(entity.ActionStandby, s.player.Direction)
			}
			if s.player.Sitting {
				sitFrames = s.playerRender.FrameCount(entity.ActionSit, s.player.Direction)
			}
		}
		s.player.AdvanceAnimation(deltaMs, idleFrames, walkFrames, onceFrames, standbyFrames, sitFrames)

		// Update cell position
		s.TileX, s.TileY = s.player.CurrentCell()

		s.traceGround()
		s.traceHeightGrid()
	}

	// The click flourish runs down on its own; nothing else clears it.
	if s.markerPulse > 0 {
		s.markerPulse -= deltaMs
		if s.markerPulse < 0 {
			s.markerPulse = 0
		}
	}

	// Map models with animation — windmills, clock towers — are rebuilt here.
	if s.scene != nil && s.MapLoaded {
		s.scene.AdvanceModels(deltaMs)
	}

	// Update all entities
	s.entityManager.Update(dt)
	updateUnits(s.entityManager, deltaMs, s.unitAnim)
	s.effectTimeMs += deltaMs

	return nil
}

// unitAnim reports a unit's animation length and frame rate, so it loops at
// the length of its own sprite and runs at the rate its own ACT specifies
// rather than the player's fixed one. Zero until the sheet for that appearance
// has been baked, which parks it on frame 0.
func (s *InGameState) unitAnim(e *entity.Entity, action, direction int) (int, float32) {
	// A warp is an effect, not a sheet: asking would bake the sprite the
	// table names for class 45, which nobody wants to see.
	if s.playerRender == nil || e.Type == entity.TypeWarp || !unitIsDrawable(e) {
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

		// The screen's own axes, worked out once from where the camera is and
		// what it is looking at. Every sprite in the frame is drawn on them,
		// which is what keeps them all upright and square to the view — a
		// quad turned toward the camera one sprite at a time tips over for
		// anything near it, and RO never leans a sprite at all.
		//
		// The position is asked for rather than read off the camera's cached
		// PosX/PosY/PosZ. Those are filled in by whoever last called Position
		// and are not guaranteed to be this frame's or this target's: taken
		// as the camera's whereabouts they gave a basis that was not the
		// screen's at all, and the sprites rose and fell as the camera turned
		// about a character standing still.
		eye := s.camera.Position(x, y, z)

		right, up := character.BillboardVectors(
			eye.X, eye.Y, eye.Z,
			x, y+camera.LookTargetLift, z)

		s.playerRender.Render(viewProj, s.player, s.camera.PosX, s.camera.PosZ, right, up)
		s.renderUnits(viewProj, right, up)
	})
	return nil
}

// renderUnits draws the other units on the map, sharing the player's billboard
// renderer and its sheet cache.
func (s *InGameState) renderUnits(viewProj math.Mat4, right, up [3]float32) {
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
		if e.Type == entity.TypeWarp {
			// The portal, not a sprite: no name, no shadow, no sheet.
			if s.portals != nil {
				s.portals.Render(viewProj, e.Body.RenderX, e.Body.RenderY, e.Body.RenderZ, s.effectTimeMs, e.Alpha())
			}
			continue
		}
		s.playerRender.RenderUnit(viewProj, e.Body, s.camera.PosX, s.camera.PosZ,
			right, up, load, unitSpec(e), e.Alpha())
	}

	s.drawGroundMarker(viewProj)
	s.drawCastAuras(viewProj)
	s.traceUnitStats(tracked, drawn)
}

// SetHoverCell records the cell the cursor is over, or that it is over nothing.
// The marker follows this rather than the pointer, so it snaps cell to cell as
// the original does.
func (s *InGameState) SetHoverCell(cellX, cellY int, ok bool) {
	if ok && (cellX != s.hoverCellX || cellY != s.hoverCellY) {
		trace.Emit(trace.HUD, "target", zap.Int("x", cellX), zap.Int("y", cellY))
	}

	s.hoverCellX, s.hoverCellY, s.hoverValid = cellX, cellY, ok
}

// PulseMarker starts the flourish that answers a click.
func (s *InGameState) PulseMarker() {
	s.markerPulse = scene.MarkerPulseMs
}

// drawGroundMarker puts the cell highlight under the cursor.
func (s *InGameState) drawGroundMarker(viewProj math.Mat4) {
	if s.marker == nil || !s.hoverValid {
		return
	}

	worldX, worldZ := entity.CellToWorld(s.hoverCellX, s.hoverCellY)
	worldY := s.terrainHeight(worldX, worldZ)

	// Progress runs 0 at the click and 1 when the flourish is spent, which is
	// also the size a marker that is merely following the cursor draws at.
	progress := float32(1)
	if s.markerPulse > 0 {
		progress = 1 - s.markerPulse/scene.MarkerPulseMs
	}

	// A skill waiting for a cell holds the marker at the top of its swell, so
	// the ring on the ground reads as armed rather than as the ordinary
	// where-you-would-walk mark. Not for a skill waiting for somebody: there
	// the cursor is what says so, and a swollen ring would point at ground
	// the skill is not going to.
	if s.placingSkill != 0 && !s.placingAtUnit {
		progress = 0
	}

	s.marker.Render(viewProj, worldX, worldY, worldZ, entity.CellSize, progress, 1)

	// Reported once a second while a marker is on screen. The cell alone does
	// not say whether it is being drawn anywhere you can see — this does, and
	// is what separates "no marker" from "marker behind the camera".
	if trace.On(trace.HUD) && time.Since(s.markerTraceAt) >= time.Second {
		s.markerTraceAt = time.Now()
		trace.Emit(trace.HUD, "marker",
			zap.Int("cellX", s.hoverCellX), zap.Int("cellY", s.hoverCellY),
			zap.Float32("worldX", worldX), zap.Float32("worldY", worldY),
			zap.Float32("worldZ", worldZ),
			zap.Float32("scale", scene.MarkerScale(progress)))
	}
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
	err := config.UpdateUIState(func(state *config.UIState) {
		state.CameraZoom = s.camera.Distance
	})
	if err != nil {
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
// handleSkillList takes the character's skills, which the server sends once
// on entering the map.
func (s *InGameState) handleSkillList(data []byte) error {
	list := packets.DecodeSkillList(data)

	s.skills = list

	trace.Emit(trace.HUD, "skill-list", zap.Int("count", len(list)))

	return nil
}

// handleInventoryNormal takes the stackable half of the inventory.
func (s *InGameState) handleInventoryNormal(data []byte) error {
	return s.takeInventory(data, packets.NormalItemLen, "normal", packets.DecodeInventoryNormal)
}

// handleUseItemAck applies the server's answer to using an item.
//
// Two things about this packet decide the shape of the code. The count it
// carries is what is *left*, not what was spent, so it is assigned rather than
// subtracted — which also makes a missed ack self-correcting. And rAthena
// sends the success case to everyone nearby, not just to the player who used
// the item, so an ack for someone else's potion arrives here too and must not
// be allowed to touch our inventory.
//
// Nothing is changed on a refusal. The server keeps the item in that case, and
// showing it spent would be a lie the next inventory list would quietly undo.
func (s *InGameState) handleUseItemAck(data []byte) error {
	ack, ok := packets.DecodeUseItemAck(data)
	if !ok {
		logger.Warn("short use-item ack", zap.Int("len", len(data)))

		return nil
	}

	if ack.AccountID != s.selfAID() {
		return nil
	}

	if !ack.OK {
		trace.Emit(trace.HUD, "use-item-refused",
			zap.Int("index", ack.Index), zap.Uint32("item", ack.ItemID))

		return nil
	}

	for i := range s.inventory {
		if s.inventory[i].Index != ack.Index {
			continue
		}

		if ack.Amount <= 0 {
			s.inventory = append(s.inventory[:i], s.inventory[i+1:]...)
		} else {
			s.inventory[i].Count = ack.Amount
		}

		break
	}

	trace.Emit(trace.HUD, "use-item-ack",
		zap.Int("index", ack.Index), zap.Int("left", ack.Amount))

	return nil
}

// handleInventoryEquip takes the worn half.
func (s *InGameState) handleInventoryEquip(data []byte) error {
	return s.takeInventory(data, packets.EquipItemLen, "equip", packets.DecodeInventoryEquip)
}

// mergeInventory folds a delivered list into what we already hold, keyed by
// slot, and reports how much of it was new.
//
// Keyed rather than appended because the server delivers the inventory more
// than once — on a map change among other things — and appending gave a
// second row for every item each time you walked through a warp. The slot is
// the server's own name for an item and cannot collide, so a repeat delivery
// lands back on the rows it came from.
//
// Rows the list does not mention are left alone: the two lists arrive
// separately and each covers half the bag, so dropping what is missing from
// one would empty the other.
func (s *InGameState) mergeInventory(items []packets.InventoryItem) (added, replaced int) {
	for _, item := range items {
		existing := -1
		for i := range s.inventory {
			if s.inventory[i].Index == item.Index {
				existing = i

				break
			}
		}

		if existing >= 0 {
			s.inventory[existing] = item
			replaced++

			continue
		}

		s.inventory = append(s.inventory, item)
		added++
	}

	return added, replaced
}

// takeInventory folds one of the two lists into the inventory, and complains
// loudly if the entries did not divide evenly.
//
// That check is the important part. The entry layout changed several times
// across packet versions and the sizes here are read off the server's structs
// with our version's guards resolved by hand. If the arithmetic is wrong the
// remainder says so, which is far easier to act on than a window full of
// nonsense items.
func (s *InGameState) takeInventory(
	data []byte, entryLen int, which string, decode func([]byte) []packets.InventoryItem,
) error {
	if left := packets.ItemListRemainder(data, entryLen); left != 0 {
		logger.Warn("inventory list does not divide into whole entries",
			zap.String("list", which),
			zap.Int("entryLen", entryLen),
			zap.Int("remainder", left),
			zap.Int("bytes", len(data)))

		return nil
	}

	items := decode(data)
	added, replaced := s.mergeInventory(items)

	trace.Emit(trace.HUD, "inventory",
		zap.String("list", which),
		zap.Int("added", added),
		zap.Int("replaced", replaced),
		zap.Int("total", len(s.inventory)))

	return nil
}

// UseItem asks to use the item in an inventory slot.
//
// Nothing is changed locally. The server answers with the item's effect and a
// new count, and acting before it does would show a potion drunk that the
// server refused.
func (s *InGameState) UseItem(index int) error {
	trace.Emit(trace.HUD, "use-item", zap.Int("index", index))

	return s.client.Send(packets.EncodeUseItem(index))
}

// Inventory returns what the character is carrying, for the interface.
func (s *InGameState) Inventory() []packets.InventoryItem {
	return s.inventory
}

// Skills returns the character's skills for the interface, oldest order kept:
// the server sends them in the order the window is meant to list them.
func (s *InGameState) Skills() []packets.Skill {
	return s.skills
}

// handleStatus takes the whole status window: the six primary stats, what
// raising each costs, and the points left to spend.
func (s *InGameState) handleStatus(data []byte) error {
	status := packets.DecodeStatus(data)
	if status == nil {
		logger.Warn("malformed status window packet", zap.Int("bytes", len(data)))

		return nil
	}

	s.stats.ApplyStatus(status)

	trace.Emit(trace.Status, "window",
		zap.Int("points", status.StatusPoints),
		zap.Ints("values", status.Values[:]))

	return nil
}

// handleCoupleStatus takes one primary stat and the bonus on it, which is the
// only packet that carries the bonus at all.
func (s *InGameState) handleCoupleStatus(data []byte) error {
	couple := packets.DecodeCoupleStatus(data)
	if couple == nil {
		logger.Warn("malformed couple status packet", zap.Int("bytes", len(data)))

		return nil
	}

	if !s.stats.ApplyCoupleStatus(couple) {
		// The server sends these for derived numbers too — attack, defense
		// and the rest — which the window does not show yet.
		return nil
	}

	trace.Emit(trace.Status, "couple",
		zap.Uint16("varID", couple.VarID),
		zap.Int("base", couple.Base),
		zap.Int("bonus", couple.Bonus))

	return nil
}

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
	s.client.RegisterHandler(packets.ZC_STATE_CHANGE, s.handleStateChange)
	s.client.RegisterHandler(packets.ZC_DISPEL, s.handleCastCancelled)
	s.client.RegisterHandler(packets.ZC_SKILL_ENTRY, s.handleSkillUnit)
	s.client.RegisterHandler(packets.ZC_SKILL_DISAPPEAR, s.handleSkillUnitGone)
	s.client.RegisterHandler(packets.ZC_NPCACK_MAPMOVE, s.handleMapChange)
	s.client.RegisterHandler(packets.ZC_NPCACK_SERVERMOVE, s.handleServerMove)
	s.client.RegisterHandler(packets.ZC_NOTIFY_PLAYERMOVE, s.handlePlayerMove)
	s.client.RegisterHandler(packets.ZC_NOTIFY_TIME, s.handleServerTick)
	s.client.RegisterHandler(packets.ZC_NOTIFY_CHAT, s.handleChat)
	s.client.RegisterHandler(packets.ZC_NOTIFY_PLAYERCHAT, s.handlePlayerChat)
	s.client.RegisterHandler(packets.ZC_BROADCAST, s.handleBroadcast)
	s.client.RegisterHandler(packets.ZC_NPC_CHAT, s.handleNPCChat)
	s.client.RegisterHandler(packets.ZC_USER_COUNT, s.handleUserCount)
	s.client.RegisterHandler(packets.ZC_WHISPER, s.handleWhisper)
	s.client.RegisterHandler(packets.ZC_ACK_WHISPER, s.handleWhisperAck)
	s.client.RegisterHandler(packets.ZC_RESTART_ACK, s.handleRestartAck)
	s.client.RegisterHandler(packets.ZC_ACK_REQ_DISCONNECT, s.handleDisconnectAck)
	s.client.RegisterHandler(packets.ZC_PAR_CHANGE, s.handleStatusChange)
	s.client.RegisterHandler(packets.ZC_STATUS, s.handleStatus)
	s.client.RegisterHandler(packets.ZC_SKILLINFO_LIST, s.handleSkillList)
	s.client.RegisterHandler(packets.ZC_ACK_TOUSESKILL, s.handleSkillFail)
	s.client.RegisterHandler(packets.ZC_USESKILL_ACK, s.handleSkillCast)
	s.client.RegisterHandler(packets.ZC_USE_SKILL, s.handleSkillUse)
	s.client.RegisterHandler(packets.ZC_NOTIFY_SKILL, s.handleSkillDamage)
	s.client.RegisterHandler(packets.ZC_NOTIFY_GROUNDSKILL, s.handleGroundSkill)
	s.client.RegisterHandler(packets.ZC_INVENTORY_ITEMLIST_NORMAL, s.handleInventoryNormal)
	s.client.RegisterHandler(packets.ZC_INVENTORY_ITEMLIST_EQUIP, s.handleInventoryEquip)
	s.client.RegisterHandler(packets.ZC_USE_ITEM_ACK, s.handleUseItemAck)
	s.client.RegisterHandler(packets.ZC_REQ_WEAR_EQUIP_ACK, s.handleEquipAck)
	s.client.RegisterHandler(packets.ZC_REQ_TAKEOFF_EQUIP_ACK, s.handleUnequipAck)
	s.client.RegisterHandler(packets.ZC_CONFIG_NOTIFY, s.handleConfigNotify)
	s.client.RegisterHandler(packets.ZC_SPRITE_CHANGE, s.handleSpriteChange)
	s.client.RegisterHandler(packets.ZC_ITEM_ENTRY, s.handleGroundItemEntry)
	s.client.RegisterHandler(packets.ZC_ITEM_FALL_ENTRY, s.handleGroundItemFall)
	s.client.RegisterHandler(packets.ZC_ITEM_DISAPPEAR, s.handleGroundItemGone)
	s.client.RegisterHandler(packets.ZC_ITEM_PICKUP_ACK, s.handlePickupAck)
	s.client.RegisterHandler(packets.ZC_ITEM_THROW_ACK, s.handleDropAck)
	s.client.RegisterHandler(packets.ZC_NOTIFY_ACT, s.handleDamage)
	s.client.RegisterHandler(packets.ZC_MONSTER_HP_INFO, s.handleMonsterHP)
	s.client.RegisterHandler(packets.ZC_ATTACK_RANGE, s.handleAttackRange)
	s.client.RegisterHandler(packets.ZC_NOTIFY_EFFECT, s.handleLevelUpEffect)
	s.client.RegisterHandler(packets.ZC_COUPLESTATUS, s.handleCoupleStatus)
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
	if s.player == nil || s.mapLoader != nil {
		return nil
	}

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
		// only note it — and, when it was death, lie down.
		trace.Emit(trace.Net, "vanish-self", zap.Uint8("reason", reason))

		if reason == packets.VanishDied && s.player != nil {
			s.player.Die()
			s.forgetAttack()
			s.forgetPendingPickup()
			s.forgetPendingBlows()
		}

		return nil
	}

	// A death that answers a blow still on its way waits for it. The server
	// decides the moment it decides, which is while the sword is mid-swing,
	// and a monster that falls over then died before it was hit.
	//
	// Only death waits. Every other reason is a unit leaving our view, which
	// has nothing to do with any blow and nothing to watch.
	if reason == packets.VanishDied && s.killDeferred(aid) {
		trace.Emit(trace.Net, "vanish-held",
			zap.Uint32("aid", aid), zap.Uint8("reason", reason))

		return nil
	}

	if reason == packets.VanishDied {
		s.killUnit(aid)
	}

	removeUnit(s.entityManager, aid)
	trace.Emit(trace.Net, "vanish",
		zap.Uint32("aid", aid),
		zap.Uint8("reason", reason),
		zap.Int("units", s.entityManager.Count()))
	return nil
}

// killUnit lays a unit down where it stands.
//
// It is taken off the map when the fade finishes rather than blinking out
// mid-blow, which is what removeUnit begins.
func (s *InGameState) killUnit(aid uint32) {
	if s.entityManager == nil {
		return
	}

	if e := s.entityManager.Get(aid); e != nil && e.Body != nil {
		e.IsDead = true
		e.Body.Die()
	}

	removeUnit(s.entityManager, aid)
}

// warnIfUndrawable says so when a unit arrives that nothing can draw.
//
// A monster whose id is not in the sprite table is not merely undrawn: it is
// invisible and unclickable, and it still fights. Somebody summoning one has
// no way to tell that from a bug in the renderer, so the id is named — once
// each, since the server repeats a unit's report whenever it comes back into
// view and a warning per sighting would bury the map.
func (s *InGameState) warnIfUndrawable(e *entity.Entity) {
	if e.Type != entity.TypeMonster && e.Type != entity.TypeNPC {
		return
	}

	if _, known := charsprite.SpriteName(e.Job); known {
		return
	}

	if s.unknownJobs == nil {
		s.unknownJobs = map[int]bool{}
	}
	if s.unknownJobs[e.Job] {
		return
	}
	s.unknownJobs[e.Job] = true

	logger.Warn("no sprite for this unit, so it is invisible and cannot be clicked",
		zap.Int("job", e.Job), zap.String("name", e.Name),
		zap.String("fix", "regenerate spritenames.go from the client's npcidentity.lub"))
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

	e := upsertUnit(s.entityManager, u, s.unitPath, s.terrainHeight)
	if e == nil {
		return nil
	}

	s.warnIfUndrawable(e)

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

// handleNPCChat handles a line that carries its own color.
//
// A handful of @ commands answer this way rather than on 0x008E — @cash,
// @points, @request and @auction — and the color is the server's choice, so
// it overrides the one the kind would give.
func (s *InGameState) handleNPCChat(data []byte) error {
	return s.addChat(packets.DecodeNPCChat(data))
}

// handleWhisper handles a private message.
func (s *InGameState) handleWhisper(data []byte) error {
	return s.addChat(packets.DecodeWhisper(data))
}

// handleUserCount prints the answer to /who.
//
// The count is the server's to know, so unlike the other / commands this one
// answers a frame or two later, through here.
func (s *InGameState) handleUserCount(data []byte) error {
	count, ok := packets.DecodeUserCount(data)
	if !ok {
		logger.Warn("could not read the player count", zap.Int("bytes", len(data)))

		return nil
	}

	trace.Emit(trace.Cmd, "who-reply", zap.Int("count", count))

	people := "players are"
	if count == 1 {
		people = "player is"
	}
	s.chat.AddLocal(ChatNotice, fmt.Sprintf("%d %s online.", count, people))

	return nil
}

// pendingWhisper is a sent private message held until the server says what
// became of it.
type pendingWhisper struct {
	target string
	text   string
}

// handleWhisperAck reports what became of the whisper we sent.
//
// The line you send is displayed here rather than when you send it: the server
// echoes public chat back but not private messages, so this acknowledgement is
// the only confirmation that anyone received it.
func (s *InGameState) handleWhisperAck(data []byte) error {
	result, ok := packets.DecodeWhisperAck(data)
	if !ok {
		return nil
	}

	pending := s.pendingWhisper
	s.pendingWhisper = pendingWhisper{}

	// Nothing outstanding means the ack is not ours to display — a stale one
	// after a relog, say. Better silent than attributed to the wrong name.
	if pending.target == "" {
		return nil
	}

	trace.Emit(trace.HUD, "whisper-ack",
		zap.Uint8("result", result), zap.String("target", pending.target))

	if failure := packets.WhisperFailure(result, pending.target); failure != "" {
		return s.addChat(&packets.ChatMessage{
			Kind: packets.ChatWhisper,
			Text: failure,
		})
	}

	return s.addChat(&packets.ChatMessage{
		Kind:    packets.ChatWhisper,
		Speaker: "To " + pending.target,
		Text:    pending.text,
	})
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

// SendChat says something in public chat.
//
// The server checks that the line begins with our own character's name, so the
// name comes from the session rather than from anything the caller passes —
// getting it wrong forces a relog rather than showing an error.
func (s *InGameState) SendChat(message string) error {
	name := ""
	if s.manager != nil && s.manager.Session.Char != nil {
		name = s.manager.Session.Char.GetName()
	}

	pkt := packets.EncodeChat(name, message)
	if pkt == nil {
		logger.Warn("not sending an unsendable chat line",
			zap.String("name", name), zap.Int("bytes", len(message)))

		return nil
	}

	trace.Emit(trace.HUD, "chat-send",
		zap.String("name", name), zap.Int("bytes", len(message)))

	return s.client.Send(pkt)
}

// SendWhisper sends a private message to target.
//
// Nothing is added to the scrollback here. The line is held until the server
// acknowledges it, because the acknowledgement is what says whether the target
// exists — displaying it on send would show a message to a name that is not
// online as though it had arrived.
func (s *InGameState) SendWhisper(target, message string) error {
	pkt := packets.EncodeWhisper(target, message)
	if pkt == nil {
		logger.Warn("not sending an unsendable whisper",
			zap.String("target", target), zap.Int("bytes", len(message)))

		return nil
	}

	trace.Emit(trace.HUD, "whisper-send",
		zap.String("target", target), zap.Int("bytes", len(message)))

	if err := s.client.Send(pkt); err != nil {
		return err
	}

	s.pendingWhisper = pendingWhisper{target: target, text: message}

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

// traceGround reports what the ground under the player is doing, for telling
// a wrong height apart from a wrong sprite.
//
// Only while walking and only every so often: standing still it says the same
// thing forever, and every frame of a walk would bury everything else in the
// log.
func (s *InGameState) traceGround() {
	if !trace.On(trace.Move) || s.scene == nil || !s.MapLoaded || s.player == nil {
		return
	}

	if !s.player.IsMoving {
		s.groundTraceMs = 0

		return
	}

	s.groundTraceMs++
	if s.groundTraceMs%groundTraceEvery != 0 {
		return
	}

	tileX, tileZ, u, v, corners := s.scene.TerrainProbe(s.player.RenderX, s.player.RenderZ)

	trace.Emit(trace.Move, "ground",
		zap.Float32("renderY", s.player.RenderY),
		zap.Float32("groundY", s.scene.GetTerrainHeight(s.player.RenderX, s.player.RenderZ)),
		zap.Float32("gatY", s.scene.GatHeight(s.player.RenderX, s.player.RenderZ)),
		zap.Int("tileX", tileX), zap.Int("tileZ", tileZ),
		zap.Float32("u", u), zap.Float32("v", v),
		zap.Float32("sw", corners[0]), zap.Float32("se", corners[1]),
		zap.Float32("nw", corners[2]), zap.Float32("ne", corners[3]))
}

// groundTraceEvery is how many frames apart the ground trace speaks, so a
// walk leaves a readable trail rather than a frame-by-frame flood.
const groundTraceEvery = 10

// traceHeightGrid prints the ground around the player as a grid, once.
//
// Two grids, side by side: what the mesh says the ground is and what the
// collision map says you walk at. They are not the same map — a town's raised
// plaza can be built out of models standing on flat ground — and reading the
// wrong one puts a character inside the steps rather than on them. Seeing them
// together is the only way to tell which is which.
func (s *InGameState) traceHeightGrid() {
	if s.gridTraced || !trace.On(trace.Map) || s.scene == nil || !s.MapLoaded || s.player == nil {
		return
	}

	s.gridTraced = true

	cellX, cellY := s.player.CurrentCell()

	var mesh, walk strings.Builder

	for dy := heightGridSpan; dy >= -heightGridSpan; dy-- {
		for dx := -heightGridSpan; dx <= heightGridSpan; dx++ {
			x, z := entity.CellToWorld(cellX+dx, cellY+dy)

			fmt.Fprintf(&mesh, "%5.0f", s.scene.GetTerrainHeight(x, z))
			fmt.Fprintf(&walk, "%5.0f", s.scene.GatHeight(x, z))
		}

		mesh.WriteByte('\n')
		walk.WriteByte('\n')
	}

	trace.Emit(trace.Map, "height-grid",
		zap.Int("cellX", cellX), zap.Int("cellY", cellY),
		zap.String("mesh", "\n"+mesh.String()),
		zap.String("walkable", "\n"+walk.String()))
}

// heightGridSpan is how many cells either side of the player the grid covers.
const heightGridSpan = 8

// terrainHeight returns the height a unit stands at, so it follows the ground
// as it walks rather than sinking through it. Returns zero before the map is
// loaded, which is what a flat map would give.
//
// From the collision map rather than the ground mesh. The two are not the same
// surface and where they differ the collision map is the one to believe: a
// town's steps are built out of map models standing on flat ground, so the
// mesh reports one height for the whole flight while the collision map carries
// the climb. Geffen's ramp is flat in the mesh and rises from -39 to -12 in
// the collision map, which is the difference between walking up the steps and
// walking through them with your head out of the top.
//
// The mesh is the fallback for a map with no collision data, where it is the
// only surface there is.
func (s *InGameState) terrainHeight(worldX, worldZ float32) float32 {
	if s.scene == nil || !s.MapLoaded {
		return 0
	}

	if s.scene.HasGAT() {
		return s.scene.GatHeight(worldX, worldZ)
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

// warpFacing is the server's numbering for south — facing the viewer — which
// is how the original stands a character that has just arrived through a
// warp; the packet carries no direction of its own.
const warpFacing = 4

// handleMapChange handles ZC_NPCACK_MAPMOVE: the server has moved us.
//
// Three cases, and only the last is a load. rAthena sends one on every login
// for the map being entered (pc_authok), and it arrives while that map is
// loading: noted, not acted on. One naming the map we are already standing
// on is a teleport within it — a staircase, or the server warping us again on
// arrival — which the original handles without reloading. Anything else is a
// warp to another map.
func (s *InGameState) handleMapChange(data []byte) error {
	mc := packets.DecodeMapChange(data)
	if mc == nil {
		return fmt.Errorf("invalid ZC_NPCACK_MAPMOVE: %d bytes", len(data))
	}

	at := spawnPoint{x: mc.X, y: mc.Y, dir: warpFacing}
	same := mc.BaseName() == packets.MapBaseName(s.MapName)

	switch {
	case same && !s.MapLoaded:
		// The map is loading, or about to: this is the login echo.
		s.pendingSpawn = at
		trace.Emit(trace.Map, "change",
			zap.String("from", s.MapName), zap.String("to", mc.MapName),
			zap.Int("x", mc.X), zap.Int("y", mc.Y),
			zap.String("origin", "login"), zap.Bool("same", true),
			zap.String("note", "already loading"))
	case same:
		s.localTeleport(at)
	default:
		s.beginMapLoad(mc.MapName, at, "warp")
	}
	return nil
}

// localTeleport moves us within the current map. Nothing is reloaded: the
// units are dropped, since the server describes the new surroundings when we
// report ready, and the character is put where it was told.
func (s *InGameState) localTeleport(at spawnPoint) {
	s.dropDialog()
	s.cancelWalk()
	s.entityManager.Clear()
	s.forgetPendingBlows()
	s.placePlayer(at)

	trace.Emit(trace.Map, "change",
		zap.String("from", s.MapName), zap.String("to", s.MapName),
		zap.Int("x", at.x), zap.Int("y", at.y),
		zap.String("origin", "teleport"), zap.Bool("same", true))

	s.sendLoadingComplete()
	trace.Emit(trace.Map, "ready", zap.String("map", s.MapName), zap.String("origin", "teleport"))
}

// handleServerMove handles ZC_NPCACK_SERVERMOVE: the destination lives on
// another map server. We run one, so this is reported rather than followed —
// reconnecting to a second server is a feature of its own.
func (s *InGameState) handleServerMove(data []byte) error {
	sm := packets.DecodeServerMove(data)
	if sm == nil {
		return fmt.Errorf("invalid ZC_NPCACK_SERVERMOVE: %d bytes", len(data))
	}

	logger.Warn("map server change is not supported",
		zap.String("map", sm.MapName), zap.Int("x", sm.X), zap.Int("y", sm.Y),
		zap.String("server", sm.Address()), zap.String("domain", sm.Domain))
	trace.Emit(trace.Map, "server-move",
		zap.String("map", sm.MapName), zap.String("server", sm.Address()))
	return nil
}

// Ground picking: how far apart the samples along a click's ray are, and how
// far along it they are taken.
//
// The step is half a cell. The crossing is pinned by bisection afterwards, so
// the step only has to be fine enough not to stride over a feature whole — and
// a step of half a cell cannot skip a wall a cell wide. The reach is a long
// way across a map, because a low camera looking at the horizon runs a long
// way before it meets anything.
const (
	groundPickStep  = float32(2.5)
	groundPickReach = float32(2000)
)

// ScreenToTile maps a screen-space click (in viewport pixels) to the cell it
// landed on, by casting a ray through the most recent view-projection matrix
// the scene rendered with and walking it into the ground.
//
// Returns ok=false if the scene hasn't rendered yet, if the pointer is outside
// the window, or if the ray meets neither the ground nor the plane through it.
func (s *InGameState) ScreenToTile(screenX, screenY, viewportW, viewportH float32) (tileX, tileY int, ok bool) {
	if s.scene == nil || viewportW <= 0 || viewportH <= 0 {
		trace.Emit(trace.Pick, "reject", zap.Bool("hasScene", s.scene != nil),
			zap.Float32("viewportW", viewportW), zap.Float32("viewportH", viewportH))

		return 0, 0, false
	}

	// A pointer outside the window is reported as minus the largest float
	// there is, and a ray built from that is all NaN — which compares false
	// against everything, so every test it meets passes it along and a cell
	// gets picked out of nothing.
	if screenX < 0 || screenY < 0 || screenX > viewportW || screenY > viewportH {
		trace.Emit(trace.Pick, "reject-offscreen",
			zap.Float32("screenX", screenX), zap.Float32("screenY", screenY))

		return 0, 0, false
	}
	invViewProj := s.scene.LastViewProj().Inverse()
	ray := picking.ScreenToRay(screenX, screenY, viewportW, viewportH, invViewProj)
	groundY := float32(0)
	if s.player != nil {
		groundY = s.player.RenderY
	}
	// Walk the real surface. The plane through the player's feet is only
	// right where it happens to touch the ground, and a ray that misses by a
	// little vertically misses by a lot horizontally once the camera is
	// anything but overhead — which is the pointer and the cell it picks
	// drifting apart on every ramp and stair.
	worldX, worldZ, hit := ray.IntersectGround(s.terrainHeight, groundPickStep, groundPickReach)
	onGround := hit

	// The plane is the fallback for a ray that never meets the ground, which
	// is what pointing at the sky does.
	if !hit {
		worldX, worldZ, hit = ray.IntersectPlaneY(groundY)
	}

	if trace.On(trace.Pick) {
		trace.Emit(trace.Pick, "ray",
			zap.Float32("screenX", screenX), zap.Float32("screenY", screenY),
			zap.Float32("viewportW", viewportW), zap.Float32("viewportH", viewportH),
			zap.Float32("originX", ray.Origin[0]), zap.Float32("originY", ray.Origin[1]),
			zap.Float32("originZ", ray.Origin[2]),
			zap.Float32("dirX", ray.Direction[0]), zap.Float32("dirY", ray.Direction[1]),
			zap.Float32("dirZ", ray.Direction[2]),
			zap.Float32("groundY", groundY),
			zap.Bool("onGround", onGround),
			zap.Bool("hit", hit))
	}

	if !hit {
		return 0, 0, false
	}
	cellX, cellY := entity.WorldToCell(worldX, worldZ)
	markerX, markerZ := entity.CellToWorld(cellX, cellY)

	if trace.On(trace.Pick) {
		playerCellX, playerCellY := 0, 0
		if s.player != nil {
			playerCellX, playerCellY = s.player.CurrentCell()
		}
		walkable := s.pathFinder == nil || s.pathFinder.IsWalkable(cellX, cellY)
		// Where the cell the click landed on draws, back in screen pixels.
		// The gap between that and the pointer is the thing to watch: the
		// marker sits on the walkable surface and the pointer is over
		// whatever is drawn there, and the two are not always the same
		// surface.
		markX, markY := s.projectToScreen(
			markerX, s.terrainHeight(markerX, markerZ), markerZ, viewportW, viewportH)

		trace.Emit(trace.Pick, "hit",
			zap.Float32("worldX", worldX), zap.Float32("worldZ", worldZ),
			zap.Int("cellX", cellX), zap.Int("cellY", cellY),
			zap.Int("playerCellX", playerCellX), zap.Int("playerCellY", playerCellY),
			zap.Float32("markerScreenX", markX), zap.Float32("markerScreenY", markY),
			zap.Float32("offX", markX-screenX), zap.Float32("offY", markY-screenY),
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
	// There is no world to click on until the map is up.
	if s.mapLoader != nil || s.player == nil {
		return
	}

	// A skill waiting for a cell takes the click before anything else can:
	// while one is held, clicking means "here" and not "walk there" or
	// "attack that".
	if s.placeHeldSkill(mouseX, mouseY, viewportW, viewportH) {
		return
	}

	// Whatever this click turns out to mean, it replaces any errand still in
	// progress. PickUpItem sets a new one if that is what this click was.
	s.forgetPendingPickup()
	s.forgetPendingSkill()
	s.forgetAttack()

	// A unit under the pointer takes the click. For an NPC, walking there
	// instead would be the wrong thing twice over: the conversation would not
	// start, and the server would refuse a step into the cell the NPC is
	// standing on. For a warp it is the other way round — there is nothing to
	// say to it; you walk into it, and the server does the rest.
	if e := s.PickEntity(mouseX, mouseY, viewportW, viewportH); e != nil {
		if e.Type == entity.TypeWarp {
			cellX, cellY := e.Body.CurrentCell()
			stepX, stepY, reachable := s.WarpApproach(cellX, cellY)
			trace.Emit(trace.Map, "warp-click",
				zap.Uint32("aid", e.ID), zap.String("name", e.Name),
				zap.Int("cellX", cellX), zap.Int("cellY", cellY),
				zap.Int("stepX", stepX), zap.Int("stepY", stepY),
				zap.Bool("reachable", reachable))
			if !reachable {
				logger.Warn("no way to stand in this warp",
					zap.String("name", e.Name),
					zap.Int("cellX", cellX), zap.Int("cellY", cellY))
				return
			}
			if err := s.RequestMove(stepX, stepY); err != nil {
				logger.Warn("walk to warp failed", zap.Error(err))
			}
			return
		}

		if s.isAttackable(e) {
			s.AttackTarget(e)

			return
		}

		if e.Type == entity.TypeItem {
			// The server decides whether we are close enough, and refuses
			// with a message rather than by walking us there. Walking to it
			// ourselves would be a guess at a range only the server knows.
			s.PickUpItem(e)

			return
		}

		trace.Emit(trace.NPC, "click",
			zap.Uint32("npcID", e.ID), zap.String("name", e.Name),
			zap.Float32("screenX", mouseX), zap.Float32("screenY", mouseY))

		s.ContactNPC(e)

		return
	}

	tileX, tileY, ok := s.ScreenToTile(mouseX, mouseY, viewportW, viewportH)
	if !ok {
		trace.Emit(trace.Pick, "miss")
		return
	}

	// The flourish answers the click itself, whether or not the walk is
	// accepted — the marker is feedback that the click landed on a cell, and
	// the server's refusal comes later if at all.
	s.PulseMarker()

	if err := s.RequestMove(tileX, tileY); err != nil {
		logger.Warn("click-to-move RequestMove failed", zap.Error(err))
	}
}

// RequestMove asks the server to walk to a cell, remembering it as the
// player's actual intent so a destination beyond one request's reach can be
// walked in stages.
func (s *InGameState) RequestMove(tileX, tileY int) error {
	// A seated character does not go anywhere. The server would refuse it
	// anyway — unit_can_move is false while sitting — so asking would leave
	// the client walking toward a cell no acknowledgement is ever coming for,
	// which is what made a sitting character slide across the ground.
	//
	// Turning is still worth doing: it is the only thing a click can mean
	// here, and a seated character that ignores the pointer entirely reads as
	// one that has stopped responding.
	if s.Sitting() {
		s.faceCell(tileX, tileY)

		return nil
	}

	// Nor does one in the middle of a cast. unit_can_move is false while the
	// skill timer runs, so the server refuses the walk the same way, and the
	// original gives no way to walk out of a cast: it lands, or something
	// breaks it. Asking anyway left the character sliding along while the bar
	// filled.
	if s.Casting() {
		s.faceCell(tileX, tileY)

		return nil
	}

	s.destCellX, s.destCellY = tileX, tileY
	s.hasDest = true
	s.chainCellX, s.chainCellY = -1, -1

	return s.sendWalkRequest(tileX, tileY)
}

// faceCell turns the character to look at a cell.
func (s *InGameState) faceCell(tileX, tileY int) {
	if s.player == nil {
		return
	}

	fromX, fromY := s.player.CurrentCell()
	if dir := entity.DirectionFromCellDelta(tileX-fromX, tileY-fromY); dir >= 0 {
		s.player.Direction = dir
	}
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

// barWorthShowing reports whether a unit's health is worth the screen space.
//
// Monsters earn a bar by being the target or by being pointed at. Everything
// else that carries one — other players, once they are modeled — keeps it
// unconditionally, because theirs is not a fight the pointer is choosing.
func (s *InGameState) barWorthShowing(e *entity.Entity) bool {
	if e.Type != entity.TypeMonster {
		return true
	}

	if e.ID == s.targetID {
		return true
	}

	return s.hoverEntity != nil && s.hoverEntity.ID == e.ID
}

// msSinceStart is the elapsed time since t in milliseconds, for traces.
func msSinceStart(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000
}

// EntityBars returns the bars to draw under each unit, already projected into
// viewport pixels.
//
// Projection lives here because the view matrix does. The UI layer gets
// finished positions and needs nothing from the scene.
//
// The player is always included: their own bars are what the request was for,
// and their HP and SP are the only ones the server keeps current.
//
// A monster's bar is shown only while it is the target or under the pointer.
// The server keeps sending a damaged monster's health for as long as it is in
// view, so drawing every one that has ever been hit left bars standing over
// half the map long after the fight — and the bar was asked for as part of
// what "selected" looks like, alongside the name and the mark.
func (s *InGameState) EntityBars(viewportW, viewportH float32) []EntityBar {
	if s.scene == nil || !s.SceneReady {
		return nil
	}

	var bars []EntityBar

	if s.player != nil && s.stats.MaxHP > 0 {
		if x, y := s.projectToScreen(s.player.RenderX, s.player.RenderY, s.player.RenderZ,
			viewportW, viewportH); x >= 0 {
			bars = append(bars, EntityBar{
				ScreenX: x, ScreenY: y,
				Type:  entity.TypePlayer,
				HP:    s.stats.HP,
				MaxHP: s.stats.MaxHP,
				HasSP: s.stats.MaxSP > 0,
				SP:    s.stats.SP,
				MaxSP: s.stats.MaxSP,
				Alpha: 1,
			})
		}
	}

	for _, e := range s.entityManager.All() {
		if e.Body == nil || !e.ShowHP || e.MaxHP <= 0 || e.ID == s.selfAID() {
			continue
		}

		if !s.barWorthShowing(e) {
			continue
		}

		x, y := s.projectToScreen(e.Body.RenderX, e.Body.RenderY, e.Body.RenderZ,
			viewportW, viewportH)
		if x < 0 {
			continue
		}

		bars = append(bars, EntityBar{
			ScreenX: x, ScreenY: y,
			Type:  e.Type,
			HP:    e.HP,
			MaxHP: e.MaxHP,
			// The server never tells us another unit's SP, so they get one bar.
			HasSP: false,
			Alpha: e.Alpha(),
		})
	}

	return bars
}

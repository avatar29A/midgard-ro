package entity

// Cell geometry and walk timing.
//
// RO servers speak in GAT cells, not world units. A GND tile spans `zoom`
// world units (10 on virtually every official map) and is subdivided into
// 2x2 GAT cells, so one server cell is half a tile wide.
const (
	// CellSize is the width of one GAT cell in world units.
	CellSize = 5.0

	// DefaultWalkSpeedMs is rAthena's baseline `speed` stat: the number of
	// milliseconds it takes to cross one straight cell. It is a duration,
	// not a velocity — a *smaller* number means a *faster* character.
	DefaultWalkSpeedMs = 150.0

	// DiagonalCostFactor is how much longer a diagonal step takes than a
	// straight one. rAthena charges the same 1.4x the pathfinder does.
	DiagonalCostFactor = 1.4

	// maxCarryCells is how far the drawn position may sit from a newly
	// received path's start before we give up on catching up smoothly and
	// snap. Beyond a few cells the slide would be more distracting than the
	// jump it avoids.
	maxCarryCells = 5.0

	// CatchUpFraction is how much extra speed is spent closing the gap between
	// where the character is drawn and where the server says they are, as a
	// fraction of walking speed. Low enough to read as walking, high enough to
	// converge long before the drift can accumulate.
	CatchUpFraction = 0.5
)

// cellDeltaToDirection maps a one-cell step to an RO compass direction,
// indexed by (dy+1)*3 + (dx+1). Server y grows north, x grows east.
// -1 marks "no movement", which leaves the current facing alone.
var cellDeltaToDirection = [9]int{
	DirSW, DirS, DirSE, // dy = -1 (south)
	DirW, -1, DirE, // dy =  0
	DirNW, DirN, DirNE, // dy = +1 (north)
}

// serverDirDeltas mirrors rAthena's dirx/diry lookup tables (src/map/unit.cpp)
// indexed by its `enum directions` (src/map/path.hpp):
//
//	DIR_NORTH=0, DIR_NORTHWEST=1, DIR_WEST=2, DIR_SOUTHWEST=3,
//	DIR_SOUTH=4, DIR_SOUTHEAST=5, DIR_EAST=6, DIR_NORTHEAST=7
//
// Note this runs the opposite way around the compass from the sprite indices
// RO art uses (S=0, SW=1, W=2, ...), so the two are never interchangeable —
// assigning a server direction straight to a sprite direction points the
// character the wrong way everywhere except due west and due east.
var serverDirDeltas = [8][2]int{
	{0, 1},   // DIR_NORTH
	{-1, 1},  // DIR_NORTHWEST
	{-1, 0},  // DIR_WEST
	{-1, -1}, // DIR_SOUTHWEST
	{0, -1},  // DIR_SOUTH
	{1, -1},  // DIR_SOUTHEAST
	{1, 0},   // DIR_EAST
	{1, 1},   // DIR_NORTHEAST
}

// DirectionFromServer converts a direction as the server numbers them into the
// sprite direction index. It goes through the same cell-delta table the walk
// code uses, so the two can't drift apart.
func DirectionFromServer(serverDir uint8) int {
	if int(serverDir) >= len(serverDirDeltas) {
		return DirS
	}
	d := serverDirDeltas[serverDir]
	if dir := DirectionFromCellDelta(d[0], d[1]); dir >= 0 {
		return dir
	}
	return DirS
}

// CellDeltaForDirection returns the one-cell step that moves a character in the
// given RO direction. It is the inverse of DirectionFromCellDelta.
func CellDeltaForDirection(dir int) (dx, dy int) {
	for i, d := range cellDeltaToDirection {
		if d == dir {
			return i%3 - 1, i/3 - 1
		}
	}
	return 0, 0
}

// CellToWorld returns the world XZ of a cell's center.
func CellToWorld(cellX, cellY int) (worldX, worldZ float32) {
	return (float32(cellX) + 0.5) * CellSize, (float32(cellY) + 0.5) * CellSize
}

// WorldToCell returns the cell containing a world XZ.
func WorldToCell(worldX, worldZ float32) (cellX, cellY int) {
	return int(worldX / CellSize), int(worldZ / CellSize)
}

// DirectionFromCellDelta returns the RO compass direction for a single-cell
// step, or -1 if the step doesn't move.
func DirectionFromCellDelta(dx, dy int) int {
	if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		dx = sign(dx)
		dy = sign(dy)
	}
	return cellDeltaToDirection[(dy+1)*3+(dx+1)]
}

// CurrentCell returns the cell the character currently occupies.
func (c *Character) CurrentCell() (cellX, cellY int) {
	return WorldToCell(c.WorldX, c.WorldZ)
}

// SetCell places the character on a map cell, at the terrain height there.
//
// This is a correction from the server rather than a step of a walk, so it
// goes through SetPosition: any path in progress is abandoned and the render
// position is brought with it, putting the character where the server says it
// is instead of sliding it there.
func (c *Character) SetCell(cellX, cellY int) {
	x, z := CellToWorld(cellX, cellY)
	c.SetPosition(x, c.groundAt(x, z), z)
}

// IsWalkingPath reports whether a server path is being walked.
func (c *Character) IsWalkingPath() bool {
	return len(c.path) > 0
}

// Path returns the cell path currently being walked (nil when idle).
// The slice is owned by the character; callers must not mutate it.
func (c *Character) Path() [][2]int {
	return c.path
}

// FollowPath starts walking the given cell path, which must begin at the
// character's authoritative start cell (path[0]) and list every cell along
// the way. Timing comes from WalkSpeedMs, so the walk lands on each cell at
// the same moment the server thinks it does.
//
// The world position moves to path[0] at once — the server is authoritative —
// but the drawn position does not teleport there. Whatever gap exists between
// the two is kept as a visual offset and bled off over the following frames at
// CatchUpFraction of walking speed.
//
// Without local prediction we are always a round trip behind, so this gap is
// the normal case rather than an error. Snapping it away teleports the
// character; folding it into the first step instead makes them sprint for one
// cell, since that step then has to cover the gap plus a cell in one cell's
// worth of time. Carrying it does neither. Past maxCarryCells the gap is too
// large to walk off and we snap.
func (c *Character) FollowPath(path [][2]int) {
	if len(path) < 2 {
		c.StopWalking()
		return
	}

	startX, startZ := CellToWorld(path[0][0], path[0][1])
	c.WorldX, c.WorldZ = startX, startZ
	c.WorldY = c.groundAt(startX, startZ)

	c.offsetX = c.RenderX - startX
	c.offsetZ = c.RenderZ - startZ
	if c.offsetX*c.offsetX+c.offsetZ*c.offsetZ > (maxCarryCells*CellSize)*(maxCarryCells*CellSize) {
		c.offsetX, c.offsetZ = 0, 0
	}

	c.path = path
	c.pathIdx = 1
	c.segElapsed = 0
	c.HasDestination = false
	c.IsMoving = true
	c.beginSegment()
}

// walkUnitsPerSecond converts the ms-per-cell walk speed into a velocity.
func (c *Character) walkUnitsPerSecond() float32 {
	ms := c.WalkSpeedMs
	if ms <= 0 {
		ms = DefaultWalkSpeedMs
	}
	return CellSize * 1000.0 / ms
}

// decayOffset shrinks the visual offset toward zero at a bounded speed, so
// catching up reads as walking slightly faster rather than as a jump.
func (c *Character) decayOffset(deltaMs float32) {
	dist := sqrtf32(c.offsetX*c.offsetX + c.offsetZ*c.offsetZ)
	if dist < 0.01 {
		c.offsetX, c.offsetZ = 0, 0
		return
	}

	step := c.walkUnitsPerSecond() * CatchUpFraction * deltaMs / 1000.0
	if step >= dist {
		c.offsetX, c.offsetZ = 0, 0
		return
	}

	scale := (dist - step) / dist
	c.offsetX *= scale
	c.offsetZ *= scale
}

// applyVisualOffset draws the character at the authoritative position plus
// whatever is left of the offset, after bleeding some of it off.
func (c *Character) applyVisualOffset(baseX, baseZ, deltaMs float32) {
	c.decayOffset(deltaMs)
	c.RenderX = baseX + c.offsetX
	c.RenderZ = baseZ + c.offsetZ
	c.RenderY = c.groundAt(c.RenderX, c.RenderZ)
}

// VisualOffset returns how far the drawn position currently sits from the
// authoritative one, in world units. Exposed for diagnostics.
func (c *Character) VisualOffset() float32 {
	return sqrtf32(c.offsetX*c.offsetX + c.offsetZ*c.offsetZ)
}

// StopWalking abandons the current path and returns the character to idle.
//
// It deliberately leaves CurrentAction alone: AdvanceAnimation owns that, and
// holds the walk cycle briefly across the gap between one server-acknowledged
// step and the next so a character walking cell by cell doesn't flicker to
// idle between them.
func (c *Character) StopWalking() {
	c.path = nil
	c.pathIdx = 0
	c.segElapsed = 0
	c.segDuration = 0
	c.IsMoving = false
}

// UpdateWalk advances along the active path by deltaMs. Returns true if the
// character moved. Callers should prefer Update, which dispatches here when
// a path is active.
func (c *Character) UpdateWalk(deltaMs float32) bool {
	if len(c.path) == 0 {
		return false
	}

	c.segElapsed += deltaMs

	// A long frame (or a very fast character) can consume several cells.
	for c.segElapsed >= c.segDuration {
		c.segElapsed -= c.segDuration
		c.arriveAtSegmentEnd()
		c.pathIdx++
		if c.pathIdx >= len(c.path) {
			c.StopWalking()
			c.applyVisualOffset(c.WorldX, c.WorldZ, deltaMs)
			return true
		}
		c.beginSegment()
	}

	t := c.segElapsed / c.segDuration
	c.applyVisualOffset(
		c.segFromX+(c.segToX-c.segFromX)*t,
		c.segFromZ+(c.segToZ-c.segFromZ)*t,
		deltaMs,
	)
	return true
}

// beginSegment sets up the step from path[pathIdx-1] to path[pathIdx].
//
// Segments always run cell center to cell center, at exactly one cell's worth
// of time. Any discrepancy between that and where the character is drawn lives
// in the visual offset, so the walk itself always moves at walking speed.
func (c *Character) beginSegment() {
	from := c.path[c.pathIdx-1]
	to := c.path[c.pathIdx]

	c.segFromX, c.segFromZ = CellToWorld(from[0], from[1])
	c.segToX, c.segToZ = CellToWorld(to[0], to[1])

	dx, dy := to[0]-from[0], to[1]-from[1]
	if dir := DirectionFromCellDelta(dx, dy); dir >= 0 {
		c.Direction = dir
	}

	speed := c.WalkSpeedMs
	if speed <= 0 {
		speed = DefaultWalkSpeedMs
	}
	if dx != 0 && dy != 0 {
		speed *= DiagonalCostFactor
	}
	if speed <= 0 {
		speed = 1 // never let the step loop spin forever
	}
	c.segDuration = speed
}

// arriveAtSegmentEnd plants the character exactly on the cell it just
// reached, so rounding never accumulates across a long path. The drawn
// position is left to applyVisualOffset, which still owes any outstanding
// correction.
func (c *Character) arriveAtSegmentEnd() {
	c.WorldX, c.WorldZ = c.segToX, c.segToZ
	c.WorldY = c.groundAt(c.WorldX, c.WorldZ)
}

// groundAt returns the terrain altitude at a world XZ, or the current
// altitude when no terrain query is wired up.
func (c *Character) groundAt(worldX, worldZ float32) float32 {
	if c.TerrainHeight != nil {
		return c.TerrainHeight(worldX, worldZ)
	}
	return c.WorldY
}

// CellLine returns the cells a straight walk from start to end passes
// through, using RO's step rule: move diagonally while both axes still have
// ground to cover, then straight for the remainder. It is the fallback for
// when A* can't produce a path (missing or disagreeing GAT) — the walk stays
// smooth and correctly timed even if it clips a corner.
func CellLine(startX, startY, endX, endY int) [][2]int {
	path := [][2]int{{startX, startY}}
	x, y := startX, startY
	// Bounded so a bad input can never spin: no legitimate walk request
	// crosses more cells than the largest official map is wide.
	for i := 0; i < 1024 && (x != endX || y != endY); i++ {
		x += sign(endX - x)
		y += sign(endY - y)
		path = append(path, [2]int{x, y})
	}
	return path
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

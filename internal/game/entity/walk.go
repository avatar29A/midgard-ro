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

	// resyncCells is how far the rendered position may drift from a newly
	// received path's start cell before we hard-snap instead of gliding.
	resyncCells = 2.0
)

// cellDeltaToDirection maps a one-cell step to an RO compass direction,
// indexed by (dy+1)*3 + (dx+1). Server y grows north, x grows east.
// -1 marks "no movement", which leaves the current facing alone.
var cellDeltaToDirection = [9]int{
	DirSW, DirS, DirSE, // dy = -1 (south)
	DirW, -1, DirE, // dy =  0
	DirNW, DirN, DirNE, // dy = +1 (north)
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
// The character's world position snaps to path[0] immediately — the server
// is authoritative — but the *rendered* position glides on from wherever it
// already was, so a mid-walk re-path doesn't visibly jump. A drift of more
// than resyncCells is treated as a desync and hard-snapped instead.
func (c *Character) FollowPath(path [][2]int) {
	if len(path) < 2 {
		c.StopWalking()
		return
	}

	startX, startZ := CellToWorld(path[0][0], path[0][1])
	c.WorldX, c.WorldZ = startX, startZ
	c.WorldY = c.groundAt(startX, startZ)

	dx := c.RenderX - startX
	dz := c.RenderZ - startZ
	if dx*dx+dz*dz > (resyncCells*CellSize)*(resyncCells*CellSize) {
		c.RenderX, c.RenderY, c.RenderZ = c.WorldX, c.WorldY, c.WorldZ
	}

	c.path = path
	c.pathIdx = 1
	c.segElapsed = 0
	c.HasDestination = false
	c.IsMoving = true
	c.CurrentAction = ActionWalk
	c.beginSegment(true)
}

// StopWalking abandons the current path and returns the character to idle.
func (c *Character) StopWalking() {
	c.path = nil
	c.pathIdx = 0
	c.segElapsed = 0
	c.segDuration = 0
	c.IsMoving = false
	c.CurrentAction = ActionIdle
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
			return true
		}
		c.beginSegment(false)
	}

	t := c.segElapsed / c.segDuration
	c.RenderX = c.segFromX + (c.segToX-c.segFromX)*t
	c.RenderZ = c.segFromZ + (c.segToZ-c.segFromZ)*t
	c.RenderY = c.groundAt(c.RenderX, c.RenderZ)
	return true
}

// beginSegment sets up the step from path[pathIdx-1] to path[pathIdx].
// On the first segment of a path we start the visual glide from wherever the
// character is already rendered rather than from the cell center, so a
// re-path mid-stride stays smooth.
func (c *Character) beginSegment(first bool) {
	from := c.path[c.pathIdx-1]
	to := c.path[c.pathIdx]

	c.segFromX, c.segFromZ = CellToWorld(from[0], from[1])
	if first {
		c.segFromX, c.segFromZ = c.RenderX, c.RenderZ
	}
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
// reached, so rounding never accumulates across a long path.
func (c *Character) arriveAtSegmentEnd() {
	c.WorldX, c.WorldZ = c.segToX, c.segToZ
	c.WorldY = c.groundAt(c.WorldX, c.WorldZ)
	c.RenderX, c.RenderY, c.RenderZ = c.WorldX, c.WorldY, c.WorldZ
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

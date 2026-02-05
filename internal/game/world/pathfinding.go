// Package world provides game world functionality.
package world

import (
	"container/heap"

	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// PathNode represents a node in the A* pathfinding algorithm.
type PathNode struct {
	X, Y   int     // Tile coordinates
	G      float32 // Cost from start
	H      float32 // Heuristic (estimated cost to goal)
	F      float32 // Total cost (G + H)
	Parent *PathNode
	Index  int // Index in heap
}

// PathHeap implements a priority queue for A* pathfinding.
type PathHeap []*PathNode

func (h PathHeap) Len() int           { return len(h) }
func (h PathHeap) Less(i, j int) bool { return h[i].F < h[j].F }
func (h PathHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}

func (h *PathHeap) Push(x interface{}) {
	n := len(*h)
	node := x.(*PathNode)
	node.Index = n
	*h = append(*h, node)
}

func (h *PathHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.Index = -1
	*h = old[0 : n-1]
	return node
}

// PathFinder handles pathfinding on the game map.
type PathFinder struct {
	gat    *formats.GAT
	width  int
	height int
}

// NewPathFinder creates a new pathfinder.
func NewPathFinder(gat *formats.GAT) *PathFinder {
	if gat == nil {
		return nil
	}
	return &PathFinder{
		gat:    gat,
		width:  int(gat.Width),
		height: int(gat.Height),
	}
}

// FindPath finds a path from start to goal using A* algorithm.
// Returns nil if no path exists.
func (pf *PathFinder) FindPath(startX, startY, goalX, goalY int) [][2]int {
	if pf == nil || pf.gat == nil {
		return nil
	}

	// Check bounds
	if !pf.inBounds(startX, startY) || !pf.inBounds(goalX, goalY) {
		return nil
	}

	// Check if goal is walkable
	if !pf.gat.IsWalkable(goalX, goalY) {
		return nil
	}

	// A* algorithm
	openSet := &PathHeap{}
	heap.Init(openSet)

	closedSet := make(map[int]bool)
	nodeMap := make(map[int]*PathNode)

	startNode := &PathNode{
		X: startX,
		Y: startY,
		G: 0,
		H: pf.heuristic(startX, startY, goalX, goalY),
	}
	startNode.F = startNode.G + startNode.H
	heap.Push(openSet, startNode)
	nodeMap[pf.key(startX, startY)] = startNode

	// Directions: 8-way movement
	// Order matches RO direction indices
	directions := [][2]int{
		{0, 1},   // S
		{-1, 1},  // SW
		{-1, 0},  // W
		{-1, -1}, // NW
		{0, -1},  // N
		{1, -1},  // NE
		{1, 0},   // E
		{1, 1},   // SE
	}

	// Diagonal movement cost (sqrt(2) ~= 1.414)
	diagonalCost := float32(1.414)
	straightCost := float32(1.0)

	maxIterations := pf.width * pf.height // Prevent infinite loops
	iterations := 0

	for openSet.Len() > 0 && iterations < maxIterations {
		iterations++

		// Get node with lowest F score
		current := heap.Pop(openSet).(*PathNode)

		// Check if we reached the goal
		if current.X == goalX && current.Y == goalY {
			return pf.reconstructPath(current)
		}

		closedSet[pf.key(current.X, current.Y)] = true

		// Check all neighbors
		for i, dir := range directions {
			nx, ny := current.X+dir[0], current.Y+dir[1]

			// Skip if out of bounds or not walkable
			if !pf.inBounds(nx, ny) || !pf.gat.IsWalkable(nx, ny) {
				continue
			}

			// Skip if already processed
			if closedSet[pf.key(nx, ny)] {
				continue
			}

			// Calculate movement cost (diagonal or straight)
			var moveCost float32
			if i%2 == 1 { // Diagonal directions (SW, NW, NE, SE)
				moveCost = diagonalCost
				// For diagonal movement, both adjacent cells must be walkable
				if !pf.gat.IsWalkable(current.X+dir[0], current.Y) ||
					!pf.gat.IsWalkable(current.X, current.Y+dir[1]) {
					continue
				}
			} else {
				moveCost = straightCost
			}

			g := current.G + moveCost

			neighbor, exists := nodeMap[pf.key(nx, ny)]
			if !exists {
				// New node
				neighbor = &PathNode{
					X:      nx,
					Y:      ny,
					G:      g,
					H:      pf.heuristic(nx, ny, goalX, goalY),
					Parent: current,
				}
				neighbor.F = neighbor.G + neighbor.H
				nodeMap[pf.key(nx, ny)] = neighbor
				heap.Push(openSet, neighbor)
			} else if g < neighbor.G {
				// Found better path
				neighbor.G = g
				neighbor.F = neighbor.G + neighbor.H
				neighbor.Parent = current
				heap.Fix(openSet, neighbor.Index)
			}
		}
	}

	// No path found
	return nil
}

// IsWalkable checks if a tile is walkable.
func (pf *PathFinder) IsWalkable(x, y int) bool {
	if pf == nil || pf.gat == nil {
		return false
	}
	if !pf.inBounds(x, y) {
		return false
	}
	return pf.gat.IsWalkable(x, y)
}

// heuristic calculates the estimated distance using octile distance.
func (pf *PathFinder) heuristic(x1, y1, x2, y2 int) float32 {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	// Octile distance: min(dx,dy)*sqrt(2) + |dx-dy|
	if dx < dy {
		return float32(dx)*1.414 + float32(dy-dx)
	}
	return float32(dy)*1.414 + float32(dx-dy)
}

func (pf *PathFinder) inBounds(x, y int) bool {
	return x >= 0 && x < pf.width && y >= 0 && y < pf.height
}

func (pf *PathFinder) key(x, y int) int {
	return y*pf.width + x
}

func (pf *PathFinder) reconstructPath(node *PathNode) [][2]int {
	var path [][2]int
	for node != nil {
		path = append(path, [2]int{node.X, node.Y})
		node = node.Parent
	}
	// Reverse path (it's built from goal to start)
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

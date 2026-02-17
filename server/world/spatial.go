package world

import (
	"sync"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// GridCell represents a cell in the spatial hash grid.
// Using a flat coordinate pair for efficient map lookups.
type GridCell struct {
	X int32
	Z int32
}

// spatialGrid is a hash grid for efficient spatial entity queries.
// It partitions the world into cells to enable O(1) proximity lookups
// instead of scanning all entities.
type spatialGrid struct {
	mu       sync.RWMutex
	cells    map[GridCell][]*EntityHandle
	cellSize int32 // Size of each cell in blocks
}

// newSpatialGrid creates a new spatial grid with the specified cell size.
func newSpatialGrid(cellSize int32) *spatialGrid {
	return &spatialGrid{
		cells:    make(map[GridCell][]*EntityHandle),
		cellSize: cellSize,
	}
}

// cellForPos returns the grid cell for a world position.
func (g *spatialGrid) cellForPos(pos mgl64.Vec3) GridCell {
	return GridCell{
		X: int32(pos[0]) / g.cellSize,
		Z: int32(pos[2]) / g.cellSize,
	}
}

// cellForChunkPos returns the grid cell for a chunk position.
func (g *spatialGrid) cellForChunkPos(pos ChunkPos) GridCell {
	return GridCell{
		X: pos[0],
		Z: pos[1],
	}
}

// Add adds an entity handle to the spatial grid.
func (g *spatialGrid) Add(handle *EntityHandle) {
	pos := handle.data.Pos
	cell := g.cellForPos(pos)

	g.mu.Lock()
	defer g.mu.Unlock()

	g.cells[cell] = append(g.cells[cell], handle)
}

// Remove removes an entity handle from the spatial grid.
func (g *spatialGrid) Remove(handle *EntityHandle) {
	pos := handle.data.Pos
	cell := g.cellForPos(pos)

	g.mu.Lock()
	defer g.mu.Unlock()

	handles := g.cells[cell]
	for i, h := range handles {
		if h == handle {
			g.cells[cell] = append(handles[:i], handles[i+1:]...)
			return
		}
	}
}

// Update moves an entity handle to a new cell if needed.
// Returns true if the cell changed.
func (g *spatialGrid) Update(handle *EntityHandle, oldPos, newPos mgl64.Vec3) bool {
	oldCell := g.cellForPos(oldPos)
	newCell := g.cellForPos(newPos)

	if oldCell == newCell {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Remove from old cell
	oldHandles := g.cells[oldCell]
	for i, h := range oldHandles {
		if h == handle {
			g.cells[oldCell] = append(oldHandles[:i], oldHandles[i+1:]...)
			break
		}
	}

	// Add to new cell
	g.cells[newCell] = append(g.cells[newCell], handle)
	return true
}

// QueryNearby returns all entity handles within a bounding box.
func (g *spatialGrid) QueryNearby(box cube.BBox) []*EntityHandle {
	minCell := g.cellForPos(box.Min())
	maxCell := g.cellForPos(box.Max())

	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*EntityHandle

	for x := minCell.X; x <= maxCell.X; x++ {
		for z := minCell.Z; z <= maxCell.Z; z++ {
			cell := GridCell{X: x, Z: z}
			handles, ok := g.cells[cell]
			if !ok {
				continue
			}
			for _, h := range handles {
				if box.Vec3Within(h.data.Pos) {
					result = append(result, h)
				}
			}
		}
	}

	return result
}

// QueryChunks returns all entity handles in the specified chunk positions.
func (g *spatialGrid) QueryChunks(chunks []ChunkPos) []*EntityHandle {
	g.mu.RLock()
	defer g.mu.RUnlock()

	seen := make(map[*EntityHandle]bool)
	var result []*EntityHandle

	for _, cp := range chunks {
		cell := g.cellForChunkPos(cp)
		handles, ok := g.cells[cell]
		if !ok {
			continue
		}
		for _, h := range handles {
			if !seen[h] {
				seen[h] = true
				result = append(result, h)
			}
		}
	}

	return result
}

// QueryRadius returns all entity handles within radius of a position.
func (g *spatialGrid) QueryRadius(pos mgl64.Vec3, radius int32) []*EntityHandle {
	box := cube.Box(
		pos[0]-float64(radius), pos[1]-float64(radius), pos[2]-float64(radius),
		pos[0]+float64(radius), pos[1]+float64(radius), pos[2]+float64(radius),
	)
	return g.QueryNearby(box)
}

// Clear removes all entities from the grid.
func (g *spatialGrid) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	clear(g.cells)
	g.cells = make(map[GridCell][]*EntityHandle)
}

// Count returns the total number of entities in the grid.
func (g *spatialGrid) Count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var total int
	for _, handles := range g.cells {
		total += len(handles)
	}
	return total
}

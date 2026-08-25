package states

import (
	"encoding/binary"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/game/world"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// openGAT builds a fully walkable map for the pathfinder to work over.
func openGAT(size int) *formats.GAT {
	g := &formats.GAT{
		Width:  uint32(size),
		Height: uint32(size),
		Cells:  make([]formats.GATCell, size*size),
	}
	for i := range g.Cells {
		g.Cells[i].Type = formats.GATWalkable
	}
	return g
}

// stateAt returns an in-game state with a pathfinder and a character standing
// on the given cell. No network: the tests drive predictWalk and
// handlePlayerMove directly.
func stateAt(cellX, cellY int) *InGameState {
	gat := openGAT(64)
	x, z := entity.CellToWorld(cellX, cellY)

	s := &InGameState{pathFinder: world.NewPathFinder(gat)}
	s.player = entity.NewCharacter(x, 0, z)
	return s
}

// encodePlayerMove builds a ZC_NOTIFY_PLAYERMOVE body with rAthena's packed
// start/end position pair.
func encodePlayerMove(startX, startY, endX, endY int) []byte {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint16(data[0:2], 0x0087)
	binary.LittleEndian.PutUint32(data[2:6], 1000)

	b := data[6:12]
	b[0] = byte(startX >> 2)
	b[1] = byte((startX << 6) | ((startY >> 4) & 0x3F))
	b[2] = byte((startY << 4) | ((endX >> 6) & 0x0F))
	b[3] = byte((endX << 2) | ((endY >> 8) & 0x03))
	b[4] = byte(endY)
	return data
}

func TestEncodePlayerMoveRoundTrips(t *testing.T) {
	// Guards the test helper itself: if this packing is wrong, every test
	// below is meaningless.
	data := encodePlayerMove(10, 20, 14, 22)
	mv := decodeForTest(data)
	if mv[0] != 10 || mv[1] != 20 || mv[2] != 14 || mv[3] != 22 {
		t.Fatalf("packed positions round-tripped as %v, want [10 20 14 22]", mv)
	}
}

func decodeForTest(data []byte) [4]int {
	b := data[6:12]
	return [4]int{
		int(b[0])<<2 | int(b[1])>>6,
		(int(b[1]) & 0x3F << 4) | int(b[2])>>4,
		(int(b[2]) & 0x0F << 6) | int(b[3])>>2,
		(int(b[3]) & 0x03 << 8) | int(b[4]),
	}
}

// TestPredictionStartsWalkingImmediately is the point of the whole exercise:
// the character sets off on the click, not a round trip later.
func TestPredictionStartsWalkingImmediately(t *testing.T) {
	s := stateAt(10, 10)

	s.predictWalk(10, 10, 14, 10)

	if !s.player.IsWalkingPath() {
		t.Fatal("character is not walking after a predicted move")
	}
	if !s.hasPrediction {
		t.Error("prediction was not recorded, so the matching ack cannot be recognised")
	}
}

// TestMatchingAckDoesNotRestartTheWalk is the regression guard for the subtle
// half. Re-issuing an identical path would reset progress through the current
// step, so the walk would take longer than it should and stutter on every
// acknowledgement — undoing the point of predicting.
func TestMatchingAckDoesNotRestartTheWalk(t *testing.T) {
	s := stateAt(10, 10)
	s.predictWalk(10, 10, 14, 10)

	// Get partway into the first step.
	s.player.Update(100)

	if err := s.handlePlayerMove(encodePlayerMove(10, 10, 14, 10)); err != nil {
		t.Fatalf("handlePlayerMove: %v", err)
	}

	if hits, total := s.PredictionAccuracy(); hits != 1 || total != 1 {
		t.Errorf("prediction accuracy = %d/%d, want 1/1", hits, total)
	}

	// Finish the walk and check it took the expected time rather than having
	// restarted a step.
	elapsed := 100.0
	for i := 0; i < 500 && s.player.IsWalkingPath(); i++ {
		s.player.Update(10)
		elapsed += 10
	}

	want := 4 * entity.DefaultWalkSpeedMs // four straight cells
	if elapsed > want+40 {
		t.Errorf("walk took %.0f ms, want about %.0f — the matching ack restarted a step",
			elapsed, want)
	}
}

// TestDivergentAckCorrects: when the server disagrees, it wins.
func TestDivergentAckCorrects(t *testing.T) {
	s := stateAt(10, 10)
	s.predictWalk(10, 10, 14, 10)
	s.player.Update(50)

	// Server says we actually walked somewhere else entirely.
	if err := s.handlePlayerMove(encodePlayerMove(10, 10, 10, 16)); err != nil {
		t.Fatalf("handlePlayerMove: %v", err)
	}

	if hits, _ := s.PredictionAccuracy(); hits != 0 {
		t.Error("a divergent ack should not count as a confirmed prediction")
	}

	for i := 0; i < 500 && s.player.IsWalkingPath(); i++ {
		s.player.Update(10)
	}

	gotX, gotY := s.player.CurrentCell()
	if gotX != 10 || gotY != 16 {
		t.Errorf("ended at cell (%d,%d), want the server's (10,16)", gotX, gotY)
	}
}

// TestDivergentAckDoesNotTeleport: correcting must still be walked off, not
// snapped, or prediction trades one visible artefact for another.
func TestDivergentAckDoesNotTeleport(t *testing.T) {
	s := stateAt(10, 10)
	s.predictWalk(10, 10, 14, 10)
	s.player.Update(120)

	before := s.player.RenderX

	if err := s.handlePlayerMove(encodePlayerMove(10, 10, 10, 16)); err != nil {
		t.Fatalf("handlePlayerMove: %v", err)
	}

	if jump := s.player.RenderX - before; jump > 0.001 || jump < -0.001 {
		t.Errorf("drawn position jumped %.3f units on correction, want no discontinuity", jump)
	}
}

// TestPredictionToleratesOneCellStartLead covers the case that made prediction
// only half work in practice.
//
// Measured against a live server, every mismatch had an identical destination
// and a start out by exactly one cell, with us ahead — which is prediction
// working, not failing: we set off on the input while the server sets off when
// the packet lands. Rejecting those threw away half the predictions and
// restarted a step each time.
func TestPredictionToleratesOneCellStartLead(t *testing.T) {
	tests := []struct {
		name                       string
		serverStartX, serverStartY int
		wantConfirmed              bool
	}{
		{"exact match", 10, 10, true},
		{"one cell behind in x", 9, 10, true},
		{"one cell behind in y", 10, 9, true},
		{"one cell behind diagonally", 9, 9, true},
		{"one cell ahead", 11, 10, true},
		{"two cells apart is a real disagreement", 8, 10, false},
		{"three cells apart", 10, 13, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := stateAt(10, 10)
			s.predictWalk(10, 10, 14, 10)

			// Same destination throughout; only the start moves.
			if err := s.handlePlayerMove(
				encodePlayerMove(tt.serverStartX, tt.serverStartY, 14, 10),
			); err != nil {
				t.Fatalf("handlePlayerMove: %v", err)
			}

			hits, total := s.PredictionAccuracy()
			if total != 1 {
				t.Fatalf("predictions = %d, want 1", total)
			}
			if confirmed := hits == 1; confirmed != tt.wantConfirmed {
				t.Errorf("confirmed = %v, want %v (server start (%d,%d) vs predicted (10,10))",
					confirmed, tt.wantConfirmed, tt.serverStartX, tt.serverStartY)
			}
		})
	}
}

// TestPredictionRequiresTheDestinationToMatch: the start has slack, the
// destination has none. Walking somewhere the server did not agree to is the
// one failure prediction must never hide.
func TestPredictionRequiresTheDestinationToMatch(t *testing.T) {
	s := stateAt(10, 10)
	s.predictWalk(10, 10, 14, 10)

	// Start identical, destination different by a single cell.
	if err := s.handlePlayerMove(encodePlayerMove(10, 10, 14, 11)); err != nil {
		t.Fatalf("handlePlayerMove: %v", err)
	}

	if hits, _ := s.PredictionAccuracy(); hits != 0 {
		t.Error("a different destination must never count as confirmed")
	}

	for i := 0; i < 500 && s.player.IsWalkingPath(); i++ {
		s.player.Update(10)
	}
	if gotX, gotY := s.player.CurrentCell(); gotX != 14 || gotY != 11 {
		t.Errorf("ended at (%d,%d), want the server's (14,11)", gotX, gotY)
	}
}

func TestCellsApart(t *testing.T) {
	tests := []struct {
		name                 string
		ax, ay, bx, by, want int
	}{
		{"same cell", 5, 5, 5, 5, 0},
		{"one east", 6, 5, 5, 5, 1},
		{"one diagonal", 6, 6, 5, 5, 1},
		{"chebyshev takes the larger axis", 5, 9, 5, 5, 4},
		{"negative direction", 5, 5, 9, 8, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cellsApart(tt.ax, tt.ay, tt.bx, tt.by); got != tt.want {
				t.Errorf("cellsApart = %d, want %d", got, tt.want)
			}
		})
	}
}

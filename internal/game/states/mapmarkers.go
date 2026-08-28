package states

import "github.com/Faultbox/midgard-ro/internal/game/entity"

// What to show on the map window besides the player.
//
// Only what the server has told us about, which is what is near enough to be
// in view. RO sends units as they come into range and takes them away again
// when they leave, so this is "who is around", not "every Kafra in Prontera".
// The client has no list of the second kind and inventing one from the map's
// NPC data is a different feature.

// MapMarker is one unit's place on the map.
type MapMarker struct {
	CellX, CellY int
	Type         entity.Type
}

// MapMarkers returns the units to draw on the map, our own character left out
// — it is drawn as the player mark and would otherwise be there twice.
func (s *InGameState) MapMarkers() []MapMarker {
	if s.entityManager == nil {
		return nil
	}

	self := s.selfAID()

	visible := s.entityManager.AllVisible()
	markers := make([]MapMarker, 0, len(visible))

	for _, e := range visible {
		if e == nil || e.ID == self {
			continue
		}

		cellX, cellY := entity.WorldToCell(e.Position.X, e.Position.Z)
		markers = append(markers, MapMarker{CellX: cellX, CellY: cellY, Type: e.Type})
	}

	return markers
}

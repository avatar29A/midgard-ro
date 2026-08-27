package states

import (
	gomath "math"
	"sync"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/camera"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// The archive's two camera tables, read once per session.
const (
	indoorTablePath    = "data\\indoorrswtable.txt"
	viewpointTablePath = "data\\viewpointtable.txt"
)

// MapRules are the client's own per-map camera rules: which maps are indoor
// and which have a preset (an arc, and an angle to enter at).
//
// Indoors the original disables orbiting and allows zoom. We do the reverse,
// by decision (Boris, 2026-08-27, while testing): the zoom is held at the
// default distance so a room is never seen from outside, and the camera may
// turn. Presets are applied as the table says.
// Missing tables are not an error — every map is then outdoor — but they are
// said once in the log, because a client that silently lost its indoor rules
// would look like a bug in the camera.
type MapRules struct {
	Indoor  map[string]bool
	Presets map[string]formats.ViewPoint
}

// MapCamera is what a map asks of the camera.
type MapCamera struct {
	Indoor bool
	Limits camera.Limits
	// YawIn is the angle to enter the map at, in radians.
	YawIn float32
}

// For returns a map's camera rules. The original's rotation table is in
// degrees with 0 as the default view; ours is radians with the same zero.
func (r *MapRules) For(mapName string) MapCamera {
	name := packets.MapBaseName(mapName)
	var mc MapCamera
	if r == nil {
		return mc
	}

	if r.Indoor[name] {
		mc.Indoor = true
		mc.Limits.ZoomLocked = true
		mc.Limits.FixedDistance = DefaultCameraZoom
	}
	if vp, ok := r.Presets[name]; ok {
		mc.YawIn = degToRad(vp.RotationIn)
		switch {
		case vp.Fixed():
			mc.Limits.YawLocked = true
		case !vp.Unrestricted():
			mc.Limits.Arc = true
			mc.Limits.YawMin = degToRad(vp.RotationFrom)
			mc.Limits.YawMax = degToRad(vp.RotationTo)
		}
	}
	return mc
}

func degToRad(deg float32) float32 {
	return deg * gomath.Pi / 180
}

// loadMapRules reads the tables. Either may be missing; what is there is
// used.
func loadMapRules(load func(string) ([]byte, error)) *MapRules {
	r := &MapRules{Indoor: map[string]bool{}, Presets: map[string]formats.ViewPoint{}}
	if load == nil {
		return r
	}

	if data, err := load(indoorTablePath); err != nil {
		logger.Warn("no indoor map table; every map will be treated as outdoor",
			zap.String("path", indoorTablePath), zap.Error(err))
	} else {
		r.Indoor = formats.ParseIndoorRSWTable(data)
	}

	if data, err := load(viewpointTablePath); err != nil {
		logger.Warn("no camera preset table; no map will restrict the camera to an arc",
			zap.String("path", viewpointTablePath), zap.Error(err))
	} else {
		r.Presets = formats.ParseViewpointTable(data)
	}

	logger.Info("map camera rules loaded",
		zap.Int("indoorMaps", len(r.Indoor)), zap.Int("presets", len(r.Presets)))
	return r
}

// mapRulesOnce guards the once-per-session load on the Manager.
type mapRulesOnce struct {
	once  sync.Once
	rules *MapRules
}

// MapRules returns the session's camera tables, reading them on first use.
func (m *Manager) MapRules() *MapRules {
	m.mapRules.once.Do(func() {
		m.mapRules.rules = loadMapRules(m.TexLoader)
	})
	return m.mapRules.rules
}

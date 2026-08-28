// Package trace provides opt-in, channel-scoped tracing for the parts of the
// client that are hard to debug from a screenshot — where a click landed, what
// the server said about it, and which cells we actually walked.
//
// Nothing is emitted unless the channel is switched on with --trace, so
// instrumentation can sit permanently in the hot paths. Enabled channels log
// at info level so they show up without having to also turn the global log
// level down and drown in packet spam.
//
// Usage:
//
//	--trace=move          one channel
//	--trace=move,pick     several
//	--trace=all           everything
package trace

import (
	"strings"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
)

// Channels. Keep these coarse — one per subsystem you'd debug as a unit.
const (
	// Move covers walk requests, server acknowledgements and cell stepping.
	Move = "move"
	// Pick covers screen-to-world ray casting for click-to-move.
	Pick = "pick"
	// Net covers packet-level send/receive framing.
	Net = "net"
	// Render covers per-frame timing: how long update, scene and UI each take,
	// and how much of the frame is spent blocked in the buffer swap.
	Render = "render"
	// Status covers the parameter updates the server pushes — HP, SP, levels,
	// experience, weight and Zeny — including the ids we do not map yet.
	Status = "status"
	// NPC covers a conversation from the click that starts it to the close
	// that ends it. The net channel shows the same packets, but as a stream
	// of ids; this one reads as the conversation, which is what makes a
	// dialog that stopped halfway legible.
	NPC = "npc"
	// HUD covers what the in-game interface is showing: which windows are
	// open, what the minimap resolved, how many lines chat holds, where the
	// click marker sits. Stat values stay on Status — reporting the same
	// change on two channels would split the trail rather than widen it.
	HUD = "hud"
	// Map covers a map change from the packet that orders it to the one that
	// says we are ready: the load's phases and their timings, what was kept
	// and dropped on the way, and the camera rules the new map brought.
	Map = "map"
	// Cmd covers a chat command from the line that was typed to what became
	// of it. A command that silently does nothing is otherwise
	// indistinguishable from one that was never recognized, and the two want
	// opposite fixes.
	Cmd = "cmd"
)

// All is the channel spec that turns everything on.
const All = "all"

// enabled is a bitmask-free set of active channels. Read on every Emit call
// from the render thread, written once at startup, so it's kept in an atomic
// pointer rather than a mutex-guarded map.
var enabled atomic.Pointer[map[string]bool]

// Enable turns on the channels named in spec, a comma-separated list. An empty
// spec disables tracing entirely; "all" enables every channel.
func Enable(spec string) {
	set := map[string]bool{}
	for _, name := range strings.Split(spec, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name != "" {
			set[name] = true
		}
	}
	enabled.Store(&set)

	if len(set) > 0 {
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		logger.Info("tracing enabled", zap.Strings("channels", names))
	}
}

// On reports whether a channel is active. Use it to guard trace calls whose
// arguments are expensive to compute.
func On(channel string) bool {
	set := enabled.Load()
	if set == nil {
		return false
	}
	return (*set)[channel] || (*set)[All]
}

// Emit logs one trace event on a channel. The event name is a short verb
// phrase scoped to the channel — "move.request", "pick.hit" — so a trace reads
// as a sequence of steps through the pipeline.
func Emit(channel, event string, fields ...zap.Field) {
	if !On(channel) {
		return
	}
	logger.Info("trace "+channel+"."+event, fields...)
}

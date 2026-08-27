package ui

import (
	"strings"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/trace"
)

// HUDWindow is one of the windows the menu buttons open.
//
// Only the four the first row promises are named. The rest of the strip —
// party, guild, quest and the others — still opens nothing, and a name here
// would suggest otherwise.
type HUDWindow string

// The windows the menu buttons open. The values match the button names in
// hudMenuButtons, so a button knows its own window without a second table.
const (
	WindowInfo  HUDWindow = "info"
	WindowSkill HUDWindow = "skill"
	WindowItem  HUDWindow = "item"
	WindowMap   HUDWindow = "map"
)

// hudWindowFrames are the frame ids the windows draw under, so opening one
// from its menu button can clear the closed and minimized flags its own title
// bar set. Without that a window closes once and the button never brings it
// back.
var hudWindowFrames = map[HUDWindow]string{
	WindowInfo:  statsWindowID,
	WindowSkill: skillsWindowID,
}

// hudWindowTitles are what each window calls itself, matching the original.
var hudWindowTitles = map[HUDWindow]string{
	WindowInfo:  "Status",
	WindowSkill: "Skill",
	WindowItem:  "Item",
	WindowMap:   "Map",
}

// opensWindow reports whether a menu button toggles a window, and which.
//
// A button with no window keeps the behavior it had before: it draws and
// hovers, and clicking it does nothing quietly. Announcing a click that
// changes nothing reads as a fault.
func opensWindow(buttonName string) (HUDWindow, bool) {
	w := HUDWindow(buttonName)
	_, ok := hudWindowTitles[w]

	return w, ok
}

// IsWindowOpen reports whether a menu window is currently open.
func (b *UI2DBackend) IsWindowOpen(w HUDWindow) bool {
	return b.hudOpen[w]
}

// ToggleWindow opens a closed menu window or closes an open one, and reports
// the state it left it in.
func (b *UI2DBackend) ToggleWindow(w HUDWindow) bool {
	if b.hudOpen == nil {
		b.hudOpen = make(map[HUDWindow]bool, len(hudWindowTitles))
	}

	open := !b.hudOpen[w]
	if open {
		// Clearing the frame's own closed and minimized flags, so a window
		// put away from its title bar comes back from its button. The nil
		// check is for a backend built without a context, which the window
		// state's own tests do.
		if frame, ok := hudWindowFrames[w]; ok && b.ctx != nil {
			b.ctx.OpenWindow(frame)
		}

		b.hudOpen[w] = true
	} else {
		// Deleted rather than set false, so the map holds only what is open
		// and OpenWindowCount can just be its length.
		delete(b.hudOpen, w)
	}

	trace.Emit(trace.HUD, "toggle",
		zap.String("window", string(w)),
		zap.Bool("open", open))

	return open
}

// OpenWindowCount is how many menu windows are open, for the debug overlay.
func (b *UI2DBackend) OpenWindowCount() int {
	return len(b.hudOpen)
}

// describeHUD summarizes the HUD for the debug overlay: which menu windows are
// open, or that none are.
//
// Listed in the strip's own order rather than map order, so two screenshots of
// the same state read the same.
func (b *UI2DBackend) describeHUD() string {
	if len(b.hudOpen) == 0 {
		return "no windows open"
	}

	open := make([]string, 0, len(b.hudOpen))
	for _, name := range hudMenuButtons {
		if w, ok := opensWindow(name); ok && b.hudOpen[w] {
			open = append(open, hudWindowTitles[w])
		}
	}

	return strings.Join(open, ", ")
}

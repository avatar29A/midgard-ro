package states

import "fmt"

// DialogPhase is where a conversation with an NPC has got to.
//
// The server drives every transition: it sends text, then either asks for a
// Next, offers a menu, or asks for a Close. The client never decides on its
// own what comes next — which is why a stuck conversation is almost always a
// packet we did not handle, and why the phase is worth putting on screen.
type DialogPhase int

// The phases, in the order a conversation moves through them.
const (
	// DialogIdle is not talking to anyone.
	DialogIdle DialogPhase = iota
	// DialogText is showing what the NPC said, with nothing asked of us yet.
	DialogText
	// DialogWaitingNext is showing Next: the script has more to say.
	DialogWaitingNext
	// DialogWaitingClose is showing Close: the script is finished.
	DialogWaitingClose
	// DialogMenu is showing a list of choices.
	DialogMenu
)

// String names the phase for the debug overlay and the trace.
func (p DialogPhase) String() string {
	switch p {
	case DialogIdle:
		return "idle"
	case DialogText:
		return "text"
	case DialogWaitingNext:
		return "waiting-next"
	case DialogWaitingClose:
		return "waiting-close"
	case DialogMenu:
		return "menu"
	default:
		return fmt.Sprintf("unknown(%d)", int(p))
	}
}

// NPCDialog is the conversation the player is in, if any.
//
// The id is the one the server put in the packet, not an entity we looked up:
// rAthena sends a fake npc id when the script's owner is not a unit near the
// player, so there may be nothing in the entity manager to match it.
type NPCDialog struct {
	Phase DialogPhase

	// NPCID is who is talking, as the server identifies them.
	NPCID uint32

	// Name is the entity's name if we happen to know it, and empty if we do
	// not. It is for the debug overlay only — the NPC's name shown to the
	// player is written by the script into the message text.
	Name string

	// MenuItems is how many choices the last menu offered, kept so an
	// out-of-range selection can be refused before it is sent.
	MenuItems int
}

// Dialog returns the conversation in progress. The zero value is idle, so a
// caller never has to check for nil.
func (s *InGameState) Dialog() NPCDialog {
	if s == nil {
		return NPCDialog{}
	}

	return s.dialog
}

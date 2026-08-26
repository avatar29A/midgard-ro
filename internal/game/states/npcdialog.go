package states

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

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

	// Menu is the choices the server last offered, already split. Its length
	// is the bound a selection has to stay inside — see ChooseMenuItem.
	Menu []string

	// Message is what the NPC last said, exactly as the script wrote it —
	// color codes and line breaks included. Interpreting it belongs to
	// whatever draws it.
	Message string
}

// handleSayDialog shows what the NPC said.
//
// The npc id comes from the packet and is not looked up: rAthena sends a fake
// one for scripts whose owner is not a unit near the player, so there may be
// nothing in the entity manager to match it.
func (s *InGameState) handleSayDialog(data []byte) error {
	say := packets.DecodeSayDialog(data)
	if say == nil {
		logger.Warn("malformed NPC dialog packet", zap.Int("bytes", len(data)))
		return nil
	}

	// The original keeps one box per conversation and appends to it, so a
	// script that says three things in a row reads as three paragraphs rather
	// than replacing itself twice before you can read it. A message from a
	// different NPC, or after a close, starts fresh.
	if s.dialog.Phase == DialogIdle || s.dialog.NPCID != say.NPCID {
		s.dialog.Message = say.Message
	} else {
		s.dialog.Message += "\n" + say.Message
	}

	s.dialog.Phase = DialogText
	s.dialog.NPCID = say.NPCID
	s.dialog.Menu = nil
	s.dialog.Name = s.entityName(say.NPCID)

	trace.Emit(trace.NPC, "say",
		zap.Uint32("npcID", say.NPCID),
		zap.Int("bytes", len(data)),
		zap.Int("chars", len(say.Message)))

	return nil
}

// handleMenuList shows the choices the script is waiting on.
func (s *InGameState) handleMenuList(data []byte) error {
	menu := packets.DecodeMenuList(data)
	if menu == nil {
		logger.Warn("malformed NPC menu packet", zap.Int("bytes", len(data)))
		return nil
	}

	s.dialog.Phase = DialogMenu
	s.dialog.NPCID = menu.NPCID
	s.dialog.Menu = menu.Items

	if s.dialog.Name == "" {
		s.dialog.Name = s.entityName(menu.NPCID)
	}

	trace.Emit(trace.NPC, "menu",
		zap.Uint32("npcID", menu.NPCID), zap.Int("items", len(menu.Items)))

	return nil
}

// ChooseMenuItem answers a menu with a one-based selection.
//
// The bound is checked before anything is sent. rAthena calls clif_GM_kick for
// a selection of zero or past the end, so a wrong index disconnects the player
// rather than branching the script oddly — which makes this the one place in
// the conversation where being careful is not merely tidy.
func (s *InGameState) ChooseMenuItem(choice int) {
	if s == nil || s.client == nil || s.dialog.Phase != DialogMenu {
		return
	}

	npcID := s.dialog.NPCID

	pkt, err := packets.ChooseMenu(npcID, choice, len(s.dialog.Menu))
	if err != nil {
		logger.Warn("refusing to send a menu choice the server would kick us for",
			zap.Int("choice", choice), zap.Int("items", len(s.dialog.Menu)), zap.Error(err))

		return
	}

	if err := s.client.Send(pkt); err != nil {
		logger.Warn("could not answer NPC menu", zap.Uint32("npcID", npcID), zap.Error(err))
		return
	}

	// The script decides what comes next; until it says, there is no menu.
	s.dialog.Phase = DialogText
	s.dialog.Menu = nil

	trace.Emit(trace.NPC, "choose", zap.Uint32("npcID", npcID), zap.Int("choice", choice))
}

// CancelMenuChoice backs out of a menu, which ends the conversation.
//
// Canceling closes the whole window, not just the menu, because the server
// will never tell us to: `buildin_menu` handles 255 with `st->state = END`
// (script.cpp:5174) and sends nothing back, on the assumption the client has
// already closed its own window. Leaving the text up would strand the player
// with a dialog that has no button and no way out — the Close button only
// appears when the server asks for it, and it never will.
//
// A script that used `prompt` rather than `select` carries on and sends more
// text; that arrives with the dialog idle and simply opens it again.
func (s *InGameState) CancelMenuChoice() {
	if s == nil || s.client == nil || s.dialog.Phase != DialogMenu {
		return
	}

	npcID := s.dialog.NPCID

	if err := s.client.Send(packets.CancelMenu(npcID)); err != nil {
		logger.Warn("could not cancel NPC menu", zap.Uint32("npcID", npcID), zap.Error(err))
		return
	}

	s.dialog = NPCDialog{}

	trace.Emit(trace.NPC, "cancel", zap.Uint32("npcID", npcID))
}

// handleWaitDialog is the server asking for a Next button.
func (s *InGameState) handleWaitDialog(data []byte) error {
	return s.setDialogPhase(data, DialogWaitingNext, "wait")
}

// handleCloseDialog is the server asking for a Close button. It does not end
// the conversation — the player has to press it, and the script is still
// waiting until they do.
func (s *InGameState) handleCloseDialog(data []byte) error {
	return s.setDialogPhase(data, DialogWaitingClose, "close-offered")
}

// setDialogPhase handles the two packets that carry nothing but an npc id.
func (s *InGameState) setDialogPhase(data []byte, phase DialogPhase, event string) error {
	npcID, ok := packets.DecodeDialogNPCID(data)
	if !ok {
		logger.Warn("malformed NPC dialog packet", zap.Int("bytes", len(data)))
		return nil
	}

	s.dialog.Phase = phase
	s.dialog.NPCID = npcID

	if s.dialog.Name == "" {
		s.dialog.Name = s.entityName(npcID)
	}

	trace.Emit(trace.NPC, event, zap.Uint32("npcID", npcID))

	return nil
}

// AdvanceDialog asks the script to carry on, which the Next button does.
func (s *InGameState) AdvanceDialog() {
	if s == nil || s.client == nil || s.dialog.Phase != DialogWaitingNext {
		return
	}

	npcID := s.dialog.NPCID

	if err := s.client.Send(packets.RequestNextScript(npcID)); err != nil {
		logger.Warn("could not advance NPC dialog", zap.Uint32("npcID", npcID), zap.Error(err))
		return
	}

	// Back to plain text until the server says what it wants next. Without
	// this the button stays on screen and a second press sends a second
	// request the script is not waiting for.
	s.dialog.Phase = DialogText

	trace.Emit(trace.NPC, "next", zap.Uint32("npcID", npcID))
}

// EndDialog closes the conversation, which the Close button does.
func (s *InGameState) EndDialog() {
	if s == nil || s.dialog.Phase == DialogIdle {
		return
	}

	npcID := s.dialog.NPCID

	if s.client != nil {
		if err := s.client.Send(packets.CloseDialogPacket(npcID)); err != nil {
			logger.Warn("could not close NPC dialog", zap.Uint32("npcID", npcID), zap.Error(err))
		}
	}

	// Cleared whether or not the send worked: leaving a window the player has
	// dismissed on screen is worse than the server thinking we are still
	// talking, which its own timeout resolves.
	s.dialog = NPCDialog{}

	trace.Emit(trace.NPC, "close", zap.Uint32("npcID", npcID))
}

// entityName is the unit's name if we happen to know it. Empty is normal and
// not an error — see handleSayDialog.
func (s *InGameState) entityName(npcID uint32) string {
	if s.entityManager == nil {
		return ""
	}

	if e := s.entityManager.Get(npcID); e != nil {
		return e.Name
	}

	return ""
}

// Dialog returns the conversation in progress. The zero value is idle, so a
// caller never has to check for nil.
func (s *InGameState) Dialog() NPCDialog {
	if s == nil {
		return NPCDialog{}
	}

	return s.dialog
}

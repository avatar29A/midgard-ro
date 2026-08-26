package states

import "testing"

func TestDialogPhaseString(t *testing.T) {
	tests := []struct {
		phase DialogPhase
		want  string
	}{
		{DialogIdle, "idle"},
		{DialogText, "text"},
		{DialogWaitingNext, "waiting-next"},
		{DialogWaitingClose, "waiting-close"},
		{DialogMenu, "menu"},
		{DialogPhase(99), "unknown(99)"},
	}

	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.want {
			t.Errorf("DialogPhase(%d).String() = %q, want %q", int(tt.phase), got, tt.want)
		}
	}
}

// TestDialogZeroValueIsIdle pins that a state which has never talked to anyone
// reports idle rather than needing to be initialized first.
func TestDialogZeroValueIsIdle(t *testing.T) {
	var s InGameState

	if got := s.Dialog(); got.Phase != DialogIdle || got.NPCID != 0 {
		t.Errorf("Dialog() = %+v, want an idle zero value", got)
	}

	var nilState *InGameState
	if got := nilState.Dialog(); got.Phase != DialogIdle {
		t.Errorf("Dialog() on nil = %+v, want idle", got)
	}
}

// sayPacket builds a ZC_SAY_DIALOG the way the server does.
func sayPacket(npcID uint32, message string) []byte {
	body := append([]byte(message), 0)
	buf := make([]byte, 8+len(body))
	buf[0], buf[1] = 0xB4, 0x00
	buf[2], buf[3] = byte(len(buf)), byte(len(buf)>>8)
	buf[4], buf[5] = byte(npcID), byte(npcID>>8)
	buf[6], buf[7] = byte(npcID>>16), byte(npcID>>24)
	copy(buf[8:], body)

	return buf
}

// TestDialogTextAccumulates pins the behavior that makes a conversation
// readable: the original keeps one box per conversation and appends to it, so
// a script saying three things in a row reads as three paragraphs instead of
// replacing itself twice before the player can read them.
func TestDialogTextAccumulates(t *testing.T) {
	var s InGameState

	if err := s.handleSayDialog(sayPacket(42, "First.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}
	if s.dialog.Message != "First." {
		t.Errorf("Message = %q, want the first line alone", s.dialog.Message)
	}
	if s.dialog.Phase != DialogText || s.dialog.NPCID != 42 {
		t.Errorf("dialog = %+v, want text/42", s.dialog)
	}

	if err := s.handleSayDialog(sayPacket(42, "Second.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}
	// Consecutive lines of one page are just lines: every `mes` arrives as
	// its own packet, so a blank line here would double-space everything,
	// including between a speaker's name and what they say.
	if s.dialog.Message != "First.\nSecond." {
		t.Errorf("Message = %q, want both lines with no blank between", s.dialog.Message)
	}

	// A different NPC is a different conversation and starts over.
	if err := s.handleSayDialog(sayPacket(99, "Elsewhere.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}
	if s.dialog.Message != "Elsewhere." {
		t.Errorf("Message = %q, want only the new NPC's line", s.dialog.Message)
	}
}

func TestDialogPhaseFromPackets(t *testing.T) {
	var s InGameState

	if err := s.handleSayDialog(sayPacket(42, "Hello.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}

	// 00b5 <npc id>.L asks for Next; 00b6 asks for Close.
	if err := s.handleWaitDialog([]byte{0xB5, 0x00, 42, 0, 0, 0}); err != nil {
		t.Fatalf("handleWaitDialog: %v", err)
	}
	if s.dialog.Phase != DialogWaitingNext {
		t.Errorf("phase = %v, want waiting-next", s.dialog.Phase)
	}

	if err := s.handleCloseDialog([]byte{0xB6, 0x00, 42, 0, 0, 0}); err != nil {
		t.Fatalf("handleCloseDialog: %v", err)
	}
	if s.dialog.Phase != DialogWaitingClose {
		t.Errorf("phase = %v, want waiting-close", s.dialog.Phase)
	}

	// Offering Close does not end the conversation — the player has to press
	// it, and the script is still waiting until they do.
	if s.dialog.Message == "" {
		t.Error("the message was cleared when Close was merely offered")
	}
}

// TestEndDialogClearsEvenWithoutAServer pins that the window goes away when
// the player dismisses it. Leaving it up is worse than the server briefly
// thinking we are still talking, which its own timeout resolves.
func TestEndDialogClears(t *testing.T) {
	var s InGameState

	if err := s.handleSayDialog(sayPacket(42, "Hello.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}

	s.EndDialog()

	if s.dialog.Phase != DialogIdle || s.dialog.NPCID != 0 ||
		s.dialog.Message != "" || len(s.dialog.Menu) != 0 {
		t.Errorf("dialog = %+v, want the zero value", s.dialog)
	}
}

func TestMalformedDialogPacketsAreRefused(t *testing.T) {
	var s InGameState

	if err := s.handleSayDialog([]byte{0xB4, 0x00, 0x03}); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}
	if err := s.handleWaitDialog([]byte{0xB5, 0x00}); err != nil {
		t.Fatalf("handleWaitDialog: %v", err)
	}

	if s.dialog.Phase != DialogIdle {
		t.Errorf("a malformed packet opened a dialog: %+v", s.dialog)
	}
}

// menuPacket builds a ZC_MENU_LIST the way the server does: one
// colon-separated string, NUL-terminated, inside the packet's own length.
func menuPacket(npcID uint32, menu string) []byte {
	body := append([]byte(menu), 0)
	buf := make([]byte, 8+len(body))
	buf[0], buf[1] = 0xB7, 0x00
	buf[2], buf[3] = byte(len(buf)), byte(len(buf)>>8)
	buf[4], buf[5] = byte(npcID), byte(npcID>>8)
	buf[6], buf[7] = byte(npcID>>16), byte(npcID>>24)
	copy(buf[8:], body)

	return buf
}

func TestHandleMenuList(t *testing.T) {
	var s InGameState

	if err := s.handleMenuList(menuPacket(42, "Save:Use Storage:Cancel")); err != nil {
		t.Fatalf("handleMenuList: %v", err)
	}

	if s.dialog.Phase != DialogMenu {
		t.Errorf("phase = %v, want menu", s.dialog.Phase)
	}
	if len(s.dialog.Menu) != 3 || s.dialog.Menu[1] != "Use Storage" {
		t.Errorf("Menu = %q, want the three choices", s.dialog.Menu)
	}
}

// TestMenuEmptyEntriesDoNotCount is the one that keeps the player connected.
// The server counts only non-empty options into sd->npc_menu and kicks a
// selection past that bound, so `a::b` offers two choices and `b` is choice 2.
func TestMenuEmptyEntriesDoNotCount(t *testing.T) {
	var s InGameState

	if err := s.handleMenuList(menuPacket(42, "a::b")); err != nil {
		t.Fatalf("handleMenuList: %v", err)
	}

	if len(s.dialog.Menu) != 2 {
		t.Fatalf("Menu = %q, want two selectable choices", s.dialog.Menu)
	}
	if s.dialog.Menu[1] != "b" {
		t.Errorf("second choice = %q, want %q", s.dialog.Menu[1], "b")
	}
}

// TestChooseMenuItemNeedsAMenu pins that a stray choice cannot be sent when no
// menu is open — with no client wired here, reaching the send would panic, so
// the guard is what this proves.
func TestChooseMenuItemNeedsAMenu(t *testing.T) {
	var s InGameState

	s.ChooseMenuItem(1)
	s.CancelMenuChoice()

	if s.dialog.Phase != DialogIdle {
		t.Errorf("phase = %v, want idle", s.dialog.Phase)
	}
}

func TestMalformedMenuIsRefused(t *testing.T) {
	var s InGameState

	if err := s.handleMenuList([]byte{0xB7, 0x00, 0x04}); err != nil {
		t.Fatalf("handleMenuList: %v", err)
	}

	if s.dialog.Phase != DialogIdle || len(s.dialog.Menu) != 0 {
		t.Errorf("a malformed menu opened one: %+v", s.dialog)
	}
}

// TestCancelWithoutAClientLeavesTheDialogAlone guards the ordering in
// CancelMenuChoice: it must not clear the conversation when the answer never
// reached the server, or the player loses the window and the script keeps
// waiting.
func TestCancelWithoutAClientLeavesTheDialogAlone(t *testing.T) {
	var s InGameState

	if err := s.handleSayDialog(sayPacket(42, "Hello.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}
	if err := s.handleMenuList(menuPacket(42, "a:b")); err != nil {
		t.Fatalf("handleMenuList: %v", err)
	}

	s.CancelMenuChoice()

	if s.dialog.Phase != DialogMenu {
		t.Errorf("phase = %v, want the menu still open when nothing was sent", s.dialog.Phase)
	}
}

// TestParagraphBreakFollowsNext pins where the blank line goes: between pages,
// not between the lines of one. Every `mes` is its own packet, so putting it
// between messages would double-space the whole conversation — and separate a
// speaker's name from what they then say.
func TestParagraphBreakFollowsNext(t *testing.T) {
	var s InGameState

	for _, line := range []string{"[Guide]", "Welcome."} {
		if err := s.handleSayDialog(sayPacket(42, line)); err != nil {
			t.Fatalf("handleSayDialog: %v", err)
		}
	}

	if s.dialog.Message != "[Guide]\nWelcome." {
		t.Fatalf("Message = %q, want the name and its line together", s.dialog.Message)
	}

	// The player pressed Next: what comes back starts a new block.
	s.dialog.newParagraph = true

	if err := s.handleSayDialog(sayPacket(42, "[Guide]")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}
	if err := s.handleSayDialog(sayPacket(42, "More.")); err != nil {
		t.Fatalf("handleSayDialog: %v", err)
	}

	want := "[Guide]\nWelcome.\n\n[Guide]\nMore."
	if s.dialog.Message != want {
		t.Errorf("Message = %q, want %q", s.dialog.Message, want)
	}
}

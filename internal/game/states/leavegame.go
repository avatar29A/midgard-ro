package states

import (
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"

	"go.uber.org/zap"
)

// Leaving the game, both ways the ESC menu offers.
//
// Neither button acts on its own press. The server can refuse both — you
// cannot leave while cloaking or hiding, and rAthena's prevent_logout holds
// you for a few seconds after combat — so the request goes out and the
// decision arrives as its own packet. Tearing the socket down on the press
// would leave the server holding a session it had decided to keep.

// QuitFunc is what actually ends the process, handed in so the state does not
// have to know how the window is torn down.
type QuitFunc func()

// SetQuitFunc gives the state a way to end the process once the server has
// agreed to let go.
func (s *InGameState) SetQuitFunc(quit QuitFunc) {
	s.quit = quit
}

// RequestCharSelect asks to go back to character select.
func (s *InGameState) RequestCharSelect() error {
	trace.Emit(trace.HUD, "leave-request", zap.String("to", "charselect"))

	return s.client.Send(packets.EncodeRestart(packets.RestartCharSelect))
}

// RequestRespawn asks to be put back at the save point after dying.
//
// The same packet as a return to character select with a different byte, and
// the server answers it the same way: with a fresh map, at whichever Kafra
// the character last registered with. Nothing here has to know where that is.
func (s *InGameState) RequestRespawn() error {
	trace.Emit(trace.HUD, "leave-request", zap.String("to", "respawn"))

	return s.client.Send(packets.EncodeRestart(packets.RestartRespawn))
}

// Dead reports whether the character is lying down.
//
// Not read off the hit points, which was the first thing tried and does not
// work: rAthena leaves a corpse on one hit point rather than on nought, so a
// character who has just died looks like one with a sliver left, and one who
// logs in dead looks perfectly well. The server says so in its own packet
// instead — the same ZC_NOTIFY_VANISH it clears a dead monster with, sent for
// the character being played.
func (s *InGameState) Dead() bool {
	return s.playerDead
}

// handleResurrection is somebody being stood back up.
//
// Only the character being played matters here; a priest raising somebody
// else changes nothing about this screen. There is nothing to undo besides
// the flag: the sprite stands up from the same packet the entity layer reads.
func (s *InGameState) handleResurrection(data []byte) error {
	aid, ok := packets.DecodeResurrection(data)
	if !ok {
		return nil
	}

	if aid != s.selfAID() {
		return nil
	}

	trace.Emit(trace.HUD, "resurrected", zap.Uint32("aid", aid))

	s.standUp()

	return nil
}

// RequestQuit asks to leave the game.
func (s *InGameState) RequestQuit() error {
	trace.Emit(trace.HUD, "leave-request", zap.String("to", "quit"))

	return s.client.Send(packets.EncodeDisconnect())
}

// handleKicked is the server throwing us off.
//
// It sends this and then closes the socket. Nothing else says so, and a client
// that does not read it goes on drawing a world it is no longer connected to:
// every click after that goes nowhere, which reads as the game having quietly
// stopped working. It is said out loud instead, in the one place the player is
// already reading.
func (s *InGameState) handleKicked(data []byte) error {
	reason, ok := packets.DecodeKicked(data)
	if !ok {
		logger.Warn("short disconnect packet", zap.Int("len", len(data)))

		return nil
	}

	words := packets.KickedReason(reason)

	logger.Warn("thrown off the server",
		zap.Uint8("reason", reason), zap.String("why", words))
	trace.Emit(trace.HUD, "kicked", zap.Uint8("reason", reason))

	s.chat.AddLocal(ChatError, words)
	s.chat.AddLocal(ChatError, "Not connected any more — log in again.")

	// Whatever was open belongs to a conversation that no longer exists.
	s.CloseShop()
	s.dropDialog()

	return nil
}

// handleRestartAck acts on the server granting a return to character select.
func (s *InGameState) handleRestartAck(data []byte) error {
	kind, ok := packets.DecodeRestartAck(data)
	if !ok {
		return nil
	}

	trace.Emit(trace.HUD, "restart-ack", zap.Uint8("type", kind))

	if kind != packets.RestartCharSelect {
		// A respawn, which belongs to dying rather than to this menu.
		return nil
	}

	host, port := s.manager.Session.CharServerHost, s.manager.Session.CharServerPort
	if host == "" || port == 0 {
		logger.Warn("granted character select with no character server to return to")

		return nil
	}

	// The map server is done with us; the character server expects a fresh
	// connection, the same way login hands over to it.
	s.client.Disconnect()
	s.manager.Change(NewConnectingState(ConnectingStateConfig{
		NextState:  "charselect",
		ServerHost: host,
		ServerPort: port,
	}, s.client, s.manager))

	return nil
}

// handleDisconnectAck acts on the server's answer to a request to quit.
func (s *InGameState) handleDisconnectAck(data []byte) error {
	result, ok := packets.DecodeDisconnectAck(data)
	if !ok {
		return nil
	}

	trace.Emit(trace.HUD, "disconnect-ack", zap.Uint16("result", result))

	if result != packets.DisconnectGranted {
		// Refused, and the reason is not ours to guess: rAthena says no while
		// hidden and for a few seconds after combat. Staying in the game is
		// the honest outcome — the menu is still open to try again.
		logger.Info("server refused the request to quit", zap.Uint16("result", result))

		return nil
	}

	s.SaveUIState()
	s.client.Disconnect()

	if s.quit != nil {
		s.quit()
	}

	return nil
}

package states

// The ghost that hangs over a corpse.
//
// RO's own, and one of the things everybody remembers about dying in it: a
// small white ghost with its eyes shut floats above the body until the
// character is on their feet again. Without it a dead character is only a
// character lying down, which reads as one who has fallen over.
//
// Nothing on the wire asks for it. The server says the character has died and
// stops there — no effect packet is sent for it, in rAthena or anywhere else
// — so the original's client plays this off its own knowledge of the death,
// and so does this one.
//
// The art says the rest. 유령 in the effect folder is eight little sprites and
// an ACT that hangs them fifty-five pixels above the point it is played at,
// bobs them a pixel or two, and asks for an alpha of 150 out of 255: solid
// enough to read as a thing, thin enough to see the ground through.
const deadGhost = "유령"

// playDeadGhost puts one over the character.
//
// It follows the body rather than standing where the body fell, which is the
// same thing while dead — a corpse does not go anywhere — and the right thing
// if it ever is: whatever moves a dead character takes the ghost with it.
//
// No length is given, so it plays until something takes it away. That is
// standing up, in every way a character can.
func (s *InGameState) playDeadGhost() {
	s.playSpriteEffect(deadGhost, s.selfAID(), 0, 0, 0, 0)
}

// standUp is the character getting back up, however it happened.
//
// Two things do it — a priest's resurrection and the walk to the save point,
// which is not a resurrection at all but a warp — and both have to put away
// everything that being dead put up. Kept together so that neither can be
// given a third thing to undo without the other one getting it too.
func (s *InGameState) standUp() {
	s.playerDead = false

	if s.player != nil {
		s.player.Revive()
	}

	s.stopSpriteEffect(deadGhost)
}

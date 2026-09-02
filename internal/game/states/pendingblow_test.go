package states

import (
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// blowAt builds a landed blow between two ids.
func blowAt(target uint32, amount int) packets.Damage {
	// The attacker is nobody in particular: these tests are about when the
	// blow lands, not who threw it.
	const attacker = 1

	return packets.Damage{SourceID: attacker, TargetID: target, Amount: amount, Hits: 1}
}

// withMob returns a state holding one monster, and the monster.
func withMob() (*InGameState, *entity.Entity) {
	// One monster, id 7, standing where nothing else is.
	s := &InGameState{entityManager: entity.NewManager()}
	e := mobAt(7, 100, 100)
	s.entityManager.Add(e)

	return s, e
}

// TestPendingBlowWaitsForItsHitFrame: the figure is what the player reads as
// "that landed", and it belongs at the moment the blade arrives rather than
// at the moment the swing begins.
func TestPendingBlowWaitsForItsHitFrame(t *testing.T) {
	s, _ := withMob()
	s.pendingBlows = []pendingBlow{{blow: blowAt(7, 12), remainingMs: 500}}

	s.advancePendingBlows(200)
	if len(s.damageNumbers) != 0 {
		t.Fatalf("the figure showed after 200ms of a 500ms wait")
	}
	if len(s.pendingBlows) != 1 {
		t.Fatalf("the blow was dropped early")
	}

	s.advancePendingBlows(300)
	if len(s.damageNumbers) != 1 {
		t.Errorf("the figure did not show at the hit frame: %d on screen", len(s.damageNumbers))
	}
	if len(s.pendingBlows) != 0 {
		t.Errorf("a landed blow is still pending")
	}
}

// TestPendingBlowKillsAtTheHitFrame is what this is all for: the server sends
// the death while the sword is still traveling, and a monster that falls
// over then died before it was hit.
func TestPendingBlowKillsAtTheHitFrame(t *testing.T) {
	s, mob := withMob()
	s.pendingBlows = []pendingBlow{{blow: blowAt(7, 12), remainingMs: 500}}

	if !s.killDeferred(7) {
		t.Fatal("a blow in flight did not take the death")
	}

	s.advancePendingBlows(400)
	if mob.IsDead {
		t.Error("the monster died before the blow reached it")
	}

	s.advancePendingBlows(100)
	if !mob.IsDead {
		t.Error("the monster is still standing after the blow landed")
	}
}

// TestKillDeferredRefusesWhatIsNotInFlight: a unit dying with nothing on its
// way — killed by someone off screen, or vanishing for another reason — has
// nothing to wait for and must be laid down at once.
func TestKillDeferredRefusesWhatIsNotInFlight(t *testing.T) {
	s, _ := withMob()

	if s.killDeferred(7) {
		t.Error("a death was deferred with no blow in flight")
	}

	s.pendingBlows = []pendingBlow{{blow: blowAt(9, 5), remainingMs: 500}}
	if s.killDeferred(7) {
		t.Error("a death was attached to a blow aimed at somebody else")
	}
}

// TestKillDeferredTakesTheLastBlow: several blows can be in flight at once,
// and the one that killed is the one queued last. Attaching the death to an
// earlier one drops the target on a hit it survived.
func TestKillDeferredTakesTheLastBlow(t *testing.T) {
	s, _ := withMob()
	s.pendingBlows = []pendingBlow{
		{blow: blowAt(7, 5), remainingMs: 200},
		{blow: blowAt(7, 9), remainingMs: 500},
	}

	s.killDeferred(7)

	if s.pendingBlows[0].kills {
		t.Error("the death went to the first blow, which the target survived")
	}
	if !s.pendingBlows[1].kills {
		t.Error("the death did not go to the last blow")
	}
}

// TestPendingBlowsAreForgotten: a map change takes away everything a blow was
// going to happen to, and a figure floating over the last map is worse than
// one that never appeared.
func TestPendingBlowsAreForgotten(t *testing.T) {
	s, _ := withMob()
	s.pendingBlows = []pendingBlow{{blow: blowAt(7, 12), remainingMs: 500}}

	s.forgetPendingBlows()

	if len(s.pendingBlows) != 0 {
		t.Errorf("%d blows survived the map change", len(s.pendingBlows))
	}

	s.advancePendingBlows(1000)
	if len(s.damageNumbers) != 0 {
		t.Error("a forgotten blow still landed")
	}
}

// TestAdvancePendingBlowsKeepsTheRest: one blow landing must not take its
// neighbors with it, which a slice rewritten in place gets wrong easily.
func TestAdvancePendingBlowsKeepsTheRest(t *testing.T) {
	s, _ := withMob()
	s.pendingBlows = []pendingBlow{
		{blow: blowAt(7, 5), remainingMs: 100},
		{blow: blowAt(7, 9), remainingMs: 900},
		{blow: blowAt(7, 3), remainingMs: 50},
	}

	s.advancePendingBlows(150)

	if len(s.pendingBlows) != 1 {
		t.Fatalf("%d blows left, want the one that has not landed", len(s.pendingBlows))
	}
	if s.pendingBlows[0].blow.Amount != 9 {
		t.Errorf("the wrong blow was kept: %d", s.pendingBlows[0].blow.Amount)
	}
	if len(s.damageNumbers) != 2 {
		t.Errorf("%d figures shown, want 2", len(s.damageNumbers))
	}
}

// TestHitDelayWithoutASheet: nothing baked yet means nothing on screen to
// wait for, and the blow lands as it always did rather than never.
func TestHitDelayWithoutASheet(t *testing.T) {
	var s InGameState

	if got := s.hitDelayMs(1, 1); got != 0 {
		t.Errorf("hitDelayMs with no renderer = %v, want 0", got)
	}
}

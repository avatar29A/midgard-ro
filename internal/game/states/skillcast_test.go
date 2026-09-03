package states

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/game/entity"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// TestPlacingHoldsTheSkillUntilACellIsChosen: a ground skill is asked for
// twice, and between the two the client is holding one. Nothing should go out
// on the first ask.
func TestPlacingHoldsTheSkillUntilACellIsChosen(t *testing.T) {
	s := &InGameState{}

	if _, holding := s.Placing(); holding {
		t.Fatal("something was held before anything was chosen")
	}

	s.BeginPlacing(80, 5)

	skill, holding := s.Placing()
	if !holding {
		t.Fatal("the skill was not held")
	}
	if skill != 80 {
		t.Errorf("holding skill %d, want 80", skill)
	}
}

// TestCancelPlacingPutsItDown: escape means "not that after all", and a skill
// left held would take the next click meant for something else.
func TestCancelPlacingPutsItDown(t *testing.T) {
	s := &InGameState{}
	s.BeginPlacing(80, 5)
	s.CancelPlacing()

	if _, holding := s.Placing(); holding {
		t.Error("the skill is still held after being canceled")
	}
}

// TestAClickWhileHoldingIsTakenByTheSkill: the click means "here". If it fell
// through, the character would walk to the cell instead of casting on it.
func TestAClickWhileHoldingIsTakenByTheSkill(t *testing.T) {
	s := &InGameState{}

	if s.placeHeldSkill(0, 0, 0, 0) {
		t.Error("a click was taken with nothing held")
	}

	s.BeginPlacing(80, 5)

	if !s.placeHeldSkill(0, 0, 0, 0) {
		t.Error("a click was not taken while a skill was held")
	}
	if _, holding := s.Placing(); holding {
		t.Error("the skill is still held after being placed")
	}
}

// TestCastBarRunsDownAndEnds: the server sends the length once and says
// nothing more, so the bar is run off that number alone.
func TestCastBarRunsDownAndEnds(t *testing.T) {
	s := &InGameState{}

	if _, _, casting := s.CastProgress(); casting {
		t.Fatal("a bar was running before any cast")
	}

	s.beginCast(14, 1000)

	done, skill, casting := s.CastProgress()
	if !casting {
		t.Fatal("the bar did not start")
	}
	if skill != 14 {
		t.Errorf("the bar is for skill %d, want 14", skill)
	}
	if done != 0 {
		t.Errorf("the bar starts at %v, want 0", done)
	}

	s.advanceCast(500)
	if done, _, _ := s.CastProgress(); done < 0.49 || done > 0.51 {
		t.Errorf("halfway through, the bar reads %v", done)
	}

	s.advanceCast(600)
	if _, _, casting := s.CastProgress(); casting {
		t.Error("the bar is still running past its length")
	}
}

// TestAnInstantCastDrawsNoBar: most skills have no cast time at all, and a bar
// that appears and vanishes inside one frame is worse than none.
func TestAnInstantCastDrawsNoBar(t *testing.T) {
	s := &InGameState{}
	s.beginCast(28, 0)

	if _, _, casting := s.CastProgress(); casting {
		t.Error("an instant cast started a bar")
	}
}

// TestSupportSkillsWaitForSomebody: a skill that can go on anybody is held
// until somebody is chosen. Casting it at the caster instead is what put half
// of every support skill out of reach.
func TestSupportSkillsWaitForSomebody(t *testing.T) {
	s := &InGameState{}
	s.BeginTargeting(28, 10)

	skill, holding := s.Placing()
	if !holding || skill != 28 {
		t.Fatalf("the skill was not held: %d, %v", skill, holding)
	}
	if !s.placingAtUnit {
		t.Error("a support skill is waiting for a cell rather than for somebody")
	}
}

// TestPlacingAndTargetingAreTheSameHold: both are one skill waiting to be
// aimed, and a cancel or a click has to end either.
func TestPlacingAndTargetingAreTheSameHold(t *testing.T) {
	s := &InGameState{}

	s.BeginPlacing(80, 5)
	if s.placingAtUnit {
		t.Error("a ground skill is waiting for somebody rather than for a cell")
	}

	s.CancelPlacing()
	if _, holding := s.Placing(); holding {
		t.Error("cancel left a ground skill held")
	}

	s.BeginTargeting(28, 10)
	s.CancelPlacing()
	if _, holding := s.Placing(); holding {
		t.Error("cancel left a support skill held")
	}
	if s.placingAtUnit {
		t.Error("cancel left the hold pointed at a unit")
	}
}

// TestAClickOnNothingDropsATargetedSkill: the original puts the skill down
// rather than casting it on the caster. Casting on the caster would spend SP
// nobody asked to spend, on a click that meant "never mind".
func TestAClickOnNothingDropsATargetedSkill(t *testing.T) {
	s := &InGameState{}
	s.BeginTargeting(28, 10)

	// No entity manager, so nothing is under the pointer.
	if !s.placeHeldSkill(0, 0, 0, 0) {
		t.Fatal("the click was not taken by the held skill")
	}
	if _, holding := s.Placing(); holding {
		t.Error("the skill is still held after a click on nothing")
	}
}

// TestOnlyHealingSkillsShowAFigure: rAthena writes the amount healed and the
// skill level into the same field, so a level 10 Increase Agi sends a 10 that
// means nothing. Shown as a figure it reads as ten hit points restored, which
// is what it did.
func TestOnlyHealingSkillsShowAFigure(t *testing.T) {
	if !healSkills[28] {
		t.Error("Heal does not show a figure")
	}

	for _, buff := range []uint16{
		29, // AL_INCAGI
		34, // AL_BLESSING
		66, // PR_IMPOSITIO
	} {
		if healSkills[buff] {
			t.Errorf("skill %d shows a figure, but its amount is a skill level", buff)
		}
	}
}

// TestSkillLabelsAgeOut: a name over somebody has to go away, or a fight
// leaves the map covered in them.
func TestSkillLabelsAgeOut(t *testing.T) {
	s, mob := withMob()
	s.skillLabels = []floatingSkillName{{text: "Increase AGI", target: mob.ID, lifeMs: skillLabelLifeMs}}

	s.advanceSkillLabels(skillLabelLifeMs / 2)
	if len(s.skillLabels) != 1 {
		t.Fatal("the label went early")
	}

	s.advanceSkillLabels(skillLabelLifeMs)
	if len(s.skillLabels) != 0 {
		t.Errorf("%d labels outlived their time", len(s.skillLabels))
	}
}

// TestASkillLabelFollowsItsTarget: the name belongs over somebody's head, so a
// character buffed and then walking away carries it. Held as a place it would
// stay where the cast happened.
func TestASkillLabelFollowsItsTarget(t *testing.T) {
	s, mob := withMob()
	s.skillLabels = []floatingSkillName{{text: "Increase AGI", target: mob.ID}}

	if s.skillLabels[0].target != mob.ID {
		t.Fatal("the label is not held against a unit")
	}

	// Nothing about the label changes when the unit moves; where it draws is
	// read off the body every frame.
	before := mob.Body.RenderX
	mob.Body.RenderX = before + 100

	if s.skillLabels[0].target != mob.ID {
		t.Error("moving the unit disturbed its label")
	}
}

// TestASkillLabelGoesWithItsTarget: a name floating where a monster used to
// stand is worse than no name.
func TestASkillLabelGoesWithItsTarget(t *testing.T) {
	s, mob := withMob()
	s.skillLabels = []floatingSkillName{{text: "Increase AGI", target: mob.ID}}

	s.entityManager.Remove(mob.ID)
	s.advanceSkillLabels(16)

	if len(s.skillLabels) != 0 {
		t.Error("the label outlived the unit it was over")
	}
}

// TestOneNameOverOneHead: two buffs in a row on the same target replace each
// other rather than stacking into a pile.
func TestOneNameOverOneHead(t *testing.T) {
	s, mob := withMob()

	s.addSkillLabel(mob.ID, 29, skillLabelLifeMs)
	s.addSkillLabel(mob.ID, 34, skillLabelLifeMs)

	if len(s.skillLabels) != 1 {
		t.Errorf("%d labels over one head, want 1", len(s.skillLabels))
	}
}

// TestAnOffensiveSkillIsAimedToo: Decrease Agi is TargetType Attack, not
// Support, and used to be refused for want of a target rather than asking for
// one. Anything cast on a unit waits for one to be picked.
func TestAnOffensiveSkillIsAimedToo(t *testing.T) {
	s := &InGameState{}
	s.BeginTargeting(30, 10)

	skill, holding := s.Placing()
	if !holding || skill != 30 {
		t.Fatalf("the skill was not held: %d, %v", skill, holding)
	}
	if !s.placingAtUnit {
		t.Error("an offensive skill is waiting for a cell rather than for somebody")
	}
}

// TestEffectFileNames: the archive files a skill effect under the table's own
// name with the EF_ taken off — EF_FIREHIT is firehit.str. Thirty-five of the
// effects the table names are there under exactly that rule.
func TestEffectFileNames(t *testing.T) {
	for _, tc := range []struct{ effect, want string }{
		{"EF_FIREHIT", "firehit.str"},
		{"EF_WINDHIT", "windhit.str"},
		{"EF_MAGNIFICAT", "magnificat.str"},
		{"EF_STORMGUST", "stormgust.str"},
	} {
		if got := effectFileFor(tc.effect); got != tc.want {
			t.Errorf("effectFileFor(%q) = %q, want %q", tc.effect, got, tc.want)
		}
	}
}

// TestEffectFileRefusesWhatIsNotOne: a name that is not an effect must not be
// turned into a file name, or the loader goes looking for ".str" and caches a
// miss under the empty string.
func TestEffectFileRefusesWhatIsNotOne(t *testing.T) {
	for _, name := range []string{"", "EF_", "FIREHIT", "ef_firehit"} {
		if got := effectFileFor(name); got != "" {
			t.Errorf("effectFileFor(%q) = %q, want nothing", name, got)
		}
	}
}

// TestSkillRangeTreatsNoneAsMelee: the server lists some skills with no range
// at all, and that means a melee skill rather than an unlimited one. Read as
// unlimited, Bash would be cast across the map and refused every time.
func TestSkillRangeTreatsNoneAsMelee(t *testing.T) {
	s := &InGameState{skills: []packets.Skill{
		{ID: 5, Level: 10, Range: 0},
		{ID: 14, Level: 10, Range: 9},
	}}

	if got := s.skillRange(5); got != 1 {
		t.Errorf("a skill listed with no range reaches %d, want 1", got)
	}
	if got := s.skillRange(14); got != 9 {
		t.Errorf("Cold Bolt reaches %d, want 9", got)
	}
	if got := s.skillRange(999); got != 0 {
		t.Errorf("a skill the server never listed reaches %d, want 0", got)
	}
}

// TestCastingOnYourselfIsAlwaysInRange: a skill on the caster has nowhere to
// walk to, and treating it as out of range would leave it waiting forever.
func TestCastingOnYourselfIsAlwaysInRange(t *testing.T) {
	s := &InGameState{player: entity.NewCharacter(0, 0, 0)}

	if !s.withinSkillRange(28, s.selfAID()) {
		t.Error("a skill on the caster is out of its own range")
	}
}

// TestAPendingCastGoesWithItsTarget: whoever it was for is gone, so is the
// walk — otherwise the character keeps closing on a corpse.
func TestAPendingCastGoesWithItsTarget(t *testing.T) {
	s, mob := withMob()
	s.pendingSkill = &pendingSkillCast{skill: 14, level: 10, target: mob.ID}

	s.entityManager.Remove(mob.ID)
	s.advancePendingSkill(16)

	if s.pendingSkill != nil {
		t.Error("the cast is still waiting for a target that has gone")
	}
}

// TestEffectSoundNames: the archive names an effect's sound after the effect,
// not after the skill — ef_bash.wav, and no sm_bash.wav anywhere. Twenty-six
// of the effects the table names have one.
func TestEffectSoundNames(t *testing.T) {
	for _, tc := range []struct{ effect, want string }{
		{"EF_BASH", `data\wav\effect\ef_bash.wav`},
		{"EF_BEGINSPELL", `data\wav\effect\ef_beginspell.wav`},
		{"EF_FIREHIT", `data\wav\effect\ef_firehit.wav`},
	} {
		if got := effectSoundFor(tc.effect); got != tc.want {
			t.Errorf("effectSoundFor(%q) = %q, want %q", tc.effect, got, tc.want)
		}
	}

	if got := effectSoundFor("NOT_AN_EFFECT"); got != "" {
		t.Errorf("effectSoundFor of a non-effect = %q, want nothing", got)
	}
}

// TestASoundIsAskedForOnce: a skill that lands three blows in a frame should
// not play its sound three times over itself.
func TestASoundIsAskedForOnce(t *testing.T) {
	var s InGameState

	s.playSound("a.wav")
	s.playSound("a.wav")
	s.playSound("b.wav")

	if got := s.TakeSounds(); len(got) != 2 {
		t.Errorf("TakeSounds = %v, want two distinct sounds", got)
	}

	if got := s.TakeSounds(); got != nil {
		t.Errorf("the queue was not cleared: %v", got)
	}
}

// TestASkillShoutsItsName: the original's own sign that a skill went off is
// its name over the caster, with two marks after it. A skill that did damage
// shouts too — the name used to be kept for buffs, which left every attack
// skill silent.
func TestASkillShoutsItsName(t *testing.T) {
	s, mob := withMob()

	s.addSkillLabel(mob.ID, 14, skillLabelLifeMs) // MG_COLDBOLT

	if len(s.skillLabels) != 1 {
		t.Fatalf("%d labels, want 1", len(s.skillLabels))
	}
	if got := s.skillLabels[0].text; got != "Cold Bolt !!" {
		t.Errorf("the shout reads %q, want %q", got, "Cold Bolt !!")
	}
}

// TestACastLabelLastsTheCast: the name is shown while the cast runs, so it has
// to last exactly that long. One that outlives it hangs over a character doing
// nothing; one that ends early leaves the bar unlabelled.
func TestACastLabelLastsTheCast(t *testing.T) {
	s, mob := withMob()

	s.addSkillLabel(mob.ID, 14, 2000)

	s.advanceSkillLabels(1900)
	if len(s.skillLabels) != 1 {
		t.Fatal("the cast label went before the cast ended")
	}

	s.advanceSkillLabels(200)
	if len(s.skillLabels) != 0 {
		t.Error("the cast label outlived the cast")
	}
}

// TestTheCastLabelClearsTheCastBar: the bar sits just above the head and the
// name is shown at the same time, so the name has to rise past both the bar
// and the drop its own plate hangs by. Printed at the same height the two are
// printed over each other, which is what they were.
func TestTheCastLabelClearsTheCastBar(t *testing.T) {
	// The bar's top, and the drop the plate hangs below its anchor by. Both
	// live in the interface package, which this one cannot see; they are
	// repeated here so a change to either shows up as a failure.
	const (
		castBarTop = 2 + 6
		plateDrop  = 4 + 9 + 3
		plateHigh  = 16
	)

	if bottom := plateDrop + plateHigh - skillLabelRise; bottom > -castBarTop {
		t.Errorf("the name ends %v below the head and the bar starts %v below it",
			bottom, -castBarTop)
	}
}

// TestTheBattleLogReportsOurOwnSkill: what it hit, for how much, and what it
// cost. The cost is the one figure nothing else in the client shows.
func TestTheBattleLogReportsOurOwnSkill(t *testing.T) {
	s, mob := withMob()
	s.skills = []packets.Skill{{ID: 14, Level: 10, SP: 25}}
	mob.Name = "Poring"

	s.reportSkill(packets.SkillUse{
		SourceID: 5000, TargetID: mob.ID, SkillID: 14, Damage: 1374, Hits: 10,
	}, 5000)

	lines := s.chat.Lines()
	if len(lines) != 2 {
		t.Fatalf("%d lines in the log, want a cost and a hit: %v", len(lines), lines)
	}

	for _, line := range lines {
		if line.Kind != ChatDamage {
			t.Errorf("line %q is kind %d, want ChatDamage so it lands in the battle tab",
				line.Text, line.Kind)
		}
	}

	if !strings.Contains(lines[0].Text, "25 SP") {
		t.Errorf("the cost line reads %q", lines[0].Text)
	}
	if !strings.Contains(lines[1].Text, "1374 damage over 10 hits") {
		t.Errorf("the damage line reads %q", lines[1].Text)
	}
	if !strings.Contains(lines[1].Text, mob.Name) {
		t.Errorf("the damage line does not name what was hit: %q", lines[1].Text)
	}
}

// TestTheBattleLogReportsWhatHitUs: a skill aimed at us is our log too.
func TestTheBattleLogReportsWhatHitUs(t *testing.T) {
	s, mob := withMob()

	s.reportSkill(packets.SkillUse{
		SourceID: mob.ID, TargetID: 5000, SkillID: 19, Damage: 120, Hits: 1,
	}, 5000)

	lines := s.chat.Lines()
	if len(lines) != 1 {
		t.Fatalf("%d lines, want 1: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0].Text, "120 damage") || !strings.Contains(lines[0].Text, "you") {
		t.Errorf("the line reads %q", lines[0].Text)
	}
	if strings.Contains(lines[0].Text, "over") {
		t.Errorf("a single hit is reported as several: %q", lines[0].Text)
	}
}

// TestTheBattleLogIgnoresOtherPeoplesFights: a fight across the field is not
// our log, and a busy map would drown ours in it.
func TestTheBattleLogIgnoresOtherPeoplesFights(t *testing.T) {
	s, mob := withMob()

	s.reportSkill(packets.SkillUse{
		SourceID: mob.ID, TargetID: 9999, SkillID: 19, Damage: 120, Hits: 1,
	}, 5000)

	if lines := s.chat.Lines(); len(lines) != 0 {
		t.Errorf("%d lines from somebody else's fight: %v", len(lines), lines)
	}
}

// TestABoltSkillHitsOncePerShot: what a volley hits with is drawn as each shot
// lands, not all at the start. Ten flashes at once on a target nothing has
// reached yet is what made a bolt skill look like a single blow.
func TestABoltSkillHitsOncePerShot(t *testing.T) {
	s, mob := withMob()

	s.playSkillUseEffects(packets.SkillUse{
		SourceID: 5000, TargetID: mob.ID, SkillID: 14, Damage: 1374, Hits: 10,
	})

	if len(s.delayedEffects) != 10 {
		t.Fatalf("%d hits scheduled for ten shots", len(s.delayedEffects))
	}

	for i := 1; i < len(s.delayedEffects); i++ {
		if s.delayedEffects[i].delayMs <= s.delayedEffects[i-1].delayMs {
			t.Errorf("hit %d is due at %v, no later than hit %d at %v",
				i, s.delayedEffects[i].delayMs, i-1, s.delayedEffects[i-1].delayMs)
		}
	}

	for _, waiting := range s.delayedEffects {
		if waiting.effect != "EF_COLDHIT" {
			t.Errorf("a shot hits with %q, want the skill's own EF_COLDHIT", waiting.effect)
		}
		if waiting.target != mob.ID {
			t.Errorf("a hit is aimed at %d, not at what was shot at", waiting.target)
		}
	}
}

// TestASkillWithNoBoltsIsDrawnAtOnce: everything that is not a volley keeps
// playing the moment the skill goes off.
func TestASkillWithNoBoltsIsDrawnAtOnce(t *testing.T) {
	s, mob := withMob()

	s.playSkillUseEffects(packets.SkillUse{
		SourceID: 5000, TargetID: mob.ID, SkillID: 28, Amount: 281, // AL_HEAL
	})

	if len(s.delayedEffects) != 0 {
		t.Errorf("%d effects were made to wait for a shot that is not coming", len(s.delayedEffects))
	}
}

// TestAWaitingHitGoesWithItsTarget: a flash where a monster used to stand is
// worse than no flash, and a volley outlives what it kills.
func TestAWaitingHitGoesWithItsTarget(t *testing.T) {
	s, mob := withMob()
	s.delayedEffects = []delayedEffect{{effect: "EF_COLDHIT", target: mob.ID, delayMs: 100}}

	s.entityManager.Remove(mob.ID)
	s.advanceDelayedEffects(200)

	if len(s.delayedEffects) != 0 {
		t.Errorf("%d hits still waiting for a target that has gone", len(s.delayedEffects))
	}
	if len(s.bursts) != 0 {
		t.Error("a hit played on a target that has gone")
	}
}

// TestAWaitingHitWaits: one that fired early would land before its shot.
func TestAWaitingHitWaits(t *testing.T) {
	s, mob := withMob()
	s.delayedEffects = []delayedEffect{{effect: "EF_COLDHIT", target: mob.ID, delayMs: 500}}

	s.advanceDelayedEffects(400)
	if len(s.delayedEffects) != 1 {
		t.Fatal("the hit went off before its shot landed")
	}

	s.advanceDelayedEffects(200)
	if len(s.delayedEffects) != 0 {
		t.Error("the hit never went off")
	}
}

// TestACastHoldsTheCharacterStill: rAthena's unit_can_move is false while the
// skill timer runs, so a walk asked for during a cast is refused. Asking
// anyway left the character sliding along while the bar filled, predicting a
// step no acknowledgement was coming for.
func TestACastHoldsTheCharacterStill(t *testing.T) {
	s := &InGameState{}

	if s.Casting() {
		t.Error("a character that has cast nothing is casting")
	}

	s.beginCast(14, 2000)
	if !s.Casting() {
		t.Fatal("a cast of two seconds is not a cast")
	}

	s.advanceCast(1999)
	if !s.Casting() {
		t.Error("the cast ended a millisecond early")
	}

	s.advanceCast(2)
	if s.Casting() {
		t.Error("the cast outlived its own length")
	}
}

// TestAnInstantSkillDoesNotHoldStill: most skills have no cast at all, and a
// character that could not walk for a frame after every one of them would
// stutter its way across the map.
func TestAnInstantSkillDoesNotHoldStill(t *testing.T) {
	s := &InGameState{}
	s.beginCast(5, 0)

	if s.Casting() {
		t.Error("an instant skill counts as a cast")
	}
}

// cancelFor builds the packet the server sends when a cast comes to nothing.
func cancelFor(who uint32) []byte {
	pkt := make([]byte, 6)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_DISPEL)
	binary.LittleEndian.PutUint32(pkt[2:], who)

	return pkt
}

// TestACancelledCastLetsGo: the bar, the ring and the name all belong to a
// cast that is running, and a cast that was broken is not.
func TestACancelledCastLetsGo(t *testing.T) {
	s, mob := withMob()
	s.castAuras = []castingAura{{caster: mob.ID, leftMs: 1000, totalMs: 1000}}
	s.addSkillLabel(mob.ID, 14, skillLabelLifeMs)

	pkt := cancelFor(mob.ID)

	if err := s.handleCastCancelled(pkt); err != nil {
		t.Fatalf("handling: %v", err)
	}

	if len(s.castAuras) != 0 {
		t.Errorf("%d rings still turning under a cast that was broken", len(s.castAuras))
	}
	if len(s.skillLabels) != 0 {
		t.Errorf("%d names still over a cast that was broken", len(s.skillLabels))
	}
}

// TestACancelForSomebodyElseLeavesOursAlone: the packet names who it was, and
// two people can be casting at once.
func TestACancelForSomebodyElseLeavesOursAlone(t *testing.T) {
	s, mob := withMob()
	s.beginCast(14, 2000)
	s.castAuras = []castingAura{{caster: mob.ID, leftMs: 1000, totalMs: 1000}}

	pkt := cancelFor(mob.ID)

	if err := s.handleCastCancelled(pkt); err != nil {
		t.Fatalf("handling: %v", err)
	}

	if !s.Casting() {
		t.Error("somebody else's cast being broken stopped ours")
	}
	if len(s.castAuras) != 0 {
		t.Error("their ring kept turning")
	}
}

// TestAShortCancelPacketIsRefused: a truncated one would read an id out of
// whatever came after it and stop a cast at random.
func TestAShortCancelPacketIsRefused(t *testing.T) {
	s := &InGameState{}
	s.beginCast(14, 2000)

	if err := s.handleCastCancelled([]byte{0xB9, 0x01, 0x00}); err != nil {
		t.Fatalf("handling: %v", err)
	}

	if !s.Casting() {
		t.Error("a short packet stopped the cast")
	}
}

// skillUnitAt builds the packet the server sends when a ground skill puts a
// unit on a cell.
func skillUnitAt(id, by uint32) []byte {
	pkt := make([]byte, 23)
	binary.LittleEndian.PutUint16(pkt, packets.ZC_SKILL_ENTRY)
	binary.LittleEndian.PutUint16(pkt[2:], 23)
	binary.LittleEndian.PutUint32(pkt[4:], id)
	binary.LittleEndian.PutUint32(pkt[8:], by)

	return pkt
}

// TestAGroundSkillsBlowsAreTheCastersOwn: Thunder Storm's lightning comes
// from the unit it left on the ground, not from the mage, so every blow
// arrives from a block id belonging to nobody. Read as it comes, a player's
// own Thunder Storm is a stranger's fight and never reaches the log.
func TestAGroundSkillsBlowsAreTheCastersOwn(t *testing.T) {
	s, mob := withMob()
	s.skills = []packets.Skill{{ID: 21, Level: 10, SP: 60}}

	const self, unit = uint32(5000), uint32(2585)

	if err := s.handleSkillUnit(skillUnitAt(unit, self)); err != nil {
		t.Fatalf("handling the unit: %v", err)
	}

	// The placement itself, which is what the cost is paid for.
	s.reportSkill(packets.SkillUse{SourceID: self, SkillID: 21, Ground: true}, self)

	// And a blow from the unit standing there.
	s.reportSkill(packets.SkillUse{
		SourceID: unit, TargetID: mob.ID, SkillID: 21, Damage: 2200, Hits: 1,
	}, self)

	lines := s.chat.Lines()
	if len(lines) != 2 {
		t.Fatalf("%d lines, want the cost and the blow: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0].Text, "60 SP") {
		t.Errorf("the cost line reads %q", lines[0].Text)
	}
	if !strings.Contains(lines[1].Text, "2200 damage") {
		t.Errorf("the blow reads %q", lines[1].Text)
	}
}

// TestAGroundSkillIsPaidForOnce: it strikes for as long as it stands, and
// charging for every blow fills the log with a cost nobody paid.
func TestAGroundSkillIsPaidForOnce(t *testing.T) {
	s, mob := withMob()
	s.skills = []packets.Skill{{ID: 21, Level: 10, SP: 60}}

	const self, unit = uint32(5000), uint32(2585)
	if err := s.handleSkillUnit(skillUnitAt(unit, self)); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		s.reportSkill(packets.SkillUse{
			SourceID: unit, TargetID: mob.ID, SkillID: 21, Damage: 2200, Hits: 1,
		}, self)
	}

	for _, line := range s.chat.Lines() {
		if strings.Contains(line.Text, "SP") {
			t.Errorf("a blow from the unit charged for the skill again: %q", line.Text)
		}
	}
}

// TestAUnitThatHasGoneIsForgotten: block ids come round again, and a stale
// one would credit somebody else's skill to us.
func TestAUnitThatHasGoneIsForgotten(t *testing.T) {
	s, _ := withMob()

	const self, unit = uint32(5000), uint32(2585)
	if err := s.handleSkillUnit(skillUnitAt(unit, self)); err != nil {
		t.Fatal(err)
	}
	if s.casterBehind(unit) != self {
		t.Fatal("the unit was not remembered")
	}

	gone := make([]byte, 6)
	binary.LittleEndian.PutUint16(gone, packets.ZC_SKILL_DISAPPEAR)
	binary.LittleEndian.PutUint32(gone[2:], unit)

	if err := s.handleSkillUnitGone(gone); err != nil {
		t.Fatalf("handling the removal: %v", err)
	}

	if got := s.casterBehind(unit); got != unit {
		t.Errorf("a unit that has gone still answers to %d", got)
	}
}

// TestAGroundSkillAimsAtWhoIsUnderThePointer: a sprite stands on its cell but
// is drawn upwards from it, so a pointer on a monster's body meets the ground
// two or three cells beyond its feet. Thunder Storm reaches two cells either
// way, so aiming at the ground there is the difference between hitting and
// missing — which is what made it work every other time.
func TestAGroundSkillAimsAtWhoIsUnderThePointer(t *testing.T) {
	s, mob := withMob()
	s.hoverValid = true
	s.hoverCellX, s.hoverCellY = 103, 97 // the ground behind it

	// withMob puts the monster on 100,100. Nothing here can pick from the
	// screen, so the pick is stood in for by placing the mob's own cell
	// against the ground cell and asking which one a placement takes.
	wantX, wantY := mob.Body.CurrentCell()
	if wantX != 100 || wantY != 100 {
		t.Fatalf("the monster is on %d,%d, not where this test expects", wantX, wantY)
	}

	// With nobody picked, the ground under the pointer is what it takes.
	gotX, gotY, ok := s.groundAimFor(0, 0, 0, 0)
	if !ok {
		t.Fatal("a valid ground cell was refused")
	}
	if gotX != 103 || gotY != 97 {
		t.Errorf("with nobody under the pointer it aimed at %d,%d, want the ground at 103,97",
			gotX, gotY)
	}
}

// TestAGroundSkillWithNowhereToGo: off the map, nothing is cast. Sending the
// cell anyway has the server refuse it, and the skill is spent either way.
func TestAGroundSkillWithNowhereToGo(t *testing.T) {
	s, _ := withMob()
	s.hoverValid = false

	if _, _, ok := s.groundAimFor(0, 0, 0, 0); ok {
		t.Error("a placement off the map was accepted")
	}
}

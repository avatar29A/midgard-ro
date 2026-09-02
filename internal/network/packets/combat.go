package packets

import "encoding/binary"

// Combat packets: asking to hit something, and being told what a hit did.
const (
	// CZ_REQUEST_ACT asks to act on a target: `<GID>.L <action>.B`, 7 bytes,
	// with the target at offset 2 and the action at 6.
	//
	// 0x0437 comes from clif_shuffle.hpp, which settles it at this packetver.
	// The main packet table registers clif_parse_ActionRequest under a dozen
	// other ids — 0x0089, 0x0085, 0x0190, 0x009F among them — and the shuffle
	// file is included after it, so those are what the server used to think
	// before this block replaced them.
	CZ_REQUEST_ACT uint16 = 0x0437

	// ZC_NOTIFY_ACT, the result of a blow, is declared in packets.go with the
	// rest of the unit notifications. It is 34 bytes at this packetver.

	// ZC_ATTACK_RANGE is how far the character can reach: `<range>.W`, 4
	// bytes, sent whenever the weapon changes.
	//
	// The server's own figure, and the one battle_check_range measures a blow
	// against. Guessing it as one cell put the character on top of everything
	// it fought and would have been wrong the moment a bow was equipped.
	ZC_ATTACK_RANGE uint16 = 0x013A

	// ZC_MONSTER_HP_INFO carries a monster's health for the bar over its
	// head: `<GID>.L <hp>.L <maxHP>.L`, 14 bytes.
	//
	// The server's own figure, sent whenever a monster is hurt, so nothing
	// has to be deducted from the damage packets — which would drift, since
	// blows the client never saw still change the total.
	//
	// Sent only while monster_hp_bars_info is on, which it is on the server
	// we run. With it off no bar moves, and that is the server's choice
	// rather than a fault here.
	ZC_MONSTER_HP_INFO uint16 = 0x0977
)

// Action codes for CZ_REQUEST_ACT, from rAthena's e_damage_type.
const (
	// ActionAttackOnce swings once. ActionAttackRepeat keeps swinging until
	// something stops it, which is what a click on a monster means in the
	// original.
	ActionAttackOnce   uint8 = 0
	ActionAttackRepeat uint8 = 7

	ActionSit   uint8 = 2
	ActionStand uint8 = 3
)

// Types worth telling apart in ZC_NOTIFY_ACT.
//
// The packet is not only about damage. rAthena sends it for a handful of
// things a unit does that everyone nearby should see — picking an item up,
// sitting, standing — with the same shape and a type that says which. Reading
// them all as blows makes a character swing its weapon at the ground it just
// picked a potato off.
const (
	DamageNormal     uint8 = 0
	ActPickupItem    uint8 = 1
	ActSitDown       uint8 = 2
	ActStandUp       uint8 = 3
	DamageEndure     uint8 = 4
	DamageMultiHit   uint8 = 8
	DamageCritical   uint8 = 10
	DamageLuckyDodge uint8 = 11
)

// Why a unit left, as DecodeEntityVanish reports it. Dying is the only one
// worth an animation rather than simply removing the unit.
const (
	VanishOutOfSight uint8 = 0
	VanishDied       uint8 = 1
	VanishLoggedOut  uint8 = 2
	VanishTeleported uint8 = 3
	VanishTrickDead  uint8 = 4
)

// EncodeAttack asks to hit a target.
//
// Repeating rather than swinging once is what a click on a monster does in
// the original: the character keeps attacking until the target dies or
// something else is asked of it. Stopping is not a packet — walking somewhere
// is, and the server takes that as the cancellation.
func EncodeAttack(targetGID uint32, repeat bool) []byte {
	action := ActionAttackOnce
	if repeat {
		action = ActionAttackRepeat
	}

	return EncodeAction(targetGID, action)
}

// EncodeAction asks to act on a target.
//
// The same packet an attack goes out in. Sitting and standing name the
// character itself as the target, which is how rAthena reads them: the id is
// ignored for those two, but sending a zero would be sending a target the
// server has no reason to accept.
func EncodeAction(targetGID uint32, action uint8) []byte {
	pkt := make([]byte, 7)
	binary.LittleEndian.PutUint16(pkt, CZ_REQUEST_ACT)
	binary.LittleEndian.PutUint32(pkt[2:], targetGID)
	pkt[6] = action

	return pkt
}

// Damage is one blow, as the server reports it.
type Damage struct {
	// SourceID struck TargetID. Either can be us or anything else on screen —
	// the packet goes to everyone who can see the fight, so a blow between two
	// other units arrives here too.
	SourceID uint32
	TargetID uint32

	// Amount is the total across every hit in this blow, not per hit. Zero is
	// a miss, which is worth drawing as one.
	Amount int

	// Amount2 is the off-hand total for a dual-wielding attacker.
	Amount2 int

	// Hits is how many blows the total is spread over.
	Hits int

	// Type says what kind of blow it was — a critical and a lucky dodge are
	// drawn differently from an ordinary hit.
	Type uint8

	// SourceSpeed is the attacker's attack motion in milliseconds, which is
	// how long its swing animation should take. DamageDelay is how long
	// before the target reacts.
	SourceSpeed int
	DamageDelay int

	// IsSPDamage marks a blow against SP rather than HP.
	IsSPDamage bool
}

// SwingReferenceMs is how long a swing takes at the rate the sprite was drawn
// for, and the longest one ever takes.
//
// rAthena documents what the original client does with the attack motion, and
// it is not what the field's name suggests: sdelay carries the attacker's
// amotion and the client reads it as an inverted animation speed, with 432
// standing for the sprite's own rate. "216 for example means play the
// animation at double the speed. 108 is quadruple speed", and anything above
// 432 is ignored — which is why the server clamps to it before sending.
//
// A speed measured against a reference is a duration measured against that
// reference's own length, and this is that length: at 432 the swing takes
// 432ms, at 216 half of it, at 108 a quarter. Which makes the whole thing say
// something simple — a swing lasts as long as the attack motion it belongs to.
const SwingReferenceMs = 432

// SwingDurationMs is how long the attacker's whole swing should take.
//
// The attack motion, which is what ties the animation to attack speed: at ASPD
// 168 the server's amotion is 320ms, so the swing runs in 320ms and the next
// one begins as it ends. Drawn at the sprite's own rate instead, a Swordman's
// nine-frame sword swing takes 900ms — three swings deep before the first has
// finished, so it never completes, never reaches the frame the blade lands on,
// and looks nothing like the speed it is being swung at.
//
// Capped at the reference, since the client ignores anything slower, and the
// reference when the server said nothing.
func (d Damage) SwingDurationMs() float32 {
	if d.SourceSpeed <= 0 || d.SourceSpeed > SwingReferenceMs {
		return SwingReferenceMs
	}

	return float32(d.SourceSpeed)
}

// Missed reports whether the blow did nothing, which the original draws as a
// miss rather than as a zero.
func (d Damage) Missed() bool {
	return d.Amount == 0 && !d.IsSPDamage
}

// IsBlow reports whether this is a hit rather than one of the gestures the
// same packet carries.
func (d Damage) IsBlow() bool {
	switch d.Type {
	case ActPickupItem, ActSitDown, ActStandUp:
		return false
	default:
		return true
	}
}

// Critical reports whether to draw the blow as a critical hit.
func (d Damage) Critical() bool {
	return d.Type == DamageCritical
}

// DecodeDamage reads ZC_NOTIFY_ACT.
func DecodeDamage(data []byte) (Damage, bool) {
	if len(data) < 34 {
		return Damage{}, false
	}

	return Damage{
		SourceID:    binary.LittleEndian.Uint32(data[2:]),
		TargetID:    binary.LittleEndian.Uint32(data[6:]),
		SourceSpeed: int(int32(binary.LittleEndian.Uint32(data[14:]))),
		DamageDelay: int(int32(binary.LittleEndian.Uint32(data[18:]))),
		Amount:      int(int32(binary.LittleEndian.Uint32(data[22:]))),
		IsSPDamage:  data[26] != 0,
		Hits:        int(binary.LittleEndian.Uint16(data[27:])),
		Type:        data[29],
		Amount2:     int(int32(binary.LittleEndian.Uint32(data[30:]))),
	}, true
}

// DecodeMonsterHP reads ZC_MONSTER_HP_INFO.
func DecodeMonsterHP(data []byte) (id uint32, hp, maxHP int, ok bool) {
	if len(data) < 14 {
		return 0, 0, 0, false
	}

	return binary.LittleEndian.Uint32(data[2:]),
		int(int32(binary.LittleEndian.Uint32(data[6:]))),
		int(int32(binary.LittleEndian.Uint32(data[10:]))),
		true
}

// DecodeAttackRange reads ZC_ATTACK_RANGE, the reach of the equipped weapon
// in cells.
func DecodeAttackRange(data []byte) (int, bool) {
	if len(data) < 4 {
		return 0, false
	}

	return int(int16(binary.LittleEndian.Uint16(data[2:]))), true
}

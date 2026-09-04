package packets

import "encoding/binary"

// A unit's body state.
//
// The states a unit is drawn in rather than given an icon for — petrified,
// frozen, stunned, asleep. rAthena calls it opt1 on the server and bodyState
// on the wire, and only one of them holds at a time, which is why it is a
// value and not a set of bits the way the others are.
//
// It arrives twice over: once in the packet that brings a unit into view, so
// somebody already frozen is drawn frozen, and again in ZC_STATE_CHANGE
// whenever it changes.

// ZC_STATE_CHANGE is `<AID>.L <bodyState>.W <healthState>.W <effectState>.L
// <isPKModeON>.B`, fifteen bytes.
//
// 0x0229 rather than the 0x0119 the name is usually attached to: rAthena
// switched to this one at PACKETVER 7, and the difference is a four-byte
// effectState where the old one had two. Reading the old shape off this
// packet takes the top half of the option word for a PK flag.
const ZC_STATE_CHANGE uint16 = 0x0229

// ZC_DISPEL is `<gid>.L`, six bytes: the server saying a unit's cast came to
// nothing. Its name is from what else it is used for; the cast bar is the
// part this client cares about.
const ZC_DISPEL uint16 = 0x01B9

// Body states, rAthena's e_sc_opt1. Five is missing on purpose: Aegis uses it
// to mark an undead enemy rather than as a state of its own.
const (
	BodyNone      uint16 = 0
	BodyStone     uint16 = 1
	BodyFreeze    uint16 = 2
	BodyStun      uint16 = 3
	BodySleep     uint16 = 4
	BodyStoneWait uint16 = 6
	BodyBurning   uint16 = 7
	BodyImprison  uint16 = 8
)

// Option bits, rAthena's e_option. Only the ones that are drawn are named.
const (
	// OptionSight is Sight and Ruwach: the spell that lights up what is
	// hiding nearby. The client draws it as an aura around the caster rather
	// than being told to play an effect — there is no skill effect for Sight
	// at all, and the state is what says it is running.
	OptionSight uint32 = 0x00000001

	// OptionRuwach is the Acolyte's version of the same thing.
	OptionRuwach uint32 = 0x00002000
)

// Sighting reports whether the unit has Sight or Ruwach running.
func (s StateChange) Sighting() bool {
	return s.Effect&(OptionSight|OptionRuwach) != 0
}

// StateChange is a unit's states changing.
type StateChange struct {
	AID uint32

	// Body is the one drawn on the unit, and Health and Effect the two sets
	// of bits that are not: poison, curse and silence in one, riding, hiding
	// and the rest in the other.
	Body   uint16
	Health uint16
	Effect uint32

	PKMode bool
}

// Frozen reports whether the unit is sealed in ice.
func (s StateChange) Frozen() bool { return s.Body == BodyFreeze }

// DecodeStateChange reads one. Returns false on short data.
func DecodeStateChange(data []byte) (StateChange, bool) {
	const size = 15

	if len(data) < size {
		return StateChange{}, false
	}

	return StateChange{
		AID:    binary.LittleEndian.Uint32(data[2:]),
		Body:   binary.LittleEndian.Uint16(data[6:]),
		Health: binary.LittleEndian.Uint16(data[8:]),
		Effect: binary.LittleEndian.Uint32(data[10:]),
		PKMode: data[14] != 0,
	}, true
}

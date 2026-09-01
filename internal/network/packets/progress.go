package packets

import "encoding/binary"

// Spending the points a level brings.
const (
	// CZ_STATUS_CHANGE raises one stat by an amount:
	// `<statID>.W <amount>.B`, 5 bytes, with the id at offset 2 and the
	// amount at 4.
	//
	// The server decides whether it is affordable and refuses silently
	// otherwise, answering an accepted one with the stat's new value.
	CZ_STATUS_CHANGE uint16 = 0x00BB

	// CZ_UPGRADE_SKILLLEVEL spends a skill point: `<skillID>.W`, 4 bytes.
	CZ_UPGRADE_SKILLLEVEL uint16 = 0x0112

	// CZ_REQ_TAKEOFF_EQUIP takes off what is worn in a slot:
	// `<index>.W`, 4 bytes.
	CZ_REQ_TAKEOFF_EQUIP uint16 = 0x00AB

	// ZC_NOTIFY_EFFECT announces something worth showing:
	// `<AID>.L <effect>.L`, 10 bytes.
	//
	// Leveling is not its own packet. The server sends this for a base level,
	// a job level, a refine succeeding or failing and a pharmacy brew alike,
	// and the code says which.
	ZC_NOTIFY_EFFECT uint16 = 0x019B
)

// Effect codes for ZC_NOTIFY_EFFECT, from rAthena's e_notify_effect.
const (
	EffectBaseLevelUp uint32 = 0
	EffectJobLevelUp  uint32 = 1
	EffectRefineFail  uint32 = 2
	EffectRefineOK    uint32 = 3
	EffectGameOver    uint32 = 4
	EffectPharmacyOK  uint32 = 5
)

// NotifyEffect is something the server wants shown.
type NotifyEffect struct {
	// AID is who it happened to, which is not always us: a level up beside us
	// arrives here too.
	AID    uint32
	Effect uint32
}

// DecodeNotifyEffect reads ZC_NOTIFY_EFFECT.
func DecodeNotifyEffect(data []byte) (NotifyEffect, bool) {
	if len(data) < 10 {
		return NotifyEffect{}, false
	}

	return NotifyEffect{
		AID:    binary.LittleEndian.Uint32(data[2:]),
		Effect: binary.LittleEndian.Uint32(data[6:]),
	}, true
}

// EncodeStatusUp asks to raise one stat.
//
// The id is the status id — SP_STR through SP_LUK — not the row it sits on in
// the window.
func EncodeStatusUp(statID uint16, amount uint8) []byte {
	pkt := make([]byte, 5)
	binary.LittleEndian.PutUint16(pkt, CZ_STATUS_CHANGE)
	binary.LittleEndian.PutUint16(pkt[2:], statID)
	pkt[4] = amount

	return pkt
}

// EncodeSkillUp asks to spend a skill point on one skill.
func EncodeSkillUp(skillID uint16) []byte {
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt, CZ_UPGRADE_SKILLLEVEL)
	binary.LittleEndian.PutUint16(pkt[2:], skillID)

	return pkt
}

// EncodeUnequip asks to take off whatever is worn in an inventory slot.
func EncodeUnequip(index int) []byte {
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt, CZ_REQ_TAKEOFF_EQUIP)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(index))

	return pkt
}

// Casting a skill at something, which is what a skill on the quick panel does.
const (
	// CZ_USE_SKILL is `<level>.W <skill>.W <target>.L`, 10 bytes.
	//
	// 0x0438 is what the default branch of clif_shuffle.hpp registers, which
	// is the branch that applies at this packetver — the numbered branches
	// above it are all `PACKETVER ==` equalities for particular 2013-era
	// clients. The main table registers clif_parse_UseSkillToId under half a
	// dozen other ids behind version guards, and picking one of those would
	// be picking a different client's packet.
	//
	// The field order is the parse offsets rAthena declares beside the id:
	// pos[0] at 2 is the level and pos[1] at 4 the skill, which is the
	// opposite way round from how the packet reads.
	CZ_USE_SKILL uint16 = 0x0438
)

// Which skills can be cast, from rAthena's e_skill_inf. A skill that targets
// nothing is passive and has no cast at all.
const (
	InfAttack  = 0x01
	InfGround  = 0x02
	InfSelf    = 0x04
	InfSupport = 0x10
	InfTrap    = 0x20
)

// EncodeUseSkill asks to cast a skill at a unit.
//
// The target is a unit id, and for a self-cast skill that is our own: rAthena
// takes whatever is sent and checks it against what the skill allows, so a
// skill cast on the caster still names the caster.
func EncodeUseSkill(skillID uint16, level int, targetGID uint32) []byte {
	pkt := make([]byte, 10)
	binary.LittleEndian.PutUint16(pkt, CZ_USE_SKILL)
	binary.LittleEndian.PutUint16(pkt[2:], uint16(level))
	binary.LittleEndian.PutUint16(pkt[4:], skillID)
	binary.LittleEndian.PutUint32(pkt[6:], targetGID)

	return pkt
}

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
	// Levelling is not its own packet. The server sends this for a base level,
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

package packets

import "testing"

// The entity packets are the ones where a wrong offset is least likely to be
// noticed: every field is a small integer, so reading a neighboring one still
// produces a decodable entity that simply looks or behaves wrong. These tests
// give every field a distinct value, so any offset that is off by even one
// byte lands on a value belonging to a different field.

func putU16(b []byte, offset int, v uint16) {
	b[offset] = byte(v)
	b[offset+1] = byte(v >> 8)
}

func putU32(b []byte, offset int, v uint32) {
	b[offset] = byte(v)
	b[offset+1] = byte(v >> 8)
	b[offset+2] = byte(v >> 16)
	b[offset+3] = byte(v >> 24)
}

func putName(b []byte, offset int, name string) {
	copy(b[offset:offset+nameLength], name)
}

// writePrefix fills the 35 bytes every unit packet starts with.
func writePrefix(b []byte, id uint16) {
	putU16(b, 0, id)
	putU16(b, 2, uint16(len(b)))
	b[offObjectType] = byte(EntityPlayer)
	putU32(b, offAID, 2000042)
	putU32(b, offGID, 110000123)
	putU16(b, offSpeed, 150)
	putU16(b, offJob, 4054)
	putU16(b, offHead, 17)
	putU32(b, offWeapon, 1201)
	putU32(b, offShield, 2101)
	putU16(b, offAccessory, 31)
}

func checkPrefix(t *testing.T, e *Entity) {
	t.Helper()
	if e.Kind != EntityPlayer {
		t.Errorf("Kind = %d, want %d", e.Kind, EntityPlayer)
	}
	if e.AID != 2000042 {
		t.Errorf("AID = %d, want 2000042", e.AID)
	}
	if e.GID != 110000123 {
		t.Errorf("GID = %d, want 110000123", e.GID)
	}
	if e.SpeedMs != 150 {
		t.Errorf("SpeedMs = %d, want 150", e.SpeedMs)
	}
	if e.Job != 4054 {
		t.Errorf("Job = %d, want 4054", e.Job)
	}
	if e.HairStyle != 17 {
		t.Errorf("HairStyle = %d, want 17", e.HairStyle)
	}
	if e.Weapon != 1201 {
		t.Errorf("Weapon = %d, want 1201", e.Weapon)
	}
	if e.Shield != 2101 {
		t.Errorf("Shield = %d, want 2101", e.Shield)
	}
	if e.Accessory != 31 {
		t.Errorf("Accessory = %d, want 31", e.Accessory)
	}
}

func TestDecodeEntityIdle(t *testing.T) {
	b := make([]byte, entityIdleBase)
	writePrefix(b, ZC_NOTIFY_STANDENTRY)
	putU16(b, offUnitHeadPalette, 6)
	putU16(b, offUnitBodyPalette, 3)
	putU16(b, offUnitHeadDir, 2)
	putU16(b, offUnitRobe, 9)
	b[offUnitSex] = 1
	copy(b[offUnitPos:], EncodePos(153, 244, 6))
	putU16(b, offIdleLevel, 99)
	putU32(b, offIdleMaxHP, 12345)
	putU32(b, offIdleHP, 6789)
	putName(b, offIdleName, "Prontera Guard")

	e := DecodeEntityIdle(b)
	if e == nil {
		t.Fatal("DecodeEntityIdle returned nil for a well-formed packet")
	}
	checkPrefix(t, e)

	if e.HairColor != 6 || e.ClothesColor != 3 {
		t.Errorf("palettes = hair %d clothes %d, want 6 and 3", e.HairColor, e.ClothesColor)
	}
	if e.HeadDir != 2 {
		t.Errorf("HeadDir = %d, want 2", e.HeadDir)
	}
	if e.Robe != 9 {
		t.Errorf("Robe = %d, want 9", e.Robe)
	}
	if e.Sex != 1 {
		t.Errorf("Sex = %d, want 1", e.Sex)
	}
	if e.X != 153 || e.Y != 244 || e.Dir != 6 {
		t.Errorf("position = (%d,%d) dir %d, want (153,244) dir 6", e.X, e.Y, e.Dir)
	}
	if e.Level != 99 {
		t.Errorf("Level = %d, want 99", e.Level)
	}
	if e.MaxHP != 12345 || e.HP != 6789 {
		t.Errorf("HP = %d/%d, want 6789/12345", e.HP, e.MaxHP)
	}
	if e.Name != "Prontera Guard" {
		t.Errorf("Name = %q, want %q", e.Name, "Prontera Guard")
	}
	if e.Moving {
		t.Error("a standing unit should not be marked as moving")
	}
	if e.ToX != e.X || e.ToY != e.Y {
		t.Error("a standing unit's destination should be where it already is")
	}
}

// TestDecodeEntitySpawn covers the one-byte shift that separates the spawn
// packet from the idle one: it has no state byte, so everything from clevel
// onward sits one lower.
func TestDecodeEntitySpawn(t *testing.T) {
	b := make([]byte, entitySpawnBase)
	writePrefix(b, ZC_NOTIFY_NEWENTRY)
	copy(b[offUnitPos:], EncodePos(50, 60, 2))
	putU16(b, offSpawnLevel, 42)
	putU32(b, offSpawnMaxHP, 500)
	putU32(b, offSpawnHP, 250)
	putName(b, offSpawnName, "Poring")

	e := DecodeEntitySpawn(b)
	if e == nil {
		t.Fatal("DecodeEntitySpawn returned nil for a well-formed packet")
	}
	checkPrefix(t, e)

	if e.X != 50 || e.Y != 60 || e.Dir != 2 {
		t.Errorf("position = (%d,%d) dir %d, want (50,60) dir 2", e.X, e.Y, e.Dir)
	}
	if e.Level != 42 {
		t.Errorf("Level = %d, want 42; the spawn packet has no state byte, so "+
			"clevel sits one byte lower than in the idle packet", e.Level)
	}
	if e.MaxHP != 500 || e.HP != 250 {
		t.Errorf("HP = %d/%d, want 250/500", e.HP, e.MaxHP)
	}
	if e.Name != "Poring" {
		t.Errorf("Name = %q, want %q", e.Name, "Poring")
	}
}

func TestDecodeEntityWalk(t *testing.T) {
	b := make([]byte, entityWalkBase)
	writePrefix(b, ZC_NOTIFY_MOVEENTRY)
	putU32(b, offWalkMoveStart, 987654)
	putU16(b, offWalkHeadPalette, 6)
	putU16(b, offWalkBodyPalette, 3)
	putU16(b, offWalkHeadDir, 1)
	putU16(b, offWalkRobe, 9)
	b[offWalkSex] = 0
	copy(b[offWalkMoveData:], EncodePos2(100, 110, 104, 113, 8, 8))
	putU16(b, offWalkLevel, 55)
	putU32(b, offWalkMaxHP, 3000)
	putU32(b, offWalkHP, 2999)
	putName(b, offWalkName, "Walking Merchant")

	e := DecodeEntityWalk(b)
	if e == nil {
		t.Fatal("DecodeEntityWalk returned nil for a well-formed packet")
	}
	checkPrefix(t, e)

	if !e.Moving {
		t.Error("a unit from the walking packet should be marked as moving")
	}
	if e.MoveStartTick != 987654 {
		t.Errorf("MoveStartTick = %d, want 987654", e.MoveStartTick)
	}
	if e.X != 100 || e.Y != 110 {
		t.Errorf("start = (%d,%d), want (100,110)", e.X, e.Y)
	}
	if e.ToX != 104 || e.ToY != 113 {
		t.Errorf("destination = (%d,%d), want (104,113)", e.ToX, e.ToY)
	}
	if e.Level != 55 {
		t.Errorf("Level = %d, want 55; the 4-byte move start time shifts every "+
			"field after it", e.Level)
	}
	if e.MaxHP != 3000 || e.HP != 2999 {
		t.Errorf("HP = %d/%d, want 2999/3000", e.HP, e.MaxHP)
	}
	if e.Name != "Walking Merchant" {
		t.Errorf("Name = %q, want %q", e.Name, "Walking Merchant")
	}
	if e.Dir != -1 {
		t.Errorf("Dir = %d, want -1; the walking packet carries no facing, and "+
			"defaulting to 0 would face every walker south", e.Dir)
	}
}

// TestDecodeEntityShortData: a truncated packet must be rejected rather than
// read past its end. The stream is framed by declared length, so a short one
// means the declared length was wrong and nothing in it can be trusted.
func TestDecodeEntityShortData(t *testing.T) {
	tests := []struct {
		name   string
		decode func([]byte) *Entity
		size   int
	}{
		{"idle", DecodeEntityIdle, entityIdleBase},
		{"spawn", DecodeEntitySpawn, entitySpawnBase},
		{"walk", DecodeEntityWalk, entityWalkBase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decode(make([]byte, tt.size-1)); got != nil {
				t.Error("decoded a packet one byte short of the minimum")
			}
			if got := tt.decode(nil); got != nil {
				t.Error("decoded a nil buffer")
			}
			if got := tt.decode(make([]byte, tt.size)); got == nil {
				t.Error("rejected a packet of exactly the minimum size")
			}
		})
	}
}

func TestDecodeEntityVanish(t *testing.T) {
	b := make([]byte, 7)
	putU16(b, 0, ZC_NOTIFY_VANISH)
	putU32(b, 2, 110000123)
	b[6] = 2 // logged out

	aid, reason, ok := DecodeEntityVanish(b)
	if !ok {
		t.Fatal("DecodeEntityVanish rejected a well-formed packet")
	}
	if aid != 110000123 {
		t.Errorf("aid = %d, want 110000123", aid)
	}
	if reason != 2 {
		t.Errorf("reason = %d, want 2", reason)
	}

	if _, _, ok := DecodeEntityVanish(make([]byte, 6)); ok {
		t.Error("accepted a truncated vanish packet")
	}
}

// TestEntityPacketIDsAreKnown guards the failure that made entities invisible
// in the first place: handlers were registered against ids that do not exist
// at this PACKETVER, so they never fired, and an arriving packet would have
// been resynchronized past as unknown.
func TestEntityPacketIDsAreKnown(t *testing.T) {
	tests := []struct {
		name string
		id   uint16
		want int
	}{
		{"ZC_NOTIFY_STANDENTRY", ZC_NOTIFY_STANDENTRY, VariableLength},
		{"ZC_NOTIFY_NEWENTRY", ZC_NOTIFY_NEWENTRY, VariableLength},
		{"ZC_NOTIFY_MOVEENTRY", ZC_NOTIFY_MOVEENTRY, VariableLength},
		{"ZC_NOTIFY_VANISH", ZC_NOTIFY_VANISH, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := Length(tt.id)
			if !known {
				t.Fatalf("0x%04X is not in the generated length table, so the "+
					"framing layer would discard it", tt.id)
			}
			if got != tt.want {
				t.Errorf("length = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestEntityKindIsCharacter separates the units drawn from the humanoid sprite
// set from the ones that are not, which is what decides how each is composited.
func TestEntityKindIsCharacter(t *testing.T) {
	for _, kind := range []EntityKind{EntityPlayer, EntityDisguised} {
		if !kind.IsCharacter() {
			t.Errorf("kind 0x%X should be drawn as a character", kind)
		}
	}
	for _, kind := range []EntityKind{EntityMob, EntityNPC, EntityItem, EntitySkill, EntityWalkingNPC} {
		if kind.IsCharacter() {
			t.Errorf("kind 0x%X should not be drawn as a character", kind)
		}
	}
}

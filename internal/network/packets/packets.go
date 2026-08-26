// Package packets defines Hercules protocol packets.
package packets

import "fmt"

// Packet IDs for login server
const (
	// Client -> Login Server
	CA_LOGIN         uint16 = 0x0064 // Login request
	CA_REQ_HASH      uint16 = 0x01DB // Request password hash
	CA_LOGIN_HASH    uint16 = 0x01DD // Login with hash
	CA_SSO_LOGIN_REQ uint16 = 0x0825 // SSO login request
	CA_REQ_NEW_ACC   uint16 = 0x0068 // Registration request

	// Login Server -> Client
	AC_ACCEPT_LOGIN  uint16 = 0x0069 // Login accepted (old)
	AC_ACCEPT_LOGIN2 uint16 = 0x0AC4 // Login accepted (modern rAthena)
	AC_REFUSE_LOGIN  uint16 = 0x006A // Login refused (old)
	AC_REFUSE_LOGIN2 uint16 = 0x083E // Login refused (modern)
	AC_NOTIFY_ERROR  uint16 = 0x0081 // Notify error
)

// Packet IDs for character server
const (
	// Client -> Char Server
	CH_ENTER       uint16 = 0x0065 // Enter char server
	CH_SELECT_CHAR uint16 = 0x0066 // Select character
	CH_MAKE_CHAR   uint16 = 0x0067 // Create character
	CH_DELETE_CHAR uint16 = 0x0068 // Delete character

	// Char Server -> Client
	HC_ACCEPT_ENTER    uint16 = 0x006B // Enter accepted + char list
	HC_REFUSE_ENTER    uint16 = 0x006C // Enter refused
	HC_ACCEPT_MAKECHAR uint16 = 0x006D // Character created
	HC_NOTIFY_ZONESVR  uint16 = 0x0071 // Map server info (old)
	HC_NOTIFY_ZONESVR2 uint16 = 0x0AC5 // Map server info (modern rAthena)
)

// Packet IDs for map server.
//
// rAthena shuffles packet IDs by packetver. The IDs below are the ones
// rAthena binds for our pinned packetver (20211103, see
// docker/rathena/docker-compose.yml BUILDER_CONFIGURE). For
// PACKETVER > 20180307 most C->S map-server packets get re-bound to the
// 0x03XX range (see clif_shuffle.hpp).
const (
	// Client -> Map Server
	CZ_ENTER            uint16 = 0x0072 // Enter map (old, pre-2008)
	CZ_ENTER2           uint16 = 0x0436 // Enter map (modern rAthena with auth token)
	CZ_REQUEST_MOVE     uint16 = 0x035F // Request move (WalkToXY) — was 0x0085 pre-2010
	CZ_REQUEST_TIME     uint16 = 0x0360 // Keep-alive (TickSend) — must be sent or session times out
	CZ_NOTIFY_ACTORINIT uint16 = 0x007D // Loading complete

	// Map Server -> Client
	ZC_ACCEPT_ENTER  uint16 = 0x0073 // Map enter accepted (old)
	ZC_ACCEPT_ENTER2 uint16 = 0x02EB // Map enter accepted (modern rAthena)
	// Unit appearance and movement. These ids move with the packet version:
	// what older clients know as 0x0078 and 0x007B does not exist at
	// PACKETVER 20211103, so handlers registered against those never fire and
	// an arriving packet would be treated as unknown and resynchronized past.
	// See rAthena's idle_unitType / spawn_unitType / unit_walkingType.
	ZC_NOTIFY_STANDENTRY uint16 = 0x09FF // Unit standing still
	ZC_NOTIFY_NEWENTRY   uint16 = 0x09FE // Unit appearing
	ZC_NOTIFY_MOVEENTRY  uint16 = 0x09FD // Unit walking
	ZC_NOTIFY_VANISH     uint16 = 0x0080 // Unit removed
	ZC_NOTIFY_PLAYERMOVE uint16 = 0x0087 // Own player walk-OK (start_tick + packed positions)
	ZC_NOTIFY_ACT        uint16 = 0x008A // Entity action
	ZC_NPCACK_MAPMOVE    uint16 = 0x0091 // Map change (server-driven warp)
	ZC_NOTIFY_TIME       uint16 = 0x007F // Server tick reply to CZ_REQUEST_TIME
)

// LoginRequest (CA_LOGIN 0x0064)
type LoginRequest struct {
	PacketID uint16   // 0x0064
	Version  uint32   // Client version
	Username [24]byte // Username
	Password [24]byte // Password
	Type     uint8    // Client type
}

// Size returns packet size.
func (p *LoginRequest) Size() int {
	return 55
}

// Encode encodes the packet to bytes.
func (p *LoginRequest) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	buf[2] = byte(p.Version)
	buf[3] = byte(p.Version >> 8)
	buf[4] = byte(p.Version >> 16)
	buf[5] = byte(p.Version >> 24)
	copy(buf[6:30], p.Username[:])
	copy(buf[30:54], p.Password[:])
	buf[54] = p.Type
	return buf
}

// LoginAccept (AC_ACCEPT_LOGIN 0x0069)
type LoginAccept struct {
	PacketID  uint16
	PacketLen uint16
	LoginID1  uint32
	AccountID uint32
	LoginID2  uint32
	// ... more fields
}

// CharServerInfo contains character server information.
type CharServerInfo struct {
	IP       uint32
	Port     uint16
	Name     [20]byte
	Users    uint16
	State    uint16
	Property uint16
}

// CharEnter (CH_ENTER 0x0065)
type CharEnter struct {
	PacketID  uint16 // 0x0065
	AccountID uint32
	LoginID1  uint32
	LoginID2  uint32
	Sex       uint8
}

// Size returns packet size.
func (p *CharEnter) Size() int {
	return 17
}

// Encode encodes the packet.
func (p *CharEnter) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	buf[2] = byte(p.AccountID)
	buf[3] = byte(p.AccountID >> 8)
	buf[4] = byte(p.AccountID >> 16)
	buf[5] = byte(p.AccountID >> 24)
	buf[6] = byte(p.LoginID1)
	buf[7] = byte(p.LoginID1 >> 8)
	buf[8] = byte(p.LoginID1 >> 16)
	buf[9] = byte(p.LoginID1 >> 24)
	buf[10] = byte(p.LoginID2)
	buf[11] = byte(p.LoginID2 >> 8)
	buf[12] = byte(p.LoginID2 >> 16)
	buf[13] = byte(p.LoginID2 >> 24)
	// bytes 14-15 unused
	buf[16] = p.Sex
	return buf
}

// CharInfo contains character information (106 bytes in packet).
type CharInfo struct {
	CharID       uint32
	BaseExp      uint32
	Zeny         uint32
	JobExp       uint32
	JobLevel     uint32
	BodyState    uint32
	HealthState  uint32
	EffectState  uint32
	Virtue       uint32
	Honor        uint32
	StatusPoint  uint16
	HP           uint32
	MaxHP        uint32
	SP           uint32
	MaxSP        uint32
	WalkSpeed    uint16
	Class        uint16
	HairStyle    uint16
	Body         uint16
	Weapon       uint16
	BaseLevel    uint16
	SkillPoint   uint16
	HeadBottom   uint16
	Shield       uint16
	HeadTop      uint16
	HeadMid      uint16
	HairColor    uint16
	ClothesColor uint16
	Name         [24]byte
	Str          uint8
	Agi          uint8
	Vit          uint8
	Int          uint8
	Dex          uint8
	Luk          uint8
	Slot         uint8
	Rename       uint8
	MapName      [16]byte
	DeleteDate   uint32
	Robe         uint16
	SlotChange   uint32
	Rename2      uint32
	Sex          uint8
}

// CharInfoSize is the wire size of one CHARACTER_INFO, and CharInfoSizeOld
// the size before the 64-bit widening.
//
// The layout is version-dependent. At PACKETVER_RE >= 20211103 — the server
// in docker/rathena — hp, maxhp, sp and maxsp are each int64, which is what
// makes the record 175 bytes; before that they were int32/int32/int16/int16
// and it was 155. Everything else is the same. Our packet length table is
// generated for 20211103, so this decoder reads that layout; a server built
// for an older PACKETVER would need the other one.
const (
	CharInfoSize    = 175
	CharInfoSizeOld = 155
)

// Field offsets within CHARACTER_INFO, from rAthena src/common/packets.hpp:31
// with PACKETVER 20211103 and PACKETVER_RE. The struct is `#pragma pack(1)`,
// so these are just the running sum of the field widths — no padding.
//
// They were previously guessed from a live capture, and were wrong in a way
// that looked plausible: hp landed on sp and maxhp on maxsp, so a character
// with 40 HP and 11 SP read back as 11 HP and 40 SP. Both numbers were real,
// which is why it survived being looked at.
const (
	ciCharID       = 0   // uint32 GID
	ciBaseExp      = 4   // int64  exp        (int32 before 20170830)
	ciZeny         = 12  // int32  money
	ciJobExp       = 16  // int64  jobexp
	ciJobLevel     = 24  // int32  joblevel
	ciStatusPoint  = 48  // int16  jobpoint
	ciHP           = 50  // int64  hp
	ciMaxHP        = 58  // int64  maxhp
	ciSP           = 66  // int64  sp
	ciMaxSP        = 74  // int64  maxsp
	ciWalkSpeed    = 82  // int16  speed
	ciClass        = 84  // int16  job
	ciHairStyle    = 86  // int16  head
	ciBody         = 88  // int16  body       (since 20141022)
	ciWeapon       = 90  // int16  weapon
	ciBaseLevel    = 92  // int16  level
	ciSkillPoint   = 94  // int16  sppoint
	ciHeadBottom   = 96  // int16  accessory
	ciShield       = 98  // int16  shield
	ciHeadTop      = 100 // int16  accessory2
	ciHeadMid      = 102 // int16  accessory3
	ciHairColor    = 104 // int16  headpalette
	ciClothesColor = 106 // int16  bodypalette
	ciName         = 108 // char   name[24]
	ciStr          = 132 // uint8  Str, then Agi Vit Int Dex Luk
	ciSlot         = 138 // uint8  CharNum
	ciRename       = 140 // int16  bIsChangedCharName
	ciMapName      = 142 // char   mapName[16]
	ciDeleteDate   = 158 // int32  DelRevDate
	ciRobe         = 162 // int32  robePalette
	ciSlotChange   = 166 // int32  chr_slot_changeCnt
	ciRename2      = 170 // int32  chr_name_changeCnt
	ciSex          = 174 // uint8  sex
)

// DecodeCharInfo decodes one character record from a character list.
//
// The 64-bit fields are read as their low 32 bits: rAthena caps hp and maxhp
// at int32 and sp and maxsp at int16 before sending, so the high half is
// always zero and nothing is lost.
func DecodeCharInfo(data []byte) *CharInfo {
	if len(data) < CharInfoSize {
		return nil
	}

	c := &CharInfo{
		CharID:       readU32(data, ciCharID),
		BaseExp:      readU32(data, ciBaseExp),
		Zeny:         readU32(data, ciZeny),
		JobExp:       readU32(data, ciJobExp),
		JobLevel:     readU32(data, ciJobLevel),
		StatusPoint:  readU16(data, ciStatusPoint),
		HP:           readU32(data, ciHP),
		MaxHP:        readU32(data, ciMaxHP),
		SP:           readU32(data, ciSP),
		MaxSP:        readU32(data, ciMaxSP),
		WalkSpeed:    readU16(data, ciWalkSpeed),
		Class:        readU16(data, ciClass),
		HairStyle:    readU16(data, ciHairStyle),
		Body:         readU16(data, ciBody),
		Weapon:       readU16(data, ciWeapon),
		BaseLevel:    readU16(data, ciBaseLevel),
		SkillPoint:   readU16(data, ciSkillPoint),
		HeadBottom:   readU16(data, ciHeadBottom),
		Shield:       readU16(data, ciShield),
		HeadTop:      readU16(data, ciHeadTop),
		HeadMid:      readU16(data, ciHeadMid),
		HairColor:    readU16(data, ciHairColor),
		ClothesColor: readU16(data, ciClothesColor),
		Str:          data[ciStr],
		Agi:          data[ciStr+1],
		Vit:          data[ciStr+2],
		Int:          data[ciStr+3],
		Dex:          data[ciStr+4],
		Luk:          data[ciStr+5],
		Slot:         data[ciSlot],
		Rename:       uint8(readU16(data, ciRename)),
		DeleteDate:   readU32(data, ciDeleteDate),
		Robe:         readU16(data, ciRobe),
		SlotChange:   readU32(data, ciSlotChange),
		Rename2:      readU32(data, ciRename2),
		Sex:          data[ciSex],
	}
	copy(c.Name[:], data[ciName:ciName+24])
	copy(c.MapName[:], data[ciMapName:ciMapName+16])

	return c
}

// GetName returns the character name as a string.
func (c *CharInfo) GetName() string {
	for i, b := range c.Name {
		if b == 0 {
			return string(c.Name[:i])
		}
	}

	return string(c.Name[:])
}

// GetMapName returns the map name as a string.
func (c *CharInfo) GetMapName() string {
	for i, b := range c.MapName {
		if b == 0 {
			return string(c.MapName[:i])
		}
	}

	return string(c.MapName[:])
}

// CharSelectAccept (HC_ACCEPT_ENTER 0x006B) response.
// eAthena uses a 27-byte header before character data.
type CharSelectAccept struct {
	PacketID   uint16
	PacketLen  uint16
	MaxSlots   uint8
	AvailSlots uint8
	PremSlots  uint8
	Padding    [20]byte // eAthena: billing info + padding = 20 bytes
	Characters []*CharInfo
}

// DecodeCharSelectAccept decodes the character select accept packet.
func DecodeCharSelectAccept(data []byte) *CharSelectAccept {
	if len(data) < 27 {
		return nil
	}
	p := &CharSelectAccept{
		PacketID:   readU16(data, 0),
		PacketLen:  readU16(data, 2),
		MaxSlots:   data[4],
		AvailSlots: data[5],
		PremSlots:  data[6],
	}
	copy(p.Padding[:], data[7:27])

	// Parse character data starting at offset 27 (eAthena header size)
	charDataStart := 27
	charDataLen := int(p.PacketLen) - charDataStart

	// Calculate number of characters based on remaining data
	// eAthena CharInfo can vary; try to detect size from packet
	if charDataLen > 0 && charDataLen >= CharInfoSize {
		numChars := charDataLen / CharInfoSize
		for i := 0; i < numChars; i++ {
			offset := charDataStart + (i * CharInfoSize)
			if offset+CharInfoSize > len(data) {
				break
			}
			if char := DecodeCharInfo(data[offset:]); char != nil {
				p.Characters = append(p.Characters, char)
			}
		}
	}
	return p
}

// CharSelect (CH_SELECT_CHAR 0x0066) packet.
type CharSelect struct {
	PacketID uint16 // 0x0066
	Slot     uint8  // Character slot (0-8)
}

// Size returns packet size.
func (p *CharSelect) Size() int {
	return 3
}

// Encode encodes the packet.
func (p *CharSelect) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	buf[2] = p.Slot
	return buf
}

// MapServerInfo (HC_NOTIFY_ZONESVR 0x0071) response.
type MapServerInfo struct {
	PacketID uint16
	CharID   uint32
	MapName  [16]byte
	IP       uint32
	Port     uint16
}

// DecodeMapServerInfo decodes the map server info packet.
func DecodeMapServerInfo(data []byte) *MapServerInfo {
	if len(data) < 28 {
		return nil
	}
	p := &MapServerInfo{
		PacketID: readU16(data, 0),
		CharID:   readU32(data, 2),
		IP:       readU32(data, 22),
		Port:     readU16(data, 26),
	}
	copy(p.MapName[:], data[6:22])
	return p
}

// GetMapName returns the map name as a string.
func (p *MapServerInfo) GetMapName() string {
	for i, b := range p.MapName {
		if b == 0 {
			return string(p.MapName[:i])
		}
	}
	return string(p.MapName[:])
}

// GetIP returns the IP address as a dotted string.
func (p *MapServerInfo) GetIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(p.IP), byte(p.IP>>8), byte(p.IP>>16), byte(p.IP>>24))
}

// MapEnter (CZ_ENTER 0x0072) packet.
type MapEnter struct {
	PacketID   uint16 // 0x0072
	AccountID  uint32
	CharID     uint32
	LoginID1   uint32
	ClientTick uint32
	Sex        uint8
}

// Size returns packet size.
func (p *MapEnter) Size() int {
	return 19
}

// Encode encodes the packet.
func (p *MapEnter) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	writeU32(buf, 2, p.AccountID)
	writeU32(buf, 6, p.CharID)
	writeU32(buf, 10, p.LoginID1)
	writeU32(buf, 14, p.ClientTick)
	buf[18] = p.Sex
	return buf
}

// MapEnter2 (CZ_ENTER2 0x0436) packet - modern rAthena (Korangar format).
// Note: This does NOT include auth token - it uses 4 unknown bytes instead.
type MapEnter2 struct {
	PacketID   uint16 // 0x0436
	AccountID  uint32
	CharID     uint32
	LoginID1   uint32
	ClientTick uint32
	Sex        uint8
	Unknown    [4]byte // Always zeros
}

// Size returns packet size.
func (p *MapEnter2) Size() int {
	return 23
}

// Encode encodes the packet.
func (p *MapEnter2) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	writeU32(buf, 2, p.AccountID)
	writeU32(buf, 6, p.CharID)
	writeU32(buf, 10, p.LoginID1)
	writeU32(buf, 14, p.ClientTick)
	buf[18] = p.Sex
	// Unknown bytes 19-22 are left as zeros
	return buf
}

// MapAccept (ZC_ACCEPT_ENTER 0x0073 / ZC_ACCEPT_ENTER2 0x02EB) response.
type MapAccept struct {
	PacketID  uint16
	StartTime uint32
	PosDir    [3]byte // Packed position and direction
	Unknown   [2]byte
	Font      uint16 // Only in ZC_ACCEPT_ENTER2
}

// DecodeMapAccept decodes the map enter accept packet.
// Handles both ZC_ACCEPT_ENTER (11 bytes) and ZC_ACCEPT_ENTER2 (13 bytes).
func DecodeMapAccept(data []byte) *MapAccept {
	if len(data) < 11 {
		return nil
	}
	p := &MapAccept{
		PacketID:  readU16(data, 0),
		StartTime: readU32(data, 2),
	}
	copy(p.PosDir[:], data[6:9])
	copy(p.Unknown[:], data[9:11])
	// ZC_ACCEPT_ENTER2 has extra 2 bytes for font
	if len(data) >= 13 {
		p.Font = readU16(data, 11)
	}
	return p
}

// GetPosition unpacks the position from PosDir.
func (p *MapAccept) GetPosition() (x, y int, dir uint8) {
	// Position is packed in 3 bytes: XXXXYYYY YYYYDDDD
	x = (int(p.PosDir[0]) << 2) | (int(p.PosDir[1]) >> 6)
	y = ((int(p.PosDir[1]) & 0x3F) << 4) | (int(p.PosDir[2]) >> 4)
	dir = p.PosDir[2] & 0x0F
	return
}

// MoveRequest (CZ_REQUEST_MOVE 0x035F for packetver 20211103) packet.
type MoveRequest struct {
	PacketID uint16  // 0x035F
	Dest     [3]byte // Packed destination (x:10 | y:10 | dir:4)
}

// Size returns packet size.
func (p *MoveRequest) Size() int {
	return 5
}

// Encode encodes the packet.
func (p *MoveRequest) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	copy(buf[2:5], p.Dest[:])
	return buf
}

// SetDestination packs the destination coordinates.
func (p *MoveRequest) SetDestination(x, y int) {
	// Pack position into 3 bytes (rAthena WBUFPOS: x:10|y:10|dir:4)
	p.Dest[0] = byte(x >> 2)
	p.Dest[1] = byte((x << 6) | ((y >> 4) & 0x3F))
	p.Dest[2] = byte(y << 4)
}

// TickSend (CZ_REQUEST_TIME 0x0360 for packetver 20211103) — keep-alive
// from client to map server. rAthena's map server times out the session
// after a few seconds of silence, so this must be sent periodically
// (every ~10 s is safe).
type TickSend struct {
	PacketID   uint16 // 0x0360
	ClientTick uint32 // Milliseconds since some local epoch
}

// Size returns packet size.
func (p *TickSend) Size() int {
	return 6
}

// Encode encodes the packet.
func (p *TickSend) Encode() []byte {
	buf := make([]byte, p.Size())
	buf[0] = byte(p.PacketID)
	buf[1] = byte(p.PacketID >> 8)
	buf[2] = byte(p.ClientTick)
	buf[3] = byte(p.ClientTick >> 8)
	buf[4] = byte(p.ClientTick >> 16)
	buf[5] = byte(p.ClientTick >> 24)
	return buf
}

// PlayerMove (ZC_NOTIFY_PLAYERMOVE 0x0087, 12 bytes) — server confirms
// our own move, returning the start tick and packed start/end positions.
type PlayerMove struct {
	StartTick uint32
	StartX    int
	StartY    int
	EndX      int
	EndY      int
}

// DecodePlayerMove parses ZC_NOTIFY_PLAYERMOVE. Returns nil on short data.
//
// Layout: header(2) + start_tick(4) + walk_data(6) where walk_data uses
// rAthena WBUFPOS2: x0:10 | y0:10 | x1:10 | y1:10 | sx:4 | sy:4 = 48 bits.
func DecodePlayerMove(data []byte) *PlayerMove {
	if len(data) < 12 {
		return nil
	}
	tick := uint32(data[2]) | uint32(data[3])<<8 | uint32(data[4])<<16 | uint32(data[5])<<24

	x0, y0, x1, y1, _, _ := DecodePos2(data[6:12])

	return &PlayerMove{
		StartTick: tick,
		StartX:    x0,
		StartY:    y0,
		EndX:      x1,
		EndY:      y1,
	}
}

// LoadingComplete (CZ_NOTIFY_ACTORINIT 0x007D) packet.
type LoadingComplete struct {
	PacketID uint16 // 0x007D
}

// Size returns packet size.
func (p *LoadingComplete) Size() int {
	return 2
}

// Encode encodes the packet.
func (p *LoadingComplete) Encode() []byte {
	return []byte{byte(p.PacketID), byte(p.PacketID >> 8)}
}

// Helper functions for packet encoding/decoding

func readU16(data []byte, offset int) uint16 {
	return uint16(data[offset]) | uint16(data[offset+1])<<8
}

func readU32(data []byte, offset int) uint32 {
	return uint32(data[offset]) | uint32(data[offset+1])<<8 |
		uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
}

func writeU32(buf []byte, offset int, v uint32) {
	buf[offset] = byte(v)
	buf[offset+1] = byte(v >> 8)
	buf[offset+2] = byte(v >> 16)
	buf[offset+3] = byte(v >> 24)
}

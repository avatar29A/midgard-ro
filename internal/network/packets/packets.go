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
	HC_ACCEPT_ENTER     uint16 = 0x006B // Enter accepted + char list
	HC_REFUSE_ENTER     uint16 = 0x006C // Enter refused
	HC_ACCEPT_MAKECHAR  uint16 = 0x006D // Character created
	HC_NOTIFY_ZONESVR   uint16 = 0x0071 // Map server info (old)
	HC_NOTIFY_ZONESVR2  uint16 = 0x0AC5 // Map server info (modern rAthena)
)

// Packet IDs for map server
const (
	// Client -> Map Server
	CZ_ENTER            uint16 = 0x0072 // Enter map (old)
	CZ_ENTER2           uint16 = 0x0436 // Enter map (modern rAthena with auth token)
	CZ_REQUEST_MOVE     uint16 = 0x0085 // Request move
	CZ_NOTIFY_ACTORINIT uint16 = 0x007D // Loading complete

	// Map Server -> Client
	ZC_ACCEPT_ENTER      uint16 = 0x0073 // Map enter accepted (old)
	ZC_ACCEPT_ENTER2     uint16 = 0x02EB // Map enter accepted (modern rAthena)
	ZC_NOTIFY_STANDENTRY uint16 = 0x0078 // Entity spawn (standing)
	ZC_NOTIFY_MOVEENTRY  uint16 = 0x007B // Entity spawn (moving)
	ZC_NOTIFY_ACT        uint16 = 0x008A // Entity action
	ZC_NPCACK_MAPMOVE    uint16 = 0x0091 // Map change
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
	SP           uint16
	MaxSP        uint16
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

// CharInfoSize is the size of CharInfo in the packet.
// eAthena uses 175 bytes per character (with 64-bit exp/zeny fields).
// rAthena/Hercules uses 155 bytes.
const CharInfoSize = 175
const CharInfoSizeRathena = 155

// DecodeCharInfo decodes character info from bytes.
// This decoder supports eAthena's 175-byte format with 64-bit fields.
func DecodeCharInfo(data []byte) *CharInfo {
	if len(data) < CharInfoSize {
		return nil
	}

	// eAthena packet layout (175 bytes total):
	// Offset 0: CharID (4 bytes)
	// Offset 4: BaseExp (8 bytes - int64)
	// Offset 12: Zeny (8 bytes - int64)
	// Offset 20: JobExp (8 bytes - int64)
	// Offset 28: JobLevel (4 bytes)
	// ... other fields with 64-bit HP/MaxHP ...
	// Offset 48: WalkSpeed (2 bytes)
	// Offset 50: Class (2 bytes)
	// Offset 56: HP (8 bytes - int64)
	// Offset 64: MaxHP (8 bytes - int64)
	// Offset 72: SP (4 bytes)
	// Offset 76: MaxSP (4 bytes)
	// Offset 80-107: Various 2-byte fields (HairStyle, Body, Weapon, BaseLevel, etc.)
	// Offset 108: Name (24 bytes)
	// Offset 132: Stats (6 bytes: Str, Agi, Vit, Int, Dex, Luk)
	// Offset 138: Slot (1 byte)
	// Offset 139: Rename (1 byte)
	// Offset 140: Reserved (2 bytes)
	// Offset 142: MapName (16 bytes)
	// Offset 158: Remaining fields...

	// eAthena packet field positions (determined empirically):
	// Offset 0: CharID (4 bytes)
	// Offset 50: Class (2 bytes) = 40 (Super Novice)
	// Offset 58: SP (2 bytes)
	// Offset 66: HP (2 bytes) = 11
	// Offset 74: MaxHP (2 bytes) = 11
	// Offset 82: WalkSpeed (2 bytes) = 150
	// Offset 90: BaseLevel (2 bytes) = 1
	// Offset 92: JobLevel (2 bytes) = 1
	// Offset 108: Name (24 bytes)
	// Offset 132: Stats (6 bytes)
	// Offset 138: Slot (1 byte)
	// Offset 142: MapName (16 bytes)

	c := &CharInfo{
		CharID:       readU32(data, 0),
		BaseExp:      uint32(readU32(data, 4)),
		Zeny:         uint32(readU32(data, 12)),
		JobExp:       uint32(readU32(data, 20)),
		JobLevel:     readU32(data, 28),
		BodyState:    0,
		HealthState:  0,
		EffectState:  0,
		Virtue:       0,
		Honor:        0,
		StatusPoint:  0,
		HP:           uint32(readU16(data, 66)),  // HP at offset 66
		MaxHP:        uint32(readU16(data, 74)),  // MaxHP at offset 74
		SP:           readU16(data, 58),          // SP at offset 58
		MaxSP:        readU16(data, 58),          // Assume same as SP for now
		WalkSpeed:    readU16(data, 82),          // WalkSpeed at offset 82
		Class:        readU16(data, 50),          // Class at offset 50
		HairStyle:    readU16(data, 84),
		Body:         readU16(data, 86),
		Weapon:       readU16(data, 88),
		BaseLevel:    readU16(data, 90),          // BaseLevel at offset 90
		SkillPoint:   0,
		HeadBottom:   0,
		Shield:       0,
		HeadTop:      0,
		HeadMid:      0,
		HairColor:    readU16(data, 100),
		ClothesColor: readU16(data, 102),
		Str:          data[132],
		Agi:          data[133],
		Vit:          data[134],
		Int:          data[135],
		Dex:          data[136],
		Luk:          data[137],
		Slot:         data[138],
		Rename:       data[139],
		DeleteDate:   readU32(data, 158),
		Robe:         readU16(data, 162),
		SlotChange:   0,
		Rename2:      0,
		Sex:          data[174],
	}
	copy(c.Name[:], data[108:132])
	copy(c.MapName[:], data[142:158])
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
	PacketID   uint16    // 0x0436
	AccountID  uint32
	CharID     uint32
	LoginID1   uint32
	ClientTick uint32
	Sex        uint8
	Unknown    [4]byte   // Always zeros
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
	Font      uint16  // Only in ZC_ACCEPT_ENTER2
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

// MoveRequest (CZ_REQUEST_MOVE 0x0085) packet.
type MoveRequest struct {
	PacketID uint16 // 0x0085
	Dest     [3]byte // Packed destination
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
	// Pack position into 3 bytes
	p.Dest[0] = byte(x >> 2)
	p.Dest[1] = byte((x << 6) | ((y >> 4) & 0x3F))
	p.Dest[2] = byte(y << 4)
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

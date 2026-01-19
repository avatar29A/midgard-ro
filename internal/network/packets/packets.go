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
const CharInfoSize = 155

// DecodeCharInfo decodes character info from bytes.
func DecodeCharInfo(data []byte) *CharInfo {
	if len(data) < CharInfoSize {
		return nil
	}
	c := &CharInfo{
		CharID:       readU32(data, 0),
		BaseExp:      readU32(data, 4),
		Zeny:         readU32(data, 8),
		JobExp:       readU32(data, 12),
		JobLevel:     readU32(data, 16),
		BodyState:    readU32(data, 20),
		HealthState:  readU32(data, 24),
		EffectState:  readU32(data, 28),
		Virtue:       readU32(data, 32),
		Honor:        readU32(data, 36),
		StatusPoint:  readU16(data, 40),
		HP:           readU32(data, 42),
		MaxHP:        readU32(data, 46),
		SP:           readU16(data, 50),
		MaxSP:        readU16(data, 52),
		WalkSpeed:    readU16(data, 54),
		Class:        readU16(data, 56),
		HairStyle:    readU16(data, 58),
		Body:         readU16(data, 60),
		Weapon:       readU16(data, 62),
		BaseLevel:    readU16(data, 64),
		SkillPoint:   readU16(data, 66),
		HeadBottom:   readU16(data, 68),
		Shield:       readU16(data, 70),
		HeadTop:      readU16(data, 72),
		HeadMid:      readU16(data, 74),
		HairColor:    readU16(data, 76),
		ClothesColor: readU16(data, 78),
		Str:          data[104],
		Agi:          data[105],
		Vit:          data[106],
		Int:          data[107],
		Dex:          data[108],
		Luk:          data[109],
		Slot:         data[110],
		Rename:       data[111],
		DeleteDate:   readU32(data, 128),
		Robe:         readU16(data, 132),
		SlotChange:   readU32(data, 134),
		Rename2:      readU32(data, 138),
		Sex:          data[142],
	}
	copy(c.Name[:], data[80:104])
	copy(c.MapName[:], data[112:128])
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
type CharSelectAccept struct {
	PacketID   uint16
	PacketLen  uint16
	MaxSlots   uint8
	AvailSlots uint8
	PremSlots  uint8
	Padding    [17]byte
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
	copy(p.Padding[:], data[7:24])

	// Parse character data starting at offset 24
	charDataStart := 24
	charDataLen := int(p.PacketLen) - charDataStart
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

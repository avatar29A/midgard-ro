// Package packets defines Hercules protocol packets.
package packets

// Packet IDs for login server
const (
	// Client -> Login Server
	CA_LOGIN         uint16 = 0x0064 // Login request
	CA_REQ_HASH      uint16 = 0x01DB // Request password hash
	CA_LOGIN_HASH    uint16 = 0x01DD // Login with hash
	CA_SSO_LOGIN_REQ uint16 = 0x0825 // SSO login request
	CA_REQ_NEW_ACC   uint16 = 0x0068 // Registration request

	// Login Server -> Client
	AC_ACCEPT_LOGIN uint16 = 0x0069 // Login accepted
	AC_REFUSE_LOGIN uint16 = 0x006A // Login refused
	AC_NOTIFY_ERROR uint16 = 0x0081 // Notify error
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
	HC_NOTIFY_ZONESVR  uint16 = 0x0071 // Map server info
)

// Packet IDs for map server
const (
	// Client -> Map Server
	CZ_ENTER            uint16 = 0x0072 // Enter map
	CZ_REQUEST_MOVE     uint16 = 0x0085 // Request move
	CZ_NOTIFY_ACTORINIT uint16 = 0x007D // Loading complete

	// Map Server -> Client
	ZC_ACCEPT_ENTER      uint16 = 0x0073 // Map enter accepted
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

// CharInfo contains character information.
type CharInfo struct {
	CharID      uint32
	BaseExp     uint32
	Zeny        uint32
	JobExp      uint32
	JobLevel    uint32
	BodyState   uint32
	HealthState uint32
	EffectState uint32
	Virtue      uint32
	Honor       uint32
	StatusPoint uint16
	HP          uint32
	MaxHP       uint32
	SP          uint16
	MaxSP       uint16
	WalkSpeed   uint16
	Class       uint16
	HairStyle   uint16
	// ... many more fields
	Name         [24]byte
	Str          uint8
	Agi          uint8
	Vit          uint8
	Int          uint8
	Dex          uint8
	Luk          uint8
	Slot         uint8
	HairColor    uint16
	ClothesColor uint16
}

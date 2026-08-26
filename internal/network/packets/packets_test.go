package packets

import (
	"bytes"
	"testing"
)

func TestLoginRequestEncode(t *testing.T) {
	req := &LoginRequest{
		PacketID: CA_LOGIN,
		Version:  20220406,
		Type:     0,
	}
	copy(req.Username[:], "testuser")
	copy(req.Password[:], "testpass")

	data := req.Encode()

	if len(data) != 55 {
		t.Errorf("expected size 55, got %d", len(data))
	}

	// Check packet ID
	if data[0] != 0x64 || data[1] != 0x00 {
		t.Errorf("expected packet ID 0x0064, got %02x%02x", data[1], data[0])
	}

	// Check username starts at correct offset
	if !bytes.HasPrefix(data[6:30], []byte("testuser")) {
		t.Error("username not at correct offset")
	}

	// Check password starts at correct offset
	if !bytes.HasPrefix(data[30:54], []byte("testpass")) {
		t.Error("password not at correct offset")
	}
}

func TestCharEnterEncode(t *testing.T) {
	pkt := &CharEnter{
		PacketID:  CH_ENTER,
		AccountID: 2000001,
		LoginID1:  0x12345678,
		LoginID2:  0xABCDEF01,
		Sex:       1,
	}

	data := pkt.Encode()

	if len(data) != 17 {
		t.Errorf("expected size 17, got %d", len(data))
	}

	// Check packet ID
	if data[0] != 0x65 || data[1] != 0x00 {
		t.Errorf("expected packet ID 0x0065, got %02x%02x", data[1], data[0])
	}

	// Check account ID (little-endian)
	accountID := uint32(data[2]) | uint32(data[3])<<8 | uint32(data[4])<<16 | uint32(data[5])<<24
	if accountID != 2000001 {
		t.Errorf("expected account ID 2000001, got %d", accountID)
	}
}

func TestCharSelectEncode(t *testing.T) {
	pkt := &CharSelect{
		PacketID: CH_SELECT_CHAR,
		Slot:     2,
	}

	data := pkt.Encode()

	if len(data) != 3 {
		t.Errorf("expected size 3, got %d", len(data))
	}

	if data[0] != 0x66 || data[1] != 0x00 {
		t.Errorf("expected packet ID 0x0066, got %02x%02x", data[1], data[0])
	}

	if data[2] != 2 {
		t.Errorf("expected slot 2, got %d", data[2])
	}
}

func TestMapEnterEncode(t *testing.T) {
	pkt := &MapEnter{
		PacketID:   CZ_ENTER,
		AccountID:  2000001,
		CharID:     150001,
		LoginID1:   0x12345678,
		ClientTick: 1000,
		Sex:        0,
	}

	data := pkt.Encode()

	if len(data) != 19 {
		t.Errorf("expected size 19, got %d", len(data))
	}

	if data[0] != 0x72 || data[1] != 0x00 {
		t.Errorf("expected packet ID 0x0072, got %02x%02x", data[1], data[0])
	}
}

func TestMoveRequestEncode(t *testing.T) {
	pkt := &MoveRequest{
		PacketID: CZ_REQUEST_MOVE,
	}
	pkt.SetDestination(156, 200)

	data := pkt.Encode()

	if len(data) != 5 {
		t.Errorf("expected size 5, got %d", len(data))
	}

	if data[0] != 0x5F || data[1] != 0x03 {
		t.Errorf("expected packet ID 0x035F, got %02x%02x", data[1], data[0])
	}
}

func TestTickSendEncode(t *testing.T) {
	pkt := &TickSend{
		PacketID:   CZ_REQUEST_TIME,
		ClientTick: 0x12345678,
	}

	data := pkt.Encode()

	if len(data) != 6 {
		t.Errorf("expected size 6, got %d", len(data))
	}
	if data[0] != 0x60 || data[1] != 0x03 {
		t.Errorf("expected packet ID 0x0360, got %02x%02x", data[1], data[0])
	}
	// Tick is little-endian
	if data[2] != 0x78 || data[3] != 0x56 || data[4] != 0x34 || data[5] != 0x12 {
		t.Errorf("expected tick 0x12345678 LE, got %02x%02x%02x%02x", data[2], data[3], data[4], data[5])
	}
}

func TestDecodePlayerMove(t *testing.T) {
	// Build a synthetic ZC_NOTIFY_PLAYERMOVE: header(0x0087) + tick(4) + packed positions(6)
	// Pack (x0, y0, x1, y1) = (10, 20, 30, 40) using WBUFPOS2 layout.
	x0, y0, x1, y1 := 10, 20, 30, 40
	b := make([]byte, 12)
	b[0] = 0x87
	b[1] = 0x00
	// tick = 0xCAFEBABE little-endian
	b[2], b[3], b[4], b[5] = 0xBE, 0xBA, 0xFE, 0xCA
	// 6-byte packed positions (matches encoder used by rAthena WBUFPOS2)
	b[6] = byte(x0 >> 2)
	b[7] = byte((x0&0x03)<<6) | byte(y0>>4)
	b[8] = byte((y0&0x0F)<<4) | byte(x1>>6)
	b[9] = byte((x1&0x3F)<<2) | byte(y1>>8)
	b[10] = byte(y1 & 0xFF)
	b[11] = 0 // sub-cell positions, ignored

	mv := DecodePlayerMove(b)
	if mv == nil {
		t.Fatal("DecodePlayerMove returned nil")
	}
	if mv.StartTick != 0xCAFEBABE {
		t.Errorf("expected tick 0xCAFEBABE, got %08x", mv.StartTick)
	}
	if mv.StartX != x0 || mv.StartY != y0 {
		t.Errorf("expected start (%d,%d), got (%d,%d)", x0, y0, mv.StartX, mv.StartY)
	}
	if mv.EndX != x1 || mv.EndY != y1 {
		t.Errorf("expected end (%d,%d), got (%d,%d)", x1, y1, mv.EndX, mv.EndY)
	}
}

func TestMapAcceptDecode(t *testing.T) {
	// Test packet with position (100, 150, dir 4)
	// Position encoding in RO:
	// byte0 = x >> 2
	// byte1 = ((x & 3) << 6) | (y >> 4)
	// byte2 = ((y & 15) << 4) | dir
	x, y, dir := 100, 150, uint8(4)
	posB0 := byte(x >> 2)
	posB1 := byte(((x & 3) << 6) | (y >> 4))
	posB2 := byte(((y & 15) << 4) | int(dir))

	data := []byte{
		0x73, 0x00, // packet ID
		0x00, 0x00, 0x00, 0x00, // start time
		posB0, posB1, posB2, // position (packed)
		0x00, 0x00, // unknown
	}

	result := DecodeMapAccept(data)
	if result == nil {
		t.Fatal("DecodeMapAccept returned nil")
	}

	gotX, gotY, gotDir := result.GetPosition()
	if gotX != x {
		t.Errorf("expected x=%d, got %d", x, gotX)
	}
	if gotY != y {
		t.Errorf("expected y=%d, got %d", y, gotY)
	}
	if gotDir != dir {
		t.Errorf("expected dir=%d, got %d", dir, gotDir)
	}
}

// TestCharInfoDecode builds a record at the offsets rAthena's CHARACTER_INFO
// actually uses at PACKETVER_RE 20211103 and checks every field lands where
// the decoder expects.
//
// The values are picked so a misread is visible rather than plausible: HP and
// SP differ, and so do their maxima. The offsets were previously guessed from
// a capture and had hp reading sp and maxhp reading maxsp, so a character with
// 40 HP and 11 SP showed as 11 HP and 40 SP — two real numbers in the wrong
// places, which is exactly the kind of wrong that survives being looked at.
func TestCharInfoDecode(t *testing.T) {
	data := make([]byte, CharInfoSize)

	put16 := func(off int, v uint16) {
		data[off] = byte(v)
		data[off+1] = byte(v >> 8)
	}
	put32 := func(off int, v uint32) {
		writeU32(data, off, v)
	}
	put64 := func(off int, v uint64) {
		put32(off, uint32(v))
		put32(off+4, uint32(v>>32))
	}

	put32(0, 0x00020001) // GID
	put64(4, 1234)       // exp
	put32(12, 5000)      // money
	put64(16, 99)        // jobexp
	put32(24, 7)         // joblevel
	put16(48, 3)         // jobpoint / status point
	put64(50, 40)        // hp
	put64(58, 45)        // maxhp
	put64(66, 11)        // sp
	put64(74, 12)        // maxsp
	put16(82, 150)       // speed
	put16(84, 4)         // job
	put16(86, 2)         // head / hair style
	put16(88, 1)         // body
	put16(90, 1101)      // weapon
	put16(92, 33)        // level
	put16(94, 9)         // sppoint
	put16(104, 6)        // headpalette / hair color
	put16(106, 5)        // bodypalette / clothes color
	copy(data[108:132], "TestChar\x00")
	data[132], data[133], data[134] = 11, 12, 13 // Str Agi Vit
	data[135], data[136], data[137] = 14, 15, 16 // Int Dex Luk
	data[138] = 3                                // CharNum / slot
	copy(data[142:158], "prontera\x00")
	put32(162, 8) // robePalette
	data[174] = 1 // sex

	info := DecodeCharInfo(data)
	if info == nil {
		t.Fatal("DecodeCharInfo returned nil")
	}

	tests := []struct {
		field string
		got   uint64
		want  uint64
	}{
		{"CharID", uint64(info.CharID), 0x00020001},
		{"BaseExp", uint64(info.BaseExp), 1234},
		{"Zeny", uint64(info.Zeny), 5000},
		{"JobExp", uint64(info.JobExp), 99},
		{"JobLevel", uint64(info.JobLevel), 7},
		{"StatusPoint", uint64(info.StatusPoint), 3},
		{"HP", uint64(info.HP), 40},
		{"MaxHP", uint64(info.MaxHP), 45},
		{"SP", uint64(info.SP), 11},
		{"MaxSP", uint64(info.MaxSP), 12},
		{"WalkSpeed", uint64(info.WalkSpeed), 150},
		{"Class", uint64(info.Class), 4},
		{"HairStyle", uint64(info.HairStyle), 2},
		{"Body", uint64(info.Body), 1},
		{"Weapon", uint64(info.Weapon), 1101},
		{"BaseLevel", uint64(info.BaseLevel), 33},
		{"SkillPoint", uint64(info.SkillPoint), 9},
		{"HairColor", uint64(info.HairColor), 6},
		{"ClothesColor", uint64(info.ClothesColor), 5},
		{"Str", uint64(info.Str), 11},
		{"Agi", uint64(info.Agi), 12},
		{"Vit", uint64(info.Vit), 13},
		{"Int", uint64(info.Int), 14},
		{"Dex", uint64(info.Dex), 15},
		{"Luk", uint64(info.Luk), 16},
		{"Slot", uint64(info.Slot), 3},
		{"Robe", uint64(info.Robe), 8},
		{"Sex", uint64(info.Sex), 1},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.field, tt.got, tt.want)
		}
	}

	if info.GetName() != "TestChar" {
		t.Errorf("GetName() = %q, want \"TestChar\"", info.GetName())
	}
	if info.GetMapName() != "prontera" {
		t.Errorf("GetMapName() = %q, want \"prontera\"", info.GetMapName())
	}
}

// TestCharInfoDecodeShort makes sure a truncated record is refused rather than
// read past its end.
func TestCharInfoDecodeShort(t *testing.T) {
	if info := DecodeCharInfo(make([]byte, CharInfoSize-1)); info != nil {
		t.Errorf("DecodeCharInfo(short) = %+v, want nil", info)
	}
}

func TestMapServerInfoDecode(t *testing.T) {
	data := []byte{
		0x71, 0x00, // packet ID
		0x01, 0x00, 0x02, 0x00, // char ID
	}
	// Add map name (16 bytes)
	mapName := make([]byte, 16)
	copy(mapName, "prontera.gat")
	data = append(data, mapName...)
	// Add IP (4 bytes) - 192.168.1.100 = C0.A8.01.64
	data = append(data, 0xC0, 0xA8, 0x01, 0x64)
	// Add port (2 bytes) - 5121
	data = append(data, 0x01, 0x14)

	info := DecodeMapServerInfo(data)
	if info == nil {
		t.Fatal("DecodeMapServerInfo returned nil")
	}

	if info.CharID != 0x00020001 {
		t.Errorf("expected CharID 0x00020001, got %08x", info.CharID)
	}

	if info.GetMapName() != "prontera.gat" {
		t.Errorf("expected map 'prontera.gat', got '%s'", info.GetMapName())
	}

	if info.GetIP() != "192.168.1.100" {
		t.Errorf("expected IP '192.168.1.100', got '%s'", info.GetIP())
	}

	if info.Port != 5121 {
		t.Errorf("expected port 5121, got %d", info.Port)
	}
}

func TestLoadingCompleteEncode(t *testing.T) {
	pkt := &LoadingComplete{
		PacketID: CZ_NOTIFY_ACTORINIT,
	}

	data := pkt.Encode()

	if len(data) != 2 {
		t.Errorf("expected size 2, got %d", len(data))
	}

	if data[0] != 0x7D || data[1] != 0x00 {
		t.Errorf("expected packet ID 0x007D, got %02x%02x", data[1], data[0])
	}
}

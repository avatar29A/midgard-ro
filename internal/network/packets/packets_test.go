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

func TestCharInfoDecode(t *testing.T) {
	// Create a minimal char info packet
	data := make([]byte, CharInfoSize)

	// Set char ID
	data[0] = 0x01
	data[1] = 0x00
	data[2] = 0x02
	data[3] = 0x00

	// Set name at offset 108
	copy(data[108:132], "TestChar\x00")

	// Set slot at offset 138
	data[138] = 3

	info := DecodeCharInfo(data)
	if info == nil {
		t.Fatal("DecodeCharInfo returned nil")
	}

	if info.CharID != 0x00020001 {
		t.Errorf("expected CharID 0x00020001, got %08x", info.CharID)
	}

	if info.GetName() != "TestChar" {
		t.Errorf("expected name 'TestChar', got '%s'", info.GetName())
	}

	if info.Slot != 3 {
		t.Errorf("expected slot 3, got %d", info.Slot)
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

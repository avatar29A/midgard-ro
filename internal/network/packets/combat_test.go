package packets

import (
	"encoding/binary"
	"testing"
)

// TestEncodeAttack builds the request the server parses at offsets 2 and 6.
func TestEncodeAttack(t *testing.T) {
	pkt := EncodeAttack(2000042, true)

	if len(pkt) != 7 {
		t.Fatalf("built %d bytes, want 7", len(pkt))
	}
	if got := binary.LittleEndian.Uint16(pkt); got != CZ_REQUEST_ACT {
		t.Errorf("id = 0x%04X, want 0x%04X", got, CZ_REQUEST_ACT)
	}
	if got := binary.LittleEndian.Uint32(pkt[2:]); got != 2000042 {
		t.Errorf("target = %d, want 2000042", got)
	}
	if pkt[6] != ActionAttackRepeat {
		t.Errorf("action = %d, want the repeating attack", pkt[6])
	}

	if once := EncodeAttack(1, false); once[6] != ActionAttackOnce {
		t.Errorf("action = %d, want the single attack", once[6])
	}
}

// TestDecodeDamage reads a blow, with the fields at the offsets the 34-byte
// struct puts them: the amount is past two speeds, and the type is past a
// one-byte flag and a two-byte hit count.
func TestDecodeDamage(t *testing.T) {
	pkt := make([]byte, 34)
	binary.LittleEndian.PutUint16(pkt, ZC_NOTIFY_ACT)
	binary.LittleEndian.PutUint32(pkt[2:], 2000000)   // source
	binary.LittleEndian.PutUint32(pkt[6:], 110000123) // target
	binary.LittleEndian.PutUint32(pkt[14:], 800)      // source speed
	binary.LittleEndian.PutUint32(pkt[18:], 400)      // damage delay
	binary.LittleEndian.PutUint32(pkt[22:], 37)       // damage
	pkt[26] = 0
	binary.LittleEndian.PutUint16(pkt[27:], 1) // hits
	pkt[29] = DamageCritical
	binary.LittleEndian.PutUint32(pkt[30:], 0)

	blow, ok := DecodeDamage(pkt)
	if !ok {
		t.Fatal("a 34-byte blow should decode")
	}
	if blow.SourceID != 2000000 || blow.TargetID != 110000123 {
		t.Errorf("source/target = %d/%d", blow.SourceID, blow.TargetID)
	}
	if blow.Amount != 37 {
		t.Errorf("Amount = %d, want 37", blow.Amount)
	}
	if blow.Hits != 1 {
		t.Errorf("Hits = %d, want 1", blow.Hits)
	}
	if !blow.Critical() {
		t.Error("a critical blow did not read as one")
	}
	if blow.Missed() {
		t.Error("a blow for 37 read as a miss")
	}
	if blow.SourceSpeed != 800 || blow.DamageDelay != 400 {
		t.Errorf("speeds = %d/%d, want 800/400", blow.SourceSpeed, blow.DamageDelay)
	}
}

// TestDecodeDamageMiss: zero damage is a miss, which the original draws
// rather than showing a nought.
func TestDecodeDamageMiss(t *testing.T) {
	pkt := make([]byte, 34)
	binary.LittleEndian.PutUint16(pkt, ZC_NOTIFY_ACT)
	binary.LittleEndian.PutUint32(pkt[22:], 0)

	blow, ok := DecodeDamage(pkt)
	if !ok {
		t.Fatal("should decode")
	}
	if !blow.Missed() {
		t.Error("zero damage did not read as a miss")
	}
}

// TestDecodeDamageShort: anything under the full 34 bytes is refused rather
// than read past the end.
func TestDecodeDamageShort(t *testing.T) {
	if _, ok := DecodeDamage(make([]byte, 33)); ok {
		t.Error("a 33-byte blow decoded, but the packet is 34")
	}
}

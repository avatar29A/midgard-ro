package packets

// RO packs map coordinates into bit fields rather than whole bytes, because a
// cell coordinate needs 10 bits and a facing 4. The two layouts below are
// rAthena's WBUFPOS and WBUFPOS2 (src/map/clif.cpp) read back.
//
// Getting a shift wrong here does not fail loudly: the coordinates stay in
// range and the entity simply appears somewhere else, which reads as a
// movement bug rather than a decoding one.

// PosSize is the wire size of a packed position with facing.
const PosSize = 3

// Pos2Size is the wire size of a packed movement, which carries a start and an
// end cell plus sub-cell offsets.
const Pos2Size = 6

// DecodePos unpacks a 3-byte position: 10 bits of x, 10 of y, 4 of direction.
//
//	p[0] = x >> 2
//	p[1] = (x << 6) | ((y >> 4) & 0x3f)
//	p[2] = (y << 4) | (dir & 0xf)
func DecodePos(p []byte) (x, y, dir int) {
	if len(p) < PosSize {
		return 0, 0, 0
	}
	x = int(p[0])<<2 | int(p[1])>>6
	y = (int(p[1])&0x3F)<<4 | int(p[2])>>4
	dir = int(p[2]) & 0x0F
	return x, y, dir
}

// DecodePos2 unpacks a 6-byte movement: the cell moved from, the cell moved
// to, and a sub-cell offset the client uses to start the walk part-way into a
// cell.
//
//	p[0] = x0 >> 2
//	p[1] = (x0 << 6) | ((y0 >> 4) & 0x3f)
//	p[2] = (y0 << 4) | ((x1 >> 6) & 0x0f)
//	p[3] = (x1 << 2) | ((y1 >> 8) & 0x03)
//	p[4] = y1
//	p[5] = (sx0 << 4) | (sy0 & 0x0f)
func DecodePos2(p []byte) (x0, y0, x1, y1, sx0, sy0 int) {
	if len(p) < Pos2Size {
		return 0, 0, 0, 0, 0, 0
	}
	x0 = int(p[0])<<2 | int(p[1])>>6
	y0 = (int(p[1])&0x3F)<<4 | int(p[2])>>4
	x1 = (int(p[2])&0x0F)<<6 | int(p[3])>>2
	y1 = (int(p[3])&0x03)<<8 | int(p[4])
	sx0 = int(p[5]) >> 4
	sy0 = int(p[5]) & 0x0F
	return x0, y0, x1, y1, sx0, sy0
}

// EncodePos packs a position the way the server does. Only the tests need
// this, but they need it to be the server's layout rather than the decoder's
// assumptions, so it is written from rAthena's WBUFPOS rather than derived
// from DecodePos.
func EncodePos(x, y, dir int) []byte {
	return []byte{
		byte(x >> 2),
		byte(x<<6 | (y>>4)&0x3F),
		byte(y<<4 | dir&0x0F),
	}
}

// EncodePos2 packs a movement the way the server does. See EncodePos.
func EncodePos2(x0, y0, x1, y1, sx0, sy0 int) []byte {
	return []byte{
		byte(x0 >> 2),
		byte(x0<<6 | (y0>>4)&0x3F),
		byte(y0<<4 | (x1>>6)&0x0F),
		byte(x1<<2 | (y1>>8)&0x03),
		byte(y1),
		byte(sx0<<4 | sy0&0x0F),
	}
}

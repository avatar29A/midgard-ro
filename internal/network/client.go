// Package network handles communication with Hercules servers.
package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// Net is the trace channel for packet framing. Aliased here so callers don't
// need to import the trace package just to name it.
const Net = trace.Net

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ServerType represents the type of server.
type ServerType int

const (
	ServerLogin ServerType = iota
	ServerChar
	ServerMap
)

// readBufferSize is the size of the read buffer.
const readBufferSize = 65536

// Client handles network communication.
type Client struct {
	conn     net.Conn
	mu       sync.Mutex
	handlers map[uint16]PacketHandler

	// Inbound bytes, filled by a reader goroutine and drained by Process on
	// the game thread. Only the raw bytes cross the boundary: framing and
	// handler dispatch stay on the game thread, so handlers keep their
	// single-threaded guarantees.
	rx     chan []byte
	rxErr  chan error
	rxStop chan struct{}

	// Connection state
	connected  bool
	serverType ServerType

	// Read buffer for packet assembly
	readBuf    []byte
	readOffset int

	// Session info
	accountID uint32
	loginID1  uint32
	loginID2  uint32
	sex       uint8

	// Character info (set after char select)
	charID uint32

	// Auth token for modern rAthena (from AC_ACCEPT_LOGIN2)
	authToken [17]byte

	// Protocol quirk: char server sends account ID prefix
	charServerAccountIDReceived bool

	// Telemetry — exposed via Stats() for the debug overlay.
	lastSentID   uint16
	lastSentAt   time.Time
	lastSentLen  int
	lastRecvID   uint16
	lastRecvAt   time.Time
	lastRecvLen  int
	packetsSent  uint64
	packetsRecvd uint64
	bytesSent    uint64
	bytesRecvd   uint64
}

// Stats is a point-in-time snapshot of network telemetry.
type Stats struct {
	LastSentID   uint16
	LastSentAt   time.Time
	LastSentLen  int
	LastRecvID   uint16
	LastRecvAt   time.Time
	LastRecvLen  int
	PacketsSent  uint64
	PacketsRecvd uint64
	BytesSent    uint64
	BytesRecvd   uint64
}

// Stats returns a snapshot of network telemetry counters.
func (c *Client) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		LastSentID:   c.lastSentID,
		LastSentAt:   c.lastSentAt,
		LastSentLen:  c.lastSentLen,
		LastRecvID:   c.lastRecvID,
		LastRecvAt:   c.lastRecvAt,
		LastRecvLen:  c.lastRecvLen,
		PacketsSent:  c.packetsSent,
		PacketsRecvd: c.packetsRecvd,
		BytesSent:    c.bytesSent,
		BytesRecvd:   c.bytesRecvd,
	}
}

// PacketHandler handles incoming packets.
type PacketHandler func(data []byte) error

// New creates a new network client.
func New() *Client {
	return &Client{
		handlers: make(map[uint16]PacketHandler),
		readBuf:  make([]byte, readBufferSize),
	}
}

// Connect connects to a server.
func (c *Client) Connect(host string, port int, serverType ServerType) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Info("connecting to server", zap.String("addr", addr), zap.Int("type", int(serverType)))

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		logger.Error("connection failed", zap.String("addr", addr), zap.Error(err))
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}

	c.conn = conn
	c.connected = true
	c.serverType = serverType
	c.readOffset = 0                      // Reset read buffer
	c.charServerAccountIDReceived = false // Reset for new connection

	// Hand the socket to a reader goroutine. Reading on the game thread means
	// choosing between blocking it and polling it, and both cost frame time
	// for nothing: a blocking read parks the loop whenever the server is idle,
	// while a poll adds up to a frame of latency to every packet. Off-thread,
	// the read blocks as long as it likes and Process just drains what arrived.
	c.rx = make(chan []byte, 64)
	c.rxErr = make(chan error, 1)
	c.rxStop = make(chan struct{})
	go readLoop(conn, c.rx, c.rxErr, c.rxStop)

	logger.Info("connected to server", zap.String("addr", addr))
	return nil
}

// readLoop pulls bytes off the socket until it is closed or asked to stop.
//
// It takes its channels as arguments rather than reading them off the Client
// so that a reconnect cannot leave an old goroutine writing into the new
// connection's buffers.
func readLoop(conn net.Conn, rx chan<- []byte, errc chan<- error, stop <-chan struct{}) {
	buf := make([]byte, readBufferSize)

	for {
		n, err := conn.Read(buf)

		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case rx <- chunk:
			case <-stop:
				return
			}
		}

		if err != nil {
			select {
			case errc <- err:
			case <-stop:
			default: // an error is already queued; the first one is the useful one
			}
			return
		}
	}
}

// Disconnect closes the connection.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Signal first, then close: the reader is parked in Read, and closing the
	// socket is what wakes it up.
	if c.rxStop != nil {
		close(c.rxStop)
		c.rxStop = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.rx = nil
	c.rxErr = nil
	c.connected = false
}

// IsConnected returns connection status.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// RegisterHandler registers a packet handler.
func (c *Client) RegisterHandler(packetID uint16, handler PacketHandler) {
	c.handlers[packetID] = handler
}

// Send sends a packet to the server.
func (c *Client) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	if len(data) >= 2 {
		packetID := binary.LittleEndian.Uint16(data[0:2])
		logger.Debug("sending packet", zap.String("id", fmt.Sprintf("0x%04X", packetID)), zap.Int("len", len(data)))
		c.lastSentID = packetID
		c.lastSentAt = time.Now()
		c.lastSentLen = len(data)
	}

	n, err := c.conn.Write(data)
	if err != nil {
		logger.Error("send failed", zap.Error(err))
	}
	c.packetsSent++
	c.bytesSent += uint64(n)
	return err
}

// Process reads and processes incoming packets.
// Should be called regularly in the game loop.
func (c *Client) Process() (err error) {
	// Recover from any panics in packet processing to prevent crashes
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic in network processing",
				zap.Any("panic", r),
				zap.Int("readOffset", c.readOffset),
				zap.String("stack", stack))
			err = fmt.Errorf("panic in network processing: %v", r)
		}
	}()

	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil
	}
	rx, rxErr := c.rx, c.rxErr
	c.mu.Unlock()

	if rx == nil {
		return nil
	}

	// Drain whatever the reader has queued. This never blocks: if nothing has
	// arrived we fall straight through to framing what is already buffered.
	for draining := true; draining; {
		select {
		case chunk := <-rx:
			if len(chunk) > len(c.readBuf)-c.readOffset {
				// The buffer holds a whole packet many times over, so this
				// means framing has stopped consuming and the connection is
				// unrecoverable. Say so rather than silently truncating.
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()
				return fmt.Errorf("read buffer full: %d buffered, %d more arrived",
					c.readOffset, len(chunk))
			}
			copy(c.readBuf[c.readOffset:], chunk)
			c.readOffset += len(chunk)

			logger.Debug("received raw data", zap.Int("bytes", len(chunk)))

		case err := <-rxErr:
			// Frame whatever arrived before the error before reporting it —
			// the last packets are often the ones explaining the disconnect.
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			if err == io.EOF {
				return fmt.Errorf("connection closed by server")
			}
			return fmt.Errorf("read error: %w", err)

		default:
			draining = false
		}
	}

	// Process complete packets
	for c.readOffset >= 2 {
		// Handle char server account ID prefix quirk
		// Modern rAthena char servers send account ID (4 bytes) before packets
		if c.serverType == ServerChar && !c.charServerAccountIDReceived && c.readOffset >= 4 {
			possibleAccountID := binary.LittleEndian.Uint32(c.readBuf[0:4])
			if possibleAccountID == c.accountID {
				logger.Debug("skipping char server account ID prefix", zap.Uint32("accountID", possibleAccountID))
				copy(c.readBuf, c.readBuf[4:c.readOffset])
				c.readOffset -= 4
				c.charServerAccountIDReceived = true
				if c.readOffset < 2 {
					break
				}
			}
		}

		// Read packet ID
		packetID := binary.LittleEndian.Uint16(c.readBuf[0:2])

		// Determine packet length
		packetLen, known := c.packetLength(packetID, c.readBuf[:c.readOffset])
		logger.Debug("parsing packet", zap.String("id", fmt.Sprintf("0x%04X", packetID)), zap.Int("len", packetLen), zap.Int("available", c.readOffset))

		if known && packetLen == 0 {
			// Variable-length packet whose header hasn't arrived yet.
			break
		}

		if !known {
			// An id we have no length for. We cannot know where the next
			// packet starts, so scan forward for a boundary that looks real
			// rather than guessing this one's size.
			skip := resyncOffset(c.readBuf[:c.readOffset])

			logger.Warn("unknown packet id, resynchronising",
				zap.String("id", fmt.Sprintf("0x%04X", packetID)),
				zap.Int("skipped", skip),
				zap.Int("buffered", c.readOffset))
			trace.Emit(Net, "unknown-packet",
				zap.String("id", fmt.Sprintf("0x%04X", packetID)),
				zap.Int("buffered", c.readOffset),
				zap.Int("skipped", skip),
				zap.String("head", fmt.Sprintf("% X", c.readBuf[:minInt(c.readOffset, 24)])))

			if skip >= c.readOffset {
				c.readOffset = 0
				break
			}
			copy(c.readBuf, c.readBuf[skip:c.readOffset])
			c.readOffset -= skip
			continue
		}

		if c.readOffset < packetLen {
			// Not enough data yet
			break
		}

		// Extract complete packet
		packetData := make([]byte, packetLen)
		copy(packetData, c.readBuf[:packetLen])

		// Shift remaining data
		copy(c.readBuf, c.readBuf[packetLen:c.readOffset])
		c.readOffset -= packetLen

		// Dispatch to handler
		logger.Debug("received packet", zap.String("id", fmt.Sprintf("0x%04X", packetID)), zap.Int("len", packetLen))
		c.mu.Lock()
		c.lastRecvID = packetID
		c.lastRecvAt = time.Now()
		c.lastRecvLen = packetLen
		c.packetsRecvd++
		c.bytesRecvd += uint64(packetLen)
		c.mu.Unlock()
		trace.Emit(Net, "recv",
			zap.String("id", fmt.Sprintf("0x%04X", packetID)),
			zap.Int("len", packetLen),
			zap.Int("remaining", c.readOffset))

		if handler, ok := c.handlers[packetID]; ok {
			if err := handler(packetData); err != nil {
				logger.Error("packet handler error", zap.String("id", fmt.Sprintf("0x%04X", packetID)), zap.Error(err))
				return fmt.Errorf("packet %04x handler: %w", packetID, err)
			}
		} else {
			logger.Debug("no handler for packet", zap.String("id", fmt.Sprintf("0x%04X", packetID)))
			trace.Emit(Net, "unhandled", zap.String("id", fmt.Sprintf("0x%04X", packetID)))
		}
	}

	return nil
}

// packetLength returns the wire length of a packet, and whether the id is one
// we know at all.
//
// Lengths come from internal/network/packets, generated from the rAthena tree
// we build the server from (tools/packetlen/gen.py). This used to be a
// hand-written switch that fell back to reading bytes 2-4 as a length for
// anything it didn't recognize. That fallback is why walking broke: RO frames
// packets by length alone, so misreading one fixed-length packet's payload as
// a length consumed the wrong number of bytes and turned every subsequent
// packet — the walk acknowledgements included — into garbage.
//
// A returned length of 0 with known=true means "need more bytes before the
// length can be determined".
func (c *Client) packetLength(packetID uint16, data []byte) (length int, known bool) {
	size, ok := packets.Length(packetID)
	if !ok {
		return 0, false
	}

	if size == packets.VariableLength {
		if len(data) < 4 {
			return 0, true // header not fully buffered yet
		}
		declared := int(binary.LittleEndian.Uint16(data[2:4]))
		if declared < 4 {
			// A variable-length packet must at least cover id + length.
			return 0, false
		}
		return declared, true
	}

	return size, true
}

// resyncOffset scans forward for the next plausible packet boundary after a
// stream desync, returning the number of bytes to discard.
//
// It advances one byte at a time on purpose. The previous code skipped two,
// which preserves parity — so an odd-byte misalignment (the common case, since
// most payloads have odd-length fields) could never recover no matter how much
// data went past.
func resyncOffset(buf []byte) int {
	for offset := 1; offset+2 <= len(buf); offset++ {
		if packets.IsKnown(binary.LittleEndian.Uint16(buf[offset : offset+2])) {
			return offset
		}
	}
	return len(buf)
}

// SetSession sets session information from login.
func (c *Client) SetSession(accountID, loginID1, loginID2 uint32, sex uint8) {
	c.accountID = accountID
	c.loginID1 = loginID1
	c.loginID2 = loginID2
	c.sex = sex
}

// SetAuthToken sets the auth token from modern login (AC_ACCEPT_LOGIN2).
func (c *Client) SetAuthToken(token []byte) {
	copy(c.authToken[:], token)
}

// AuthToken returns the auth token.
func (c *Client) AuthToken() [17]byte {
	return c.authToken
}

// Session returns current session info.
func (c *Client) Session() (accountID, loginID1, loginID2 uint32, sex uint8) {
	return c.accountID, c.loginID1, c.loginID2, c.sex
}

// SetCharID sets the selected character ID.
func (c *Client) SetCharID(charID uint32) {
	c.charID = charID
}

// CharID returns the selected character ID.
func (c *Client) CharID() uint32 {
	return c.charID
}

// ServerType returns the current server type.
func (c *Client) ServerType() ServerType {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverType
}

// Helper functions for packet building

// WriteUint16 writes a uint16 in little-endian format.
func WriteUint16(buf []byte, offset int, v uint16) {
	binary.LittleEndian.PutUint16(buf[offset:], v)
}

// WriteUint32 writes a uint32 in little-endian format.
func WriteUint32(buf []byte, offset int, v uint32) {
	binary.LittleEndian.PutUint32(buf[offset:], v)
}

// ReadUint16 reads a uint16 in little-endian format.
func ReadUint16(buf []byte, offset int) uint16 {
	return binary.LittleEndian.Uint16(buf[offset:])
}

// ReadUint32 reads a uint32 in little-endian format.
func ReadUint32(buf []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(buf[offset:])
}

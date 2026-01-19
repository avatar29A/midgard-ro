// Package network handles communication with Hercules servers.
package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"go.uber.org/zap"
)

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
	c.readOffset = 0 // Reset read buffer
	c.charServerAccountIDReceived = false // Reset for new connection

	logger.Info("connected to server", zap.String("addr", addr))
	return nil
}

// Disconnect closes the connection.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
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
	}

	_, err := c.conn.Write(data)
	if err != nil {
		logger.Error("send failed", zap.Error(err))
	}
	return err
}

// Process reads and processes incoming packets.
// Should be called regularly in the game loop.
func (c *Client) Process() error {
	c.mu.Lock()
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		return nil
	}
	conn := c.conn
	c.mu.Unlock()

	// Set short read deadline for non-blocking behavior
	conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))

	// Read available data
	n, err := conn.Read(c.readBuf[c.readOffset:])
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// No data available, that's fine
			return nil
		}
		if err == io.EOF {
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return fmt.Errorf("connection closed by server")
		}
		return fmt.Errorf("read error: %w", err)
	}

	if n > 0 {
		logger.Debug("received raw data", zap.Int("bytes", n), zap.String("hex", fmt.Sprintf("%X", c.readBuf[c.readOffset:c.readOffset+min(n, 32)])))
	}
	c.readOffset += n

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
		packetLen := c.getPacketLength(packetID, c.readBuf[:c.readOffset])
		logger.Debug("parsing packet", zap.String("id", fmt.Sprintf("0x%04X", packetID)), zap.Int("len", packetLen), zap.Int("available", c.readOffset))
		if packetLen == 0 {
			// Unknown packet - if we have less than 32 bytes of unknown data,
			// it's likely garbage from a previous packet, so flush it
			if c.readOffset < 32 {
				logger.Debug("flushing unknown packet data", zap.Int("bytes", c.readOffset))
				c.readOffset = 0
				break
			}
			// Otherwise, skip 2 bytes and try again
			copy(c.readBuf, c.readBuf[2:c.readOffset])
			c.readOffset -= 2
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
		if handler, ok := c.handlers[packetID]; ok {
			if err := handler(packetData); err != nil {
				logger.Error("packet handler error", zap.String("id", fmt.Sprintf("0x%04X", packetID)), zap.Error(err))
				return fmt.Errorf("packet %04x handler: %w", packetID, err)
			}
		} else {
			logger.Debug("no handler for packet", zap.String("id", fmt.Sprintf("0x%04X", packetID)))
		}
	}

	return nil
}

// getPacketLength returns the length of a packet based on its ID.
// Returns 0 for unknown packets.
func (c *Client) getPacketLength(packetID uint16, data []byte) int {
	// Variable-length packets have length in bytes 2-4
	switch packetID {
	// Login server packets
	case 0x0069: // AC_ACCEPT_LOGIN (variable, old)
		if len(data) >= 4 {
			return int(binary.LittleEndian.Uint16(data[2:4]))
		}
		return 0
	case 0x0AC4: // AC_ACCEPT_LOGIN2 (variable, modern rAthena)
		if len(data) >= 4 {
			return int(binary.LittleEndian.Uint16(data[2:4]))
		}
		return 0
	case 0x006A: // AC_REFUSE_LOGIN (old)
		return 23
	case 0x0081: // AC_NOTIFY_ERROR
		return 3
	case 0x083E: // AC_REFUSE_LOGIN2 (modern)
		return 26

	// Character server packets
	case 0x006B: // HC_ACCEPT_ENTER (variable)
		if len(data) >= 4 {
			return int(binary.LittleEndian.Uint16(data[2:4]))
		}
		return 0
	case 0x006C: // HC_REFUSE_ENTER
		return 3
	case 0x006D: // HC_ACCEPT_MAKECHAR
		return 155 + 2
	case 0x0071: // HC_NOTIFY_ZONESVR
		return 28
	case 0x0AC5: // HC_NOTIFY_ZONESVR2 (modern rAthena)
		return 28

	// Map server packets
	case 0x0073: // ZC_ACCEPT_ENTER
		return 11
	case 0x02EB: // ZC_ACCEPT_ENTER2 (modern rAthena)
		return 13
	case 0x0283: // Entity ID confirmation
		return 6
	case 0x0B18: // Unknown inventory-related packet
		return 4
	case 0x0078: // ZC_NOTIFY_STANDENTRY
		return 54
	case 0x007B: // ZC_NOTIFY_MOVEENTRY
		return 60
	case 0x008A: // ZC_NOTIFY_ACT
		return 29
	case 0x0091: // ZC_NPCACK_MAPMOVE
		return 22

	// Keep-alive
	case 0x007F: // Server tick
		return 6

	default:
		// For unknown packets, try to read length from packet header
		// Only do this if length seems reasonable
		if len(data) >= 4 {
			possibleLen := int(binary.LittleEndian.Uint16(data[2:4]))
			// Sanity check: length should be reasonable (4 bytes min, 1KB max for unknown packets)
			// Known variable packets are explicitly handled above
			if possibleLen >= 4 && possibleLen <= 1024 {
				return possibleLen
			}
		}
		return 0
	}
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

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Package network handles communication with Hercules servers.
package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
)

// ServerType represents the type of server.
type ServerType int

const (
	ServerLogin ServerType = iota
	ServerChar
	ServerMap
)

// Client handles network communication.
type Client struct {
	conn     net.Conn
	mu       sync.Mutex
	handlers map[uint16]PacketHandler

	// Connection state
	connected  bool
	serverType ServerType

	// Session info
	accountID uint32
	loginID1  uint32
	loginID2  uint32
	sex       uint8
}

// PacketHandler handles incoming packets.
type PacketHandler func(data []byte) error

// New creates a new network client.
func New() *Client {
	return &Client{
		handlers: make(map[uint16]PacketHandler),
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
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}

	c.conn = conn
	c.connected = true
	c.serverType = serverType

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

	_, err := c.conn.Write(data)
	return err
}

// Process reads and processes incoming packets.
// Should be called regularly in the game loop.
func (c *Client) Process() error {
	if !c.connected {
		return nil
	}

	// TODO: Non-blocking read
	// TODO: Parse packet header
	// TODO: Read packet body
	// TODO: Dispatch to handler

	return nil
}

// SetSession sets session information from login.
func (c *Client) SetSession(accountID, loginID1, loginID2 uint32, sex uint8) {
	c.accountID = accountID
	c.loginID1 = loginID1
	c.loginID2 = loginID2
	c.sex = sex
}

// Session returns current session info.
func (c *Client) Session() (accountID, loginID1, loginID2 uint32, sex uint8) {
	return c.accountID, c.loginID1, c.loginID2, c.sex
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

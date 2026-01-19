// Package states implements game state management.
package states

import (
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
)

// LoginStateConfig contains configuration for the login state.
type LoginStateConfig struct {
	ServerHost    string
	ServerPort    int
	Username      string
	Password      string
	ClientVersion uint32
}

// LoginState handles the login screen and authentication.
type LoginState struct {
	config     LoginStateConfig
	client     *network.Client
	manager    *Manager

	// UI state
	Username   string
	Password   string
	ErrorMsg   string
	IsLoading  bool

	// Connection state
	connected  bool
	loginSent  bool
}

// NewLoginState creates a new login state.
func NewLoginState(cfg LoginStateConfig, client *network.Client, manager *Manager) *LoginState {
	return &LoginState{
		config:  cfg,
		client:  client,
		manager: manager,
		Username: cfg.Username,
		Password: cfg.Password,
	}
}

// Enter is called when entering this state.
func (s *LoginState) Enter() error {
	s.ErrorMsg = ""
	s.IsLoading = false
	s.connected = false
	s.loginSent = false

	// Register packet handlers (both old and modern versions)
	s.client.RegisterHandler(packets.AC_ACCEPT_LOGIN, s.handleLoginAccept)
	s.client.RegisterHandler(packets.AC_ACCEPT_LOGIN2, s.handleLoginAccept2)
	s.client.RegisterHandler(packets.AC_REFUSE_LOGIN, s.handleLoginRefuse)
	s.client.RegisterHandler(packets.AC_REFUSE_LOGIN2, s.handleLoginRefuse2)
	s.client.RegisterHandler(packets.AC_NOTIFY_ERROR, s.handleNotifyError)

	return nil
}

func (s *LoginState) handleNotifyError(data []byte) error {
	s.IsLoading = false

	errorCode := byte(0)
	if len(data) >= 3 {
		errorCode = data[2]
	}

	switch errorCode {
	case 1:
		s.ErrorMsg = "Server closed"
	case 2:
		s.ErrorMsg = "Someone already logged in with this ID"
	case 3:
		s.ErrorMsg = "Timeout"
	case 4:
		s.ErrorMsg = "Server full"
	case 5:
		s.ErrorMsg = "IP blocked"
	case 8:
		s.ErrorMsg = "Too many connections. Please wait."
	default:
		s.ErrorMsg = fmt.Sprintf("Server error: %d", errorCode)
	}
	return nil
}

// Exit is called when leaving this state.
func (s *LoginState) Exit() error {
	return nil
}

// Update is called every frame.
func (s *LoginState) Update(dt float64) error {
	// Process network packets
	if err := s.client.Process(); err != nil {
		s.ErrorMsg = fmt.Sprintf("Network error: %v", err)
		s.IsLoading = false
	}

	return nil
}

// Render is called every frame to draw the state.
func (s *LoginState) Render() error {
	// UI rendering will be handled by the UI system
	return nil
}

// HandleInput processes input events.
func (s *LoginState) HandleInput(event interface{}) error {
	return nil
}

// AttemptLogin attempts to connect and login to the server.
func (s *LoginState) AttemptLogin() error {
	if s.IsLoading {
		return nil
	}

	s.ErrorMsg = ""
	s.IsLoading = true

	// Connect if not already connected
	if !s.client.IsConnected() {
		err := s.client.Connect(s.config.ServerHost, s.config.ServerPort, network.ServerLogin)
		if err != nil {
			s.ErrorMsg = fmt.Sprintf("Connection failed: %v", err)
			s.IsLoading = false
			return err
		}
		s.connected = true
	}

	// Send login request
	return s.sendLoginRequest()
}

func (s *LoginState) sendLoginRequest() error {
	// Build login packet
	req := &packets.LoginRequest{
		PacketID: packets.CA_LOGIN,
		Version:  s.config.ClientVersion,
		Type:     0, // Client type
	}

	// Copy username and password (null-terminated)
	copy(req.Username[:], s.Username)
	copy(req.Password[:], s.Password)

	// Send packet
	if err := s.client.Send(req.Encode()); err != nil {
		s.ErrorMsg = fmt.Sprintf("Send failed: %v", err)
		s.IsLoading = false
		return err
	}

	s.loginSent = true
	return nil
}

func (s *LoginState) handleLoginAccept(data []byte) error {
	s.IsLoading = false

	// Parse login accept packet (AC_ACCEPT_LOGIN 0x0069)
	// Format: packet_id(2) + packet_len(2) + login_id1(4) + account_id(4) + login_id2(4)
	//         + unknown(4) + sex(1) + unknown(2) + char_servers[]
	if len(data) < 47 {
		s.ErrorMsg = "Invalid login response"
		return fmt.Errorf("login accept packet too short: %d", len(data))
	}

	packetLen := network.ReadUint16(data, 2)
	loginID1 := network.ReadUint32(data, 4)
	accountID := network.ReadUint32(data, 8)
	loginID2 := network.ReadUint32(data, 12)
	// bytes 16-19: unknown
	sex := data[20]

	// Store session
	s.client.SetSession(accountID, loginID1, loginID2, sex)

	// Parse character server list (starts at offset 47)
	// Each server entry is 32 bytes
	charServerStart := 47
	charServerSize := 32
	numServers := (int(packetLen) - charServerStart) / charServerSize

	if numServers < 1 {
		s.ErrorMsg = "No character servers available"
		return fmt.Errorf("no character servers in response")
	}

	// Get first character server
	serverData := data[charServerStart : charServerStart+charServerSize]
	ip := network.ReadUint32(serverData, 0)
	port := network.ReadUint16(serverData, 4)

	// Convert IP to string (little-endian)
	charServerIP := fmt.Sprintf("%d.%d.%d.%d",
		byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
	charServerPort := int(port)

	// Disconnect from login server before connecting to char server
	s.client.Disconnect()

	// Transition to connecting state with char server info
	s.manager.Change(NewConnectingState(ConnectingStateConfig{
		NextState:  "charselect",
		ServerHost: charServerIP,
		ServerPort: charServerPort,
	}, s.client, s.manager))

	return nil
}

func (s *LoginState) handleLoginRefuse(data []byte) error {
	s.IsLoading = false

	// Parse error code
	if len(data) < 3 {
		s.ErrorMsg = "Login refused"
		return nil
	}

	errorCode := data[2]
	s.setLoginError(errorCode)
	return nil
}

func (s *LoginState) handleLoginRefuse2(data []byte) error {
	s.IsLoading = false

	// Modern packet: 0x083E - error code at offset 2
	if len(data) < 3 {
		s.ErrorMsg = "Login refused"
		return nil
	}

	errorCode := data[2]
	s.setLoginError(errorCode)
	return nil
}

func (s *LoginState) setLoginError(errorCode byte) {
	switch errorCode {
	case 0:
		s.ErrorMsg = "Unregistered ID"
	case 1:
		s.ErrorMsg = "Incorrect password"
	case 2:
		s.ErrorMsg = "ID expired"
	case 3:
		s.ErrorMsg = "Server rejected connection"
	case 4:
		s.ErrorMsg = "Server is full"
	case 5:
		s.ErrorMsg = "Banned"
	case 6:
		s.ErrorMsg = "Server under maintenance"
	case 7:
		s.ErrorMsg = "Server overloaded"
	case 8:
		s.ErrorMsg = "No more connections allowed"
	case 9:
		s.ErrorMsg = "IP banned"
	case 10:
		s.ErrorMsg = "Locked for security"
	default:
		s.ErrorMsg = fmt.Sprintf("Login error: %d", errorCode)
	}
}

// handleLoginAccept2 handles AC_ACCEPT_LOGIN2 (0x0AC4) - modern rAthena format
func (s *LoginState) handleLoginAccept2(data []byte) error {
	s.IsLoading = false

	// Modern packet format (0x0AC4):
	// packet_id(2) + packet_len(2) + login_id1(4) + account_id(4) + login_id2(4)
	// + ip(4) + last_login(26) + sex(1) + auth_token(17) + char_servers[]
	// Offsets: 0-1=id, 2-3=len, 4-7=loginID1, 8-11=accountID, 12-15=loginID2,
	//          16-19=ip, 20-45=last_login(26), 46=sex, 47-63=auth_token(17), 64+=servers
	if len(data) < 64 {
		s.ErrorMsg = "Invalid login response"
		return fmt.Errorf("login accept2 packet too short: %d", len(data))
	}

	packetLen := network.ReadUint16(data, 2)
	loginID1 := network.ReadUint32(data, 4)
	accountID := network.ReadUint32(data, 8)
	loginID2 := network.ReadUint32(data, 12)
	// bytes 16-19: IP (unused)
	// bytes 20-45: last login time (26 bytes)
	sex := data[46]
	// bytes 47-63: auth token (17 bytes)
	authToken := data[47:64]

	// Store session
	s.client.SetSession(accountID, loginID1, loginID2, sex)
	s.client.SetAuthToken(authToken)

	// Parse character server list (starts at offset 64)
	// Each server entry is 32 bytes: IP(4) + port(2) + name(20) + users(2) + state(2) + property(2)
	charServerStart := 64
	charServerSize := 32
	numServers := (int(packetLen) - charServerStart) / charServerSize

	if numServers < 1 {
		s.ErrorMsg = "No character servers available"
		return fmt.Errorf("no character servers in response")
	}

	// Get first character server
	serverData := data[charServerStart : charServerStart+charServerSize]
	ip := network.ReadUint32(serverData, 0)
	port := network.ReadUint16(serverData, 4)

	// Convert IP to string (little-endian)
	charServerIP := fmt.Sprintf("%d.%d.%d.%d",
		byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
	charServerPort := int(port)

	// Disconnect from login server before connecting to char server
	s.client.Disconnect()

	// Transition to connecting state with char server info
	s.manager.Change(NewConnectingState(ConnectingStateConfig{
		NextState:  "charselect",
		ServerHost: charServerIP,
		ServerPort: charServerPort,
	}, s.client, s.manager))

	return nil
}

// GetUsername returns the current username.
func (s *LoginState) GetUsername() string {
	return s.Username
}

// SetUsername sets the username.
func (s *LoginState) SetUsername(username string) {
	s.Username = username
}

// GetPassword returns the current password.
func (s *LoginState) GetPassword() string {
	return s.Password
}

// SetPassword sets the password.
func (s *LoginState) SetPassword(password string) {
	s.Password = password
}

// GetErrorMessage returns the current error message.
func (s *LoginState) GetErrorMessage() string {
	return s.ErrorMsg
}

// IsLoadingState returns whether the state is currently loading.
func (s *LoginState) IsLoadingState() bool {
	return s.IsLoading
}

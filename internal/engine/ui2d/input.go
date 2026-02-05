package ui2d

// InputState holds the current input state for the UI.
type InputState struct {
	// Mouse state
	MouseX      float32
	MouseY      float32
	MouseDeltaX float32
	MouseDeltaY float32

	// Mouse buttons (current frame)
	MouseLeftDown   bool
	MouseRightDown  bool
	MouseMiddleDown bool

	// Mouse buttons (pressed this frame - edge detected)
	MouseLeftPressed   bool
	MouseRightPressed  bool
	MouseMiddlePressed bool

	// Mouse click events (set by SDL event handler, cleared after UI processes)
	MouseLeftClicked  bool
	MouseRightClicked bool

	// Mouse buttons (released this frame)
	MouseLeftReleased   bool
	MouseRightReleased  bool
	MouseMiddleReleased bool

	// Scroll
	ScrollX float32
	ScrollY float32

	// Text input
	TextInput string

	// Key state
	KeyBackspace  bool
	KeyDelete     bool
	KeyEnter      bool
	KeyTab        bool
	KeyEscape     bool
	KeyLeft       bool
	KeyRight      bool
	KeyUp         bool
	KeyDown       bool
	KeyHome       bool
	KeyEnd        bool
	KeyCtrl       bool
	KeyShift      bool
	KeyAlt        bool
	KeySelectAll  bool // Ctrl+A
	KeyCopy       bool // Ctrl+C
	KeyPaste      bool // Ctrl+V
	KeyCut        bool // Ctrl+X
	KeyUndo       bool // Ctrl+Z

	// Previous frame state for edge detection
	prevMouseLeft   bool
	prevMouseRight  bool
	prevMouseMiddle bool
	prevMouseX      float32
	prevMouseY      float32

	// Key edge detection
	prevKeyBackspace bool
	prevKeyDelete    bool
	prevKeyEnter     bool
	prevKeyEscape    bool

	// Key pressed this frame (edge detected)
	KeyBackspacePressed bool
	KeyDeletePressed    bool
	KeyEnterPressed     bool
	KeyEscapePressed    bool
}

// Update prepares input state for a new frame.
// Call this at the start of each frame after updating raw input values.
func (i *InputState) Update() {
	// Calculate deltas
	i.MouseDeltaX = i.MouseX - i.prevMouseX
	i.MouseDeltaY = i.MouseY - i.prevMouseY

	// Detect mouse press/release edges
	i.MouseLeftPressed = i.MouseLeftDown && !i.prevMouseLeft
	i.MouseRightPressed = i.MouseRightDown && !i.prevMouseRight
	i.MouseMiddlePressed = i.MouseMiddleDown && !i.prevMouseMiddle

	i.MouseLeftReleased = !i.MouseLeftDown && i.prevMouseLeft
	i.MouseRightReleased = !i.MouseRightDown && i.prevMouseRight
	i.MouseMiddleReleased = !i.MouseMiddleDown && i.prevMouseMiddle

	// Detect key press edges
	i.KeyBackspacePressed = i.KeyBackspace && !i.prevKeyBackspace
	i.KeyDeletePressed = i.KeyDelete && !i.prevKeyDelete
	i.KeyEnterPressed = i.KeyEnter && !i.prevKeyEnter
	i.KeyEscapePressed = i.KeyEscape && !i.prevKeyEscape

	// Store current state for next frame
	i.prevMouseLeft = i.MouseLeftDown
	i.prevMouseRight = i.MouseRightDown
	i.prevMouseMiddle = i.MouseMiddleDown
	i.prevMouseX = i.MouseX
	i.prevMouseY = i.MouseY
	i.prevKeyBackspace = i.KeyBackspace
	i.prevKeyDelete = i.KeyDelete
	i.prevKeyEnter = i.KeyEnter
	i.prevKeyEscape = i.KeyEscape
}

// EndFrame clears per-frame input state.
// Call this at the end of each frame.
func (i *InputState) EndFrame() {
	i.TextInput = ""
	i.ScrollX = 0
	i.ScrollY = 0
	i.KeySelectAll = false
	i.KeyCopy = false
	i.KeyPaste = false
	i.KeyCut = false
	i.KeyUndo = false
	// Clear click events (they persist until consumed by UI)
	i.MouseLeftClicked = false
	i.MouseRightClicked = false
}

// IsMouseInRect checks if the mouse is within a rectangle.
func (i *InputState) IsMouseInRect(x, y, w, h float32) bool {
	return i.MouseX >= x && i.MouseX < x+w &&
		i.MouseY >= y && i.MouseY < y+h
}

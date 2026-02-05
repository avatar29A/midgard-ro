// Package ui provides ImGui-based user interface components.
package ui

import (
	"fmt"
	"os"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// koreanGlyphRanges defines the Unicode ranges for Korean text rendering.
// Format: pairs of [start, end] values terminated by 0.
var koreanGlyphRanges = []imgui.Wchar{
	0x0020, 0x00FF, // Basic Latin + Latin Supplement
	0x3000, 0x30FF, // CJK Symbols and Punctuation, Hiragana, Katakana
	0x3130, 0x318F, // Hangul Compatibility Jamo
	0xAC00, 0xD7AF, // Hangul Syllables
	0,              // Terminator
}

// Backend wraps the ImGui SDL backend for game use.
type Backend struct {
	backend backend.Backend[sdlbackend.SDLWindowFlags]
	width   int32
	height  int32
}

// NewBackend creates a new ImGui backend.
func NewBackend(title string, width, height int32) (*Backend, error) {
	b := &Backend{
		width:  width,
		height: height,
	}

	var err error
	b.backend, err = backend.CreateBackend(sdlbackend.NewSDLBackend())
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}

	// Set up font loading hook before creating window
	b.backend.SetAfterCreateContextHook(func() {
		b.loadKoreanFont()
	})

	b.backend.SetBgColor(imgui.NewVec4(0.1, 0.1, 0.12, 1.0))
	b.backend.CreateWindow(title, int(width), int(height))

	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("init opengl: %w", err)
	}

	return b, nil
}

// loadKoreanFont loads a font with Korean glyph support.
func (b *Backend) loadKoreanFont() {
	io := imgui.CurrentIO()
	fonts := io.Fonts()

	// Try different font paths (cross-platform support)
	fontPaths := []string{
		"/Library/Fonts/Arial Unicode.ttf",                       // macOS (symlink)
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",   // macOS (actual)
		"C:\\Windows\\Fonts\\malgun.ttf",                         // Windows (Malgun Gothic)
		"C:\\Windows\\Fonts\\gulim.ttc",                          // Windows (Gulim)
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc", // Linux
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", // Linux alt
	}

	var fontPath string
	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			fontPath = path
			break
		}
	}

	if fontPath == "" {
		return
	}

	fontCfg := imgui.NewFontConfig()
	defer fontCfg.Destroy()

	fonts.AddFontFromFileTTFV(fontPath, 16.0, fontCfg, &koreanGlyphRanges[0])
}

// Run starts the main render loop.
func (b *Backend) Run(renderFunc func()) {
	b.backend.Run(renderFunc)
}

// SetWindowTitle updates the window title.
func (b *Backend) SetWindowTitle(title string) {
	b.backend.SetWindowTitle(title)
}

// GetWindowSize returns the current window size.
func (b *Backend) GetWindowSize() (int32, int32) {
	return b.width, b.height
}

// GetViewport returns the main viewport work area.
func (b *Backend) GetViewport() (posX, posY, width, height float32) {
	viewport := imgui.MainViewport()
	workPos := viewport.WorkPos()
	workSize := viewport.WorkSize()
	return workPos.X, workPos.Y, workSize.X, workSize.Y
}

// BeginFrame starts a new ImGui frame.
func (b *Backend) BeginFrame() {
	// Frame is started automatically by the backend
}

// EndFrame ends the current ImGui frame.
func (b *Backend) EndFrame() {
	// Frame is ended automatically by the backend
}

// CreateTextureFromRGBA creates an OpenGL texture from RGBA data.
func (b *Backend) CreateTextureFromRGBA(data []byte, width, height int) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(data))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	return texID
}

// DeleteTexture deletes an OpenGL texture.
func (b *Backend) DeleteTexture(texID uint32) {
	gl.DeleteTextures(1, &texID)
}

// IsKeyPressed checks if a key was pressed this frame.
func IsKeyPressed(key imgui.Key) bool {
	return imgui.IsKeyChordPressed(imgui.KeyChord(key))
}

// IsKeyDown checks if a key is currently held down.
func IsKeyDown(key imgui.Key) bool {
	return imgui.IsKeyDown(key)
}

// Package ui provides game user interface components.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v4.1-core/gl"
	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/cursor"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
)

// UI2DBackend implements UIBackend using the custom ui2d rendering system.
type UI2DBackend struct {
	ctx *ui2d.Context

	// Texture cache for GRF-based UI textures
	texCache *TextureCache

	// Login screen textures (lazy-loaded)
	loginBgTex    *TextureInfo
	loadingTex    map[int]*TextureInfo // the loading screens, by 1-based index; nil for a missing one
	loginSkin     *loginWindowSkin
	loginTexTried bool // avoid repeated load attempts

	// loginSaveID mirrors the original's "save ID" checkbox. Nothing persists
	// it yet; it is drawn because the window art has the box.
	loginSaveID bool

	// Where the login window has been dragged to.
	loginWinX, loginWinY float32
	loginWinPlaced       bool

	// The game's own mouse cursor, drawn over everything else.
	cursor *cursor.Cursor

	cursorTick time.Time

	// What the interface wants the pointer to look like this frame.
	hudCursor    cursor.State
	hudCursorSet bool

	// Raw archive reader, for assets that are not textures — character
	// sprites are composited from SPR and ACT before they can be uploaded.
	assetLoader func(string) ([]byte, error)

	// Character portraits for the select screen, keyed by sprite spec.
	charSelPortraits map[charsprite.Spec]*charSelectPortrait

	// Character select art, and where its window has been dragged to.
	charSelSkin        *charSelectSkin
	charSelTried       bool
	charSelX, charSelY float32
	charSelPlaced      bool

	// The NPC dialog's Next and Close buttons, keyed by name.
	npcButtons map[string]*npcButtonSkin

	// The original's scrollbar art, shared by every list that needs one.
	scrollSkin  *scrollbarSkin
	scrollTried bool

	// A single white pixel, stretched to paint rectangles in the image layer.
	// See fillNPCRect for why that is not the same as DrawRect.
	whiteTex uint32

	// Where the NPC dialog has been dragged to, and how far its text is
	// scrolled back. npcTextLen notices new text so the view can re-pin to
	// the bottom when the script says something more.
	npcWinX, npcWinY float32
	npcWinPlaced     bool
	npcTextScroll    int
	npcTextLen       int

	// The menu window: where it sits, which row is selected, how far the
	// list is scrolled, and its shared OK/cancel art.
	npcMenuX, npcMenuY float32
	npcMenuPlaced      bool
	npcMenuIdx         int
	npcMenuScroll      int
	npcMenuBtns        map[string]*TextureInfo

	// Basic Info panel art, where it has been dragged to, and whether it is
	// folded down to its reduced form.
	hudSkin    *basicInfoSkin
	hudTried   bool
	hudX, hudY float32
	hudPlaced  bool
	hudReduced bool

	// Which menu windows are open. The buttons under the panel toggle these;
	// each window draws itself when its entry is set.
	hudOpen map[HUDWindow]bool

	// Chat scrollback position. Pinned means following the newest line, which
	// is where it starts and where it returns once scrolled back to the bottom.
	chatScroll int
	chatPinned bool
	chatTab    int
	chatX      float32
	chatY      float32
	chatW      float32
	chatH      float32
	chatPlaced bool
	chatInput  string
	chatName   string
	chatLocked bool
	chatDirty  bool

	hotkeyX      float32
	hotkeyY      float32
	hotkeyRows   int
	hotkeyPlaced bool
	hotkeyDirty  bool

	// hotkeyItems is what each quick-panel cell holds, by row and column: an
	// item id, or zero for an empty cell.
	hotkeyItems [hotkeyMaxRows][hotkeySlots]uint32
	hotkeyDrag  hotkeyDrag
	hotkeyPress hotkeyPress

	skillScroll int
	itemScroll  int
	itemTab     int

	// mapWorldView switches the Map window between this map and the world.
	mapWorldView  bool
	mapPlaced     bool
	mapSaved      ui2d.Rect
	itemAction    ItemAction
	itemDrag      itemDrag
	dropAction    DropAction
	dropPrompt    dropPrompt
	damageArt     damageArt
	statAction    StatAction
	levelUpAction LevelUpAction

	escOpen   bool
	escAction EscAction

	soundOpen     bool
	soundSeeded   bool
	soundDirty    bool
	sound         SoundSettings
	chatPending   string
	chatPendingTo string

	// The minimap image for the map we are on. minimapTried is the path last
	// attempted, so a map that ships no image is not retried every frame.
	minimapTex     *TextureInfo
	minimapTried   string
	minimapZoomIdx int

	// The player arrow, one texture per facing, baked on first use.
	minimapArrowTex    []uint32
	minimapArrowsTried bool

	// Cached widget states
	loginUsername string
	loginPassword string
	charSelectIdx int
}

// NewUI2DBackend creates a new ui2d UI backend.
func NewUI2DBackend(width, height int) (*UI2DBackend, error) {
	ctx, err := ui2d.NewContext(width, height)
	if err != nil {
		return nil, fmt.Errorf("create ui2d context: %w", err)
	}

	return &UI2DBackend{
		ctx:           ctx,
		charSelectIdx: -1,
	}, nil
}

// Begin starts a new UI frame.
//
// We piggyback on cimgui-go's SDL backend for windowing and input. ImGui has
// already pumped SDL events into its IO by this point, so we read the mouse
// and key state straight off ImGui's IO rather than installing a parallel SDL
// event handler. Same trick the ImGuiBackend uses (see updateInputFromImGui).
func (b *UI2DBackend) Begin() {
	b.syncInputFromImGui()
	b.syncViewportSize()
	b.fixHiDPIViewport()
	b.syncPixelDensity()
	b.ctx.Begin()
}

// syncPixelDensity keeps the glyph atlas rasterized for the display we're
// actually on. We lay out in points and stretch across the full retina
// framebuffer (see fixHiDPIViewport), so without this the 14pt atlas gets
// magnified 2x and every label renders soft.
func (b *UI2DBackend) syncPixelDensity() {
	scale := imgui.CurrentIO().DisplayFramebufferScale()
	density := scale.Y
	if scale.X > density {
		density = scale.X
	}
	if density <= 0 {
		density = 1
	}
	b.ctx.SetPixelDensity(density)
}

// fixHiDPIViewport overrides the glViewport that cimgui-go's SDL backend set
// to (0, 0, DisplaySize.x, DisplaySize.y). Those numbers are in logical
// points, but glViewport interprets them as framebuffer pixels — on a 2x
// retina display that confines our drawing to the bottom-left quadrant of the
// real framebuffer. Setting the viewport to the drawable size
// (points × DisplayFramebufferScale) makes our point-space rendering land
// 1:1 under the OS cursor.
func (b *UI2DBackend) fixHiDPIViewport() {
	io := imgui.CurrentIO()
	disp := io.DisplaySize()
	scale := io.DisplayFramebufferScale()
	if scale.X <= 0 {
		scale.X = 1
	}
	if scale.Y <= 0 {
		scale.Y = 1
	}
	gl.Viewport(0, 0, int32(disp.X*scale.X), int32(disp.Y*scale.Y))
}

// syncInputFromImGui copies the current frame's mouse + key state from ImGui's
// IO into the ui2d InputState. Must run before ctx.Begin() (which calls
// InputState.Update for edge detection).
//
// Coordinate space: empirically (via the click-corner test) cimgui-go's SDL
// backend reports mouse position in *global screen* points, not relative to
// the SDL window. Click deltas across known widget widths matched our render
// units 1:1, so the only correction needed is subtracting the SDL window's
// screen position — given to us by MainViewport().Pos(). After that the
// mouse lives in the same logical 0..DisplaySize space we render into,
// which fixHiDPIViewport stretches across the full retina framebuffer.
func (b *UI2DBackend) syncInputFromImGui() {
	in := b.ctx.Input()
	io := imgui.CurrentIO()

	winPos := imgui.MainViewport().Pos()
	mp := imgui.MousePos()
	in.MouseX = mp.X - winPos.X
	in.MouseY = mp.Y - winPos.Y
	in.MouseLeftDown = imgui.IsMouseDown(imgui.MouseButtonLeft)
	in.MouseRightDown = imgui.IsMouseDown(imgui.MouseButtonRight)
	in.MouseMiddleDown = imgui.IsMouseDown(imgui.MouseButtonMiddle)
	in.ScrollX = io.MouseWheelH()
	in.ScrollY = io.MouseWheel()

	in.KeyBackspace = imgui.IsKeyDown(imgui.KeyBackspace)
	in.KeyEnter = imgui.IsKeyDown(imgui.KeyEnter)
	in.KeyEscape = imgui.IsKeyDown(imgui.KeyEscape)
	in.KeyTab = imgui.IsKeyDown(imgui.KeyTab)

	// Bridge ImGui's per-frame character input queue into ui2d's TextInput
	// so users can type into our text fields. ImGui already translates
	// SDL2 SDL_TEXTINPUT events into Wchars on its IO; we just consume the
	// queue. Non-ASCII Wchars become multi-byte UTF-8 via rune conversion.
	if chars := io.InputQueueCharacters().Slice(); len(chars) > 0 {
		var buf strings.Builder
		for _, ch := range chars {
			if ch >= 32 { // skip control chars; backspace/enter handled via key flags
				buf.WriteRune(rune(ch))
			}
		}
		in.TextInput += buf.String()
	}
}

// syncViewportSize keeps the ui2d renderer matched to ImGui's viewport size,
// so the UI scales correctly when the SDL window is resized.
func (b *UI2DBackend) syncViewportSize() {
	size := imgui.MainViewport().Size()
	curW, curH := b.ctx.GetScreenSize()
	if int(size.X) != int(curW) || int(size.Y) != int(curH) {
		b.ctx.Resize(int(size.X), int(size.Y))
	}
}

// End finishes the UI frame.
func (b *UI2DBackend) End() {
	// The cursor is drawn last so it sits over every window, which is where a
	// cursor belongs.
	b.drawCursor()

	b.ctx.End()
}

// WantCursor is the cursor an interface element under the pointer asks for,
// and whether anything asked. Set while drawing and read afterwards, so a
// widget can say what it is without knowing what else is on screen.
func (b *UI2DBackend) WantCursor() (cursor.State, bool) {
	return b.hudCursor, b.hudCursorSet
}

// wantCursor is how a widget asks. First asker wins, so a grip inside a
// window does not have its request overwritten by the window behind it.
func (b *UI2DBackend) wantCursor(state cursor.State) {
	if b.hudCursorSet {
		return
	}

	b.hudCursor = state
	b.hudCursorSet = true
}

// SetCursorState switches which cursor is shown.
func (b *UI2DBackend) SetCursorState(state cursor.State) {
	b.cursor.SetState(state)
}

// drawCursor advances the cursor's animation and draws it under the pointer.
func (b *UI2DBackend) drawCursor() {
	if b.cursor == nil {
		return
	}

	now := time.Now()
	b.cursor.Update(now.Sub(b.cursorTick))
	b.cursorTick = now

	frame, ok := b.cursor.Frame()
	if !ok {
		return
	}

	in := b.ctx.Input()

	// The sprite's origin is the point being pointed at, and the frame's
	// offset says where its top-left sits relative to that.
	// On the overlay layer, not with the other images: images are drawn
	// before solids, so anything rectangular — a gauge fill, an experience
	// bar, a button's hover shading — was painting over the cursor.
	b.ctx.Renderer().DrawImageTop(frame.Texture,
		in.MouseX+frame.OffsetX, in.MouseY+frame.OffsetY,
		frame.Width, frame.Height, ui2d.ColorWhite)
}

// SetAssetLoader wires the GRF asset loader into the UI backend.
// This enables loading RO textures for window skins and login screen.
func (b *UI2DBackend) SetAssetLoader(loadFunc func(string) ([]byte, error)) {
	b.texCache = NewTextureCache(b.ctx.Renderer(), loadFunc)
	b.assetLoader = loadFunc

	// The cursor comes from the archives like everything else, so it can only
	// be built once there is something to read them with.
	if c, err := cursor.Load(loadFunc, b.ctx.Renderer().CreateTextureNearest); err != nil {
		logger.Warn("no game cursor, leaving the system one", zap.Error(err))
	} else {
		b.cursor = c
		b.cursorTick = time.Now()
	}

	// Prefer the original window chrome; the nine-sliced skin stays as the
	// fallback for archives that lack it.
	if frame, err := LoadNativeWindowFrame(b.texCache); err == nil {
		b.ctx.SetWindowFrame(frame)
	} else {
		logger.Warn("native window chrome unavailable, using the themed skin", zap.Error(err))
	}

	// Try to load window skin
	skin, err := LoadWindowSkin(b.texCache)
	if err == nil && skin.Frame != nil {
		b.ctx.SetDefaultWindowSkin(skin.Frame)
	}

	// Try to load input skin (RO `name-edit.bmp`). If the asset is missing
	// the inputs fall back to drawSunkenInput's procedural bevel.
	if inputSkin, err := LoadInputSkin(b.texCache); err == nil && inputSkin != nil {
		b.ctx.SetDefaultInputSkin(inputSkin)
	}
}

// SetClickSound wires the sound played when a button is pressed.
func (b *UI2DBackend) SetClickSound(play func()) {
	b.ctx.SetClickSound(play)
}

// Close releases backend resources.
func (b *UI2DBackend) Close() {
	if b.texCache != nil {
		b.texCache.Close()
	}
	if b.ctx != nil {
		b.ctx.Close()
	}
}

// Resize updates the screen size.
func (b *UI2DBackend) Resize(width, height int) {
	b.ctx.Resize(width, height)
}

// GetScreenSize returns the current screen dimensions.
func (b *UI2DBackend) GetScreenSize() (width, height float32) {
	return b.ctx.GetScreenSize()
}

// MouseCaptured reports whether the pointer is over the interface.
func (b *UI2DBackend) MouseCaptured() bool {
	return b.ctx.MouseCaptured()
}

// Input returns the input state.
func (b *UI2DBackend) Input() *ui2d.InputState {
	return b.ctx.Input()
}

// DrawSceneTexture draws a 3D scene texture.
func (b *UI2DBackend) DrawSceneTexture(x, y, w, h float32, textureID uint32) {
	b.ctx.Renderer().DrawSceneTexture(x, y, w, h, textureID)
}

// loginTexBasePath is the GRF path for login screen textures.
// RO interface art lives under the Korean "user interface" folder; the asset
// manager handles the EUC-KR encoding of these paths.
const (
	uiTexBasePath    = `data\texture\유저인터페이스\`
	loginTexBasePath = uiTexBasePath + `login_interface\`

	// The login screen backdrop. Verified present in data.grf — the previous
	// login_bg.bmp / login_logo.bmp were not in the archive at all, which is
	// why the login screen rendered on bare black.
	loginBackgroundTex = uiTexBasePath + `bgi_temp.bmp`
)

// The original login window is a single 280x120 texture with the labels and
// input wells painted in; the client places five widgets on top at fixed
// offsets. Those offsets are the original's, cross-checked against roBrowser's
// transcription of the same window.
const (
	loginWinW = float32(280)
	loginWinH = float32(120)

	loginFieldX = float32(91)
	loginFieldW = float32(127)
	loginFieldH = float32(18)
	loginUserY  = float32(29)
	loginPassY  = float32(61)

	// Buttons and the checkbox are anchored to the right and bottom edges.
	loginBtnW      = float32(42)
	loginBtnH      = float32(20)
	loginBtnBottom = float32(4)
	loginConnRight = float32(50)
	loginExitRight = float32(5)

	// The checkbox art that ships with the window has its Korean caption
	// baked in, so the plain 12x12 box is used and the caption drawn.
	loginChkBox   = float32(12)
	loginChkY     = float32(31)
	loginChkRight = float32(44)

	// The window art is Korean. Its captions are painted over and redrawn in
	// English: the body behind them is flat white, and the title bar is a
	// vertical gradient that is uniform across its width, so a clean column
	// of it stretches over the title without a seam.
	loginTitleX     = float32(16) // Korean title glyphs span x 16..50
	loginTitleY     = float32(1)
	loginTitleW     = float32(35)
	loginTitleH     = float32(12)
	loginCleanColX  = float32(200) // title-bar column with no text or icon
	loginLabelMaskX = float32(30)  // captions sit within x 30..86
	loginLabelMaskW = float32(56)
	loginLabelMaskH = float32(15)
	loginLabelRight = float32(83) // captions are right-aligned to here

	// The original's UI text is around 11px. The font is 14pt, so it is scaled
	// down to sit inside an 18px well the way the artwork expects.
	loginTextScale = float32(0.7)

	// The buttons carry their captions in the texture too. Ink spans x 10..32
	// on both; columns 2..9 are clean face, so one of them stretches over the
	// caption the same way the title bar does.
	loginBtnInkX   = float32(3)
	loginBtnInkW   = float32(36)
	loginBtnInkY   = float32(2)
	loginBtnInkH   = float32(16)
	loginBtnCleanX = float32(39)
	loginBtnTexW   = float32(42)
	loginBtnTexH   = float32(20)
)

// loginWindowSkin holds the original login window art and its widget states.
type loginWindowSkin struct {
	window *TextureInfo

	connect, connectOver, connectDown *TextureInfo
	exit, exitOver, exitDown          *TextureInfo
	saveOff, saveOn                   *TextureInfo
}

// loginWindowPos returns the window's top-left, centering it the first time
// and honoring wherever it has since been dragged.
func (b *UI2DBackend) loginWindowPos(width, height float32) (float32, float32) {
	if !b.loginWinPlaced {
		b.loginWinX = float32(int((width - loginWinW) / 2))
		b.loginWinY = float32(int((height - loginWinH) / 2))
		b.loginWinPlaced = true
	}

	return b.loginWinX, b.loginWinY
}

// drawLoginCaptions paints over the window's Korean captions and redraws them
// in English.
func (b *UI2DBackend) drawLoginCaptions(skin *loginWindowSkin, x, y float32) {
	r := b.ctx.Renderer()

	// Title: stretch a clean column of the gradient across the glyphs, then
	// write over it. The icon to the left of them is left alone.
	u := loginCleanColX / loginWinW
	v0 := loginTitleY / loginWinH
	v1 := (loginTitleY + loginTitleH) / loginWinH
	r.DrawImageUV(skin.window.ID,
		x+loginTitleX, y+loginTitleY, loginTitleW, loginTitleH,
		u, v0, u+1/loginWinW, v1, ui2d.ColorWhite)

	ascent := r.FontAscent(loginTextScale)
	r.DrawText(x+loginTitleX, y+loginTitleY+(loginTitleH-ascent)/2,
		"Login", loginTextScale, ui2d.ColorText)

	// Captions: the body behind them is flat white.
	for _, caption := range []struct {
		text string
		y    float32
	}{
		{"ID", loginUserY},
		{"Password", loginPassY},
	} {
		maskY := y + caption.y + (loginFieldH-loginLabelMaskH)/2
		r.DrawRect(x+loginLabelMaskX, maskY, loginLabelMaskW, loginLabelMaskH, ui2d.ColorWhite)

		textW, _ := r.MeasureText(caption.text, loginTextScale)
		r.DrawText(x+loginLabelRight-textW, y+caption.y+(loginFieldH-ascent)/2,
			caption.text, loginTextScale, ui2d.ColorText)
	}
}

// loadLoginSkin loads the original login window art. A miss leaves the skin
// nil and the caller falls back to the themed window, so a client running
// against an archive without these textures still shows a usable login.
func (b *UI2DBackend) loadLoginSkin() *loginWindowSkin {
	if b.loginSkin != nil {
		return b.loginSkin
	}

	const dir = loginTexBasePath

	names := []string{
		`win_login.bmp`,
		`btn_connect.bmp`, `btn_connect_a.bmp`, `btn_connect_b.bmp`,
		`btn_exit.bmp`, `btn_exit_a.bmp`, `btn_exit_b.bmp`,
		`checkbox_off.bmp`, `checkbox_on.bmp`,
	}

	loaded := make([]*TextureInfo, 0, len(names))

	for _, name := range names {
		tex, err := b.texCache.Load(dir + name)
		if err != nil {
			logger.Warn("login window art unavailable, falling back to the themed window",
				zap.String("path", dir+name), zap.Error(err))

			return nil
		}

		loaded = append(loaded, tex)
	}

	b.loginSkin = &loginWindowSkin{
		window:      loaded[0],
		connect:     loaded[1],
		connectOver: loaded[2],
		connectDown: loaded[3],
		exit:        loaded[4],
		exitOver:    loaded[5],
		exitDown:    loaded[6],
		saveOff:     loaded[7],
		saveOn:      loaded[8],
	}

	return b.loginSkin
}

// renderNativeLoginWindow draws the original login window and its widgets, and
// reports whether it handled the screen.
func (b *UI2DBackend) renderNativeLoginWindow(state LoginUIState, width, height float32) bool {
	skin := b.loadLoginSkin()
	if skin == nil {
		return false
	}

	x, y := b.loginWindowPos(width, height)

	// Draggable by the title bar, as the original is. Done before the widgets
	// so a drag that starts on the bar keeps them from reacting.
	b.ctx.DragHandle("login_titlebar",
		ui2d.Rect{X: x, Y: y, W: loginWinW, H: loginTitleH + loginTitleY}, &b.loginWinX, &b.loginWinY)
	x, y = b.loginWinX, b.loginWinY

	r := b.ctx.Renderer()
	r.DrawImage(skin.window.ID, x, y, loginWinW, loginWinH, ui2d.ColorWhite)
	b.drawLoginCaptions(skin, x, y)

	doLogin := func() {
		if !state.IsLoading && state.OnLogin != nil {
			state.OnLogin()
		}
	}

	// The wells are painted into the window art, so the fields draw text only.
	user, userChanged, userSubmit := b.ctx.TextInputBareAt("login_user",
		x+loginFieldX, y+loginUserY, loginFieldW, loginFieldH, loginTextScale, b.loginUsername)
	if userChanged {
		b.loginUsername = user

		if state.OnUsernameChange != nil {
			state.OnUsernameChange(user)
		}
	}

	pass, passChanged, passSubmit := b.ctx.PasswordInputBareAt("login_pass",
		x+loginFieldX, y+loginPassY, loginFieldW, loginFieldH, loginTextScale, b.loginPassword)
	if passChanged {
		b.loginPassword = pass

		if state.OnPasswordChange != nil {
			state.OnPasswordChange(pass)
		}
	}

	saveTex := skin.saveOff
	if b.loginSaveID {
		saveTex = skin.saveOn
	}

	chkX := x + loginWinW - loginChkRight - loginChkBox

	if b.ctx.ImageButtonAt("login_save",
		chkX, y+loginChkY, loginChkBox, loginChkBox,
		saveTex.ID, saveTex.ID, saveTex.ID) {
		b.loginSaveID = !b.loginSaveID
	}

	// Centered on the box rather than sitting on its top edge.
	saveAscent := r.FontAscent(loginTextScale)
	r.DrawText(chkX+loginChkBox+4, y+loginChkY+(loginChkBox-saveAscent)/2,
		"Save", loginTextScale, ui2d.ColorText)

	btnY := y + loginWinH - loginBtnBottom - loginBtnH

	if b.skinButton("login_connect", x+loginWinW-loginConnRight-loginBtnW, btnY,
		skin.connect, skin.connectOver, skin.connectDown, "Ok") {
		doLogin()
	}

	if b.skinButton("login_exit", x+loginWinW-loginExitRight-loginBtnW, btnY,
		skin.exit, skin.exitOver, skin.exitDown, "Exit") {
		if state.OnExit != nil {
			state.OnExit()
		}
	}

	if userSubmit || passSubmit {
		doLogin()
	}

	// The original has no room for status text inside the window; the real
	// client uses a separate popup. Until there is one, it goes underneath.
	statusY := y + loginWinH + 8

	switch {
	case state.ErrorMessage != "":
		b.ctx.LabelAtColored(x, statusY, state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1})
	case state.IsLoading:
		b.ctx.LabelAtColored(x, statusY, "Connecting...", ui2d.ColorText)
	default:
		b.ctx.LabelAtColored(x, statusY, "Server: "+state.ServerName, ui2d.ColorTextDim)
	}

	return true
}

// skinButton draws one of RO's 42x20 buttons and replaces the caption
// baked into it with an English one.
func (b *UI2DBackend) skinButton(id string, x, y float32, normal, over, down *TextureInfo, label string) bool {
	clicked, drawn := b.ctx.ImageButtonAtEx(id, x, y, loginBtnW, loginBtnH,
		normal.ID, over.ID, down.ID)

	if drawn == 0 {
		return clicked
	}

	r := b.ctx.Renderer()

	// Cover the caption with a clean column of whichever state is showing —
	// the three differ, so the mask has to come from the texture on screen.
	u := loginBtnCleanX / loginBtnTexW
	r.DrawImageUV(drawn,
		x+loginBtnInkX, y+loginBtnInkY, loginBtnInkW, loginBtnInkH,
		u, loginBtnInkY/loginBtnTexH,
		u+1/loginBtnTexW, (loginBtnInkY+loginBtnInkH)/loginBtnTexH,
		ui2d.ColorWhite)

	textW, _ := r.MeasureText(label, loginTextScale)
	ascent := r.FontAscent(loginTextScale)
	r.DrawText(x+(loginBtnW-textW)/2, y+(loginBtnH-ascent)/2, label, loginTextScale, ui2d.ColorText)

	return clicked
}

// loadLoginTextures lazy-loads the login-screen backdrop. We use
// `t_login.jpg`, the Korean RO client's title-screen art, drawn fullscreen
// behind the dialog. Dialog chrome itself is rendered from the generic
// `win_msgbox` skin so labels and buttons stay text-driven (translatable);
// the per-screen `win_login.bmp` is *not* used because its Korean labels
// are baked into the artwork.
func (b *UI2DBackend) loadLoginTextures() {
	if b.loginTexTried || b.texCache == nil {
		return
	}
	b.loginTexTried = true

	bg, err := b.texCache.Load(loginBackgroundTex)
	if err == nil {
		b.loginBgTex = bg
	} else {
		// Worth saying out loud. A silently swallowed miss here is what left
		// the login screen on a black background with nobody noticing that the
		// path had never existed.
		logger.Warn("login background texture unavailable",
			zap.String("path", loginBackgroundTex), zap.Error(err))
	}
}

// RenderLoginUI renders the login screen.
//
// Layout: t_login.jpg fills the screen as the title backdrop; the dialog
// itself is a generic themed window centered on top. Labels and the Login
// button are drawn from Go strings (not from baked-in BMP artwork) so they
// remain translatable.
func (b *UI2DBackend) RenderLoginUI(state LoginUIState, width, height float32) {
	b.loadLoginTextures()

	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	if b.loginUsername == "" && state.Username != "" {
		b.loginUsername = state.Username
	}
	if b.loginPassword == "" && state.Password != "" {
		b.loginPassword = state.Password
	}

	if b.renderNativeLoginWindow(state, width, height) {
		return
	}

	// Compact dialog modeled on the original RO "Log On" — labels sit
	// LEFT of inputs (not above), and login/exit buttons live in the
	// bottom-right corner. HStack flex sizes the inputs to fill the
	// remaining width after the fixed-width label column.
	windowWidth := float32(420)
	windowHeight := float32(190)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindow("login", windowX, windowY, windowWidth, windowHeight, "Log On") {
		doLogin := func() {
			if !state.IsLoading && state.OnLogin != nil {
				state.OnLogin()
			}
		}

		labelW := float32(80) // wide enough for "Password" without wrap

		idRow := ui2d.HStack(8,
			ui2d.Sized(labelW, 0, ui2d.Label("ID")),
			ui2d.Sized(0, 22, ui2d.TextInput("username", &b.loginUsername, nil)),
		)
		passRow := ui2d.HStack(8,
			ui2d.Sized(labelW, 0, ui2d.Label("Password")),
			ui2d.Sized(0, 22, ui2d.PasswordInput("password", &b.loginPassword, doLogin)),
		)

		// Bottom action row: Filler pushes the buttons to the right edge.
		// 28px tall gives the radius-6 corners visible vertical space
		// (h - 2*r = 16px straight middle) without the button looking
		// chunky.
		btnW := float32(80)
		btnH := float32(28)
		btnRow := ui2d.HStack(6,
			ui2d.Filler(),
			ui2d.Sized(btnW, btnH, ui2d.Button("login", "login", doLogin)),
			ui2d.Sized(btnW, btnH, ui2d.Button("exit", "exit", func() {
				if state.OnExit != nil {
					state.OnExit()
				}
			})),
		)

		var rows []ui2d.Element
		rows = append(rows, idRow, passRow)
		if state.ErrorMessage != "" {
			rows = append(rows, ui2d.LabelColor(state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1}))
		}
		if state.IsLoading {
			rows = append(rows, ui2d.LabelCenteredEl("Connecting..."))
		}
		rows = append(rows,
			ui2d.LabelColor("Server: "+state.ServerName, ui2d.ColorTextDim),
			ui2d.Filler(), // pushes button row to bottom
			btnRow,
		)

		// Notify owners if the tree mutated the editable values.
		prevUser := state.Username
		prevPass := state.Password
		b.ctx.RenderTree(ui2d.VStack(8, rows...), b.ctx.CurrentWindowContentRect())
		if b.loginUsername != prevUser && state.OnUsernameChange != nil {
			state.OnUsernameChange(b.loginUsername)
		}
		if b.loginPassword != prevPass && state.OnPasswordChange != nil {
			state.OnPasswordChange(b.loginPassword)
		}

		b.ctx.EndWindow()
	}
}

// RenderConnectingUI renders the connecting screen — same backdrop as
// login/charselect, themed window with status messages.
func (b *UI2DBackend) RenderConnectingUI(state ConnectingUIState, width, height float32) {
	b.loadLoginTextures()
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	windowWidth := float32(320)
	windowHeight := float32(150)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindowEx("connecting", windowX, windowY, windowWidth, windowHeight,
		"Connecting", ui2d.WindowOptions{}) {
		var rows []ui2d.Element
		if state.StatusMessage != "" {
			rows = append(rows, ui2d.LabelCenteredEl(state.StatusMessage))
		}
		if state.ErrorMessage != "" {
			rows = append(rows, ui2d.LabelColor(state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1}))
		}
		rows = append(rows, ui2d.Spacer(8), ui2d.LabelCenteredEl("Please wait..."))

		b.ctx.RenderTree(ui2d.VStack(8, rows...), b.ctx.CurrentWindowContentRect())
		b.ctx.EndWindow()
	}
}

// RenderCharSelectUI renders the character selection screen.
//
// Layout: t_login.jpg backdrop, centered themed window. The body is a
// declarative tree — VStack of status/error rows, character Selectables,
// detail labels, and the Enter Game button — so positions are deterministic
// and every row gets the same vertical rhythm.
func (b *UI2DBackend) RenderCharSelectUI(state CharSelectUIState, width, height float32) {
	// Reuse the title-screen backdrop: it doubles as the char-select
	// scenery in vanilla RO and saves loading another big asset.
	b.loadLoginTextures()
	if b.loginBgTex != nil {
		b.ctx.Renderer().DrawImage(b.loginBgTex.ID, 0, 0, width, height, ui2d.ColorWhite)
	}

	if b.renderNativeCharSelect(state, width, height) {
		return
	}

	windowWidth := float32(420)
	windowHeight := float32(420)
	windowX := (width - windowWidth) / 2
	windowY := (height - windowHeight) / 2

	if b.ctx.BeginWindowEx("charselect", windowX, windowY, windowWidth, windowHeight,
		"Character Selection", ui2d.WindowOptions{}) {
		// Auto-select first row when characters arrive.
		if state.IsReady && b.charSelectIdx < 0 && len(state.Characters) > 0 {
			b.charSelectIdx = 0
			if state.OnSelectIndex != nil {
				state.OnSelectIndex(0)
			}
		}

		var rows []ui2d.Element
		if state.StatusMessage != "" {
			rows = append(rows, ui2d.Label(state.StatusMessage))
		}
		if state.ErrorMessage != "" {
			rows = append(rows, ui2d.LabelColor(state.ErrorMessage, ui2d.Color{R: 1, G: 0.4, B: 0.4, A: 1}))
		}

		switch {
		case !state.IsReady:
			rows = append(rows, ui2d.Spacer(8), ui2d.LabelCenteredEl("Loading character list..."))
		case len(state.Characters) == 0:
			rows = append(rows,
				ui2d.Spacer(8),
				ui2d.LabelCenteredEl("No characters found."),
				ui2d.Spacer(4),
				ui2d.LabelCenteredEl("Create a new character on the server."),
			)
		default:
			rows = append(rows, ui2d.Label("Characters:"))
			for i, char := range state.Characters {
				idx := i // capture for closure
				rows = append(rows, ui2d.Sized(0, 24, ui2d.Selectable(
					fmt.Sprintf("char_%d", i),
					fmt.Sprintf("%s  (Lv %d)", char.GetName(), char.BaseLevel),
					b.charSelectIdx == i,
					func() {
						b.charSelectIdx = idx
						if state.OnSelectIndex != nil {
							state.OnSelectIndex(idx)
						}
					},
				)))
			}

			if b.charSelectIdx >= 0 && b.charSelectIdx < len(state.Characters) {
				char := state.Characters[b.charSelectIdx]
				rows = append(rows,
					ui2d.Spacer(8),
					ui2d.Label(fmt.Sprintf("HP: %d/%d    SP: %d/%d", char.HP, char.MaxHP, char.SP, char.MaxSP)),
					ui2d.Label(fmt.Sprintf("Map: %s", char.GetMapName())),
				)
			}

			rows = append(rows, ui2d.Spacer(8))
			canEnter := !state.IsLoading && b.charSelectIdx >= 0
			enterLabel := "Enter Game"
			rows = append(rows, ui2d.Sized(0, 36, ui2d.Button("enter", enterLabel, func() {
				if canEnter && state.OnSelect != nil {
					state.OnSelect(b.charSelectIdx)
				}
			})))
		}

		b.ctx.RenderTree(ui2d.VStack(6, rows...), b.ctx.CurrentWindowContentRect())
		b.ctx.EndWindow()
	}
}

// RenderInGameUI renders the in-game HUD.
func (b *UI2DBackend) RenderInGameUI(state InGameUIState, dt float64, width, height float32) {
	// Cleared each frame: a request only holds while the pointer is still on
	// the thing that made it.
	b.hudCursor, b.hudCursorSet = cursor.StateDefault, false

	// Draw scene texture as background
	if state.SceneReady && state.SceneTexture != 0 {
		b.ctx.Renderer().DrawSceneTexture(0, 0, width, height, state.SceneTexture)
	}

	b.renderBasicInfo(state)
	// Under the units, before the panels: a bar belongs to the world, and the
	// interface sits over it.
	for _, bar := range state.EntityBars {
		b.drawEntityBars(bar)
	}

	// Over the bars, still under the panels: the label belongs to the item it
	// names, and a window drawn later covers it like anything else in the
	// world.
	b.drawDamageNumbers(state.DamageNumbers)
	b.drawTargetMarker(state.TargetMarker)

	for _, label := range state.WorldLabels {
		b.drawWorldLabel(label)
	}

	b.drawMinimap(state, width)
	b.drawChat(state, height)
	b.drawHotkeys(state, width, height)
	b.drawLevelUpButtons(state.LevelUpButtons, width, height)

	b.drawEscMenu(width, height)
	b.drawSoundConfig(width, height)
	b.drawStatsWindow(state, width, height)
	b.drawSkillsWindow(state, width, height)
	b.drawItemsWindow(state, width, height)
	b.drawMapWindow(state, width, height)

	// After the inventory, which is what it belongs to: drawn before it, the
	// window it was dragged out of covered it.
	b.drawDropQuantity(width, height)

	// Last: whatever is being dragged rides over everything it might be
	// dropped on.
	b.drawDraggedItem()

	// After the windows, so it sees whichever grip the pointer ended on. RO
	// has no resize pointer of its own, so this is the hand it shows for
	// anything you press — the pointer changing at all is the affordance.
	if b.ctx.OverResizeGrip() {
		b.wantCursor(cursor.StateClick)
	}
	b.renderNPCDialog(state, width, height)
	b.renderNPCMenu(state, width, height)

	// Debug overlay (top-left)
	if state.ShowDebugInfo {
		// Tall enough for the stat rows: the height is not derived from the
		// content, so text past it draws outside the frame rather than
		// growing it.
		if b.ctx.BeginWindow("debug", 10, 10, 520, 320, "Debug") {
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Map: %s", state.MapName))
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Load: %.0f ms", state.MapLoadMs))
			b.ctx.Row(16)
			b.ctx.Label(state.MapLoadPhases)
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Indoor: %s   Water: %d cells", describeCameraRules(state), state.WaterCells))
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Tile: (%d, %d)", state.PlayerTileX, state.PlayerTileY))
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Pos: (%.0f, %.0f, %.0f)", state.PlayerX, state.PlayerY, state.PlayerZ))
			b.ctx.Separator()
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Dir: %d  Entities: %d", state.PlayerDirection, state.EntityCount))
			b.ctx.Separator()
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("HP: %d/%d   SP: %d/%d",
				state.PlayerHP, state.PlayerMaxHP, state.PlayerSP, state.PlayerMaxSP))
			b.ctx.Row(16)
			b.ctx.Label(fmt.Sprintf("Base Lv: %d   Job Lv: %d", state.PlayerLevel, state.PlayerJobLevel))
			b.ctx.Separator()
			b.ctx.Row(16)
			b.ctx.Label("Cmd: " + describeCommand(state))
			b.ctx.Row(16)
			b.ctx.Label("Dialog: " + describeDialog(state))
			b.ctx.Row(16)
			b.ctx.Label("HUD: " + b.describeHUD())
			b.ctx.EndWindow()
		}
	}

	// Error overlay
	if state.ErrorMessage != "" {
		windowWidth := float32(300)
		windowHeight := float32(80)
		windowX := (width - windowWidth) / 2
		windowY := (height - windowHeight) / 2

		if b.ctx.BeginWindow("error", windowX, windowY, windowWidth, windowHeight, "Error") {
			b.ctx.Spacer(4)
			b.ctx.LabelColored(state.ErrorMessage, ui2d.Color{R: 1, G: 0.3, B: 0.3, A: 1})
			b.ctx.EndWindow()
		}
	}

}

// describeDialog is the debug overlay's one-line account of the conversation.
//
// A stuck dialog is the failure this is for: the phase says what the client
// thinks the server last asked for, so a screenshot is enough to tell a packet
// we ignored from a window we failed to draw.
// describeCameraRules is the map's camera rules in a word or two.
func describeCameraRules(state InGameUIState) string {
	s := "no"
	if state.Indoor {
		s = "yes"
	}
	var rules []string
	if state.CameraZoomLocked {
		rules = append(rules, "zoom locked")
	}
	switch {
	case state.CameraYawLocked:
		rules = append(rules, "yaw locked")
	case state.CameraArc:
		rules = append(rules, "arc")
	}
	if len(rules) > 0 {
		s += " (" + strings.Join(rules, ", ") + ")"
	}
	return s
}

// describeCommand reports the last command and what became of it.
//
// The outcome is the point: "unknown" means the name never resolved, while
// "sent" means it went to the server and the silence that followed is the
// server's answer rather than a client bug.
func describeCommand(state InGameUIState) string {
	if state.LastCommand == "" {
		return "none yet"
	}

	return state.LastCommand + " -> " + state.LastCommandOutcome
}

func describeDialog(state InGameUIState) string {
	if state.DialogNPCID == 0 {
		return state.DialogPhase
	}

	who := state.DialogNPCName
	if who == "" {
		// Legitimate: the server sends a fake npc id for scripts whose owner
		// is not a unit near the player, and there is nothing to look up.
		who = "?"
	}

	line := fmt.Sprintf("%s  npc %d (%s)", state.DialogPhase, state.DialogNPCID, who)
	if len(state.DialogMenu) > 0 {
		line += fmt.Sprintf("  %d items", len(state.DialogMenu))
	}

	return line
}

// RenderFPSOverlay renders an FPS counter.
func (b *UI2DBackend) RenderFPSOverlay(fps float64, width, height float32) {
	scale := float32(1.0)
	text := fmt.Sprintf("FPS: %.0f", fps)
	textW, _ := b.ctx.Renderer().MeasureText(text, scale)

	x := width - textW - 10
	y := float32(5)

	// The minimap owns the top-right corner, as it does in the original, so
	// the counter drops below it — past its zoom buttons too, which sit under
	// the map and were the second thing this printed over.
	if b.minimapTex != nil {
		y += minimapSize + minimapBtn + minimapMargin
	}

	// Semi-transparent background
	b.ctx.Renderer().DrawRect(x-5, y-2, textW+10, 20, ui2d.ColorPanelBg.WithAlpha(0.5))
	b.ctx.Renderer().DrawText(x, y, text, scale, ui2d.ColorTextOnDark)
}

// RenderScreenshotMessage renders a screenshot notification.
func (b *UI2DBackend) RenderScreenshotMessage(msg string, width, height float32) {
	scale := float32(1.0)
	textW, textH := b.ctx.Renderer().MeasureText(msg, scale)

	msgWidth := textW + 20
	x := (width - msgWidth) / 2
	y := height - 60

	// Semi-transparent background
	b.ctx.Renderer().DrawRect(x, y, msgWidth, textH+10, ui2d.ColorPanelBg.WithAlpha(0.8))
	b.ctx.Renderer().DrawText(x+10, y+5, msg, scale, ui2d.Color{R: 0.2, G: 1.0, B: 0.2, A: 1.0})
}

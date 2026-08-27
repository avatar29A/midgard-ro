package ui2d

// Metrics of the original window chrome, in texture pixels. Each is the size of
// the artwork it names, so the frame is assembled at the scale RO drew it.
const (
	// FrameTitleH is the height of the title bar strip.
	FrameTitleH = float32(17)

	// frameCapW is the width of the title bar's left and right caps. The
	// middle is tiled between them.
	frameCapW = float32(12)

	// frameSysBtn is the side of the title bar's system buttons.
	frameSysBtn = float32(11)

	// frameSysGap separates the system buttons from each other and the edge.
	frameSysGap = float32(3)

	// frameFooterH is the height of the footer bar, which carries the resize
	// grip. Windows without one end at the body.
	frameFooterH = float32(29)

	// frameGrip is the side of the resize grip in the footer's corner.
	frameGrip = float32(13)

	// FrameMinW and FrameMinH stop a window being resized into nothing.
	FrameMinW = float32(120)
	FrameMinH = FrameTitleH + 40
)

// WindowFrame holds the textures the original client assembles its windows
// from. Loading them belongs to the asset layer, which hands the ids over.
//
// A frame is optional: without one, windows fall back to the themed panel.
type WindowFrame struct {
	// Title bar, drawn as three slices with the middle tiled.
	TitleLeft, TitleMid, TitleRight uint32

	// System buttons, each with a hover texture.
	SysMiniOff, SysMiniOn   uint32
	SysCloseOff, SysCloseOn uint32

	// Footer bar and the resize grip that sits in its corner.
	FooterMid uint32
	Grip      uint32
}

// WindowOptions says which chrome a window carries. The zero value is a plain
// window: no system buttons and no resizing, which is what a blocking screen
// wants — there is nowhere to go if the user closes character select.
type WindowOptions struct {
	// Closable and Minimizable put those buttons on the title bar.
	Closable    bool
	Minimizable bool

	// Resizable gives the window a footer with a grip in its corner.
	Resizable bool

	// BitmapBody leaves the body unpainted, for a window whose background is
	// an image the caller draws.
	//
	// It has to be an option rather than something the caller paints over,
	// because rectangles and images are drawn in separate batches: the body
	// fill lands on top of an image no matter which was asked for first.
	BitmapBody bool
}

// DefaultWindowOptions is the ordinary window: it can be closed and minimized,
// but not resized.
func DefaultWindowOptions() WindowOptions {
	return WindowOptions{Closable: true, Minimizable: true}
}

// SetWindowFrame installs the chrome that windows are drawn with. Passing nil
// returns them to the themed panel.
func (c *Context) SetWindowFrame(frame *WindowFrame) {
	c.windowFrame = frame
}

// WindowFrameActive reports whether windows are drawn from the original art.
func (c *Context) WindowFrameActive() bool {
	return c.windowFrame != nil
}

// frameTitleBarH is the title bar height in use, which decides where a
// window's content starts.
func (c *Context) frameTitleBarH() float32 {
	if c.windowFrame != nil {
		return FrameTitleH
	}

	if c.defaultSkin != nil && c.defaultSkin.Top > 0 {
		return float32(c.defaultSkin.Top)
	}

	// The themed panel's own bar, kept for the skinless fallback.
	return 25
}

// frameFooterHeight is the footer height in use. Only resizable windows have
// one, because the grip it carries is what makes them resizable.
func (c *Context) frameFooterHeight(ws *WindowState) float32 {
	if c.windowFrame == nil || !ws.Options.Resizable {
		return 0
	}

	return frameFooterH
}

// drawWindowFrame draws the chrome and handles the controls on it. It reports
// whether the window is still open, so a close click takes effect the same
// frame rather than leaving a dead window drawn.
func (c *Context) drawWindowFrame(ws *WindowState, title string) bool {
	f := c.windowFrame
	r := c.renderer

	// Title bar: caps at each end, the middle stretched between them. The art
	// is a vertical gradient with no horizontal variation, so stretching it
	// reads the same as tiling and costs one quad.
	midW := ws.W - 2*frameCapW
	if midW < 0 {
		midW = 0
	}

	r.DrawImage(f.TitleLeft, ws.X, ws.Y, frameCapW, FrameTitleH, ColorWhite)
	r.DrawImage(f.TitleMid, ws.X+frameCapW, ws.Y, midW, FrameTitleH, ColorWhite)
	r.DrawImage(f.TitleRight, ws.X+ws.W-frameCapW, ws.Y, frameCapW, FrameTitleH, ColorWhite)

	// Body.
	bodyY := ws.Y + FrameTitleH
	bodyH := ws.H - FrameTitleH - c.frameFooterHeight(ws)
	if bodyH < 0 {
		bodyH = 0
	}

	if !ws.BitmapBody {
		r.DrawRect(ws.X, bodyY, ws.W, bodyH, ColorWindowBody)
	}

	r.DrawRect(ws.X, bodyY, 1, bodyH, ColorPanelBorder)
	r.DrawRect(ws.X+ws.W-1, bodyY, 1, bodyH, ColorPanelBorder)

	if footerH := c.frameFooterHeight(ws); footerH > 0 {
		r.DrawImage(f.FooterMid, ws.X, bodyY+bodyH, ws.W, footerH, ColorWhite)
	} else {
		r.DrawRect(ws.X, bodyY+bodyH-1, ws.W, 1, ColorPanelBorder)
	}

	// Title text, left of the system buttons where the original puts it.
	if title != "" {
		scale := float32(0.7)
		ascent := r.FontAscent(scale)
		r.DrawText(ws.X+frameSysGap+2, ws.Y+(FrameTitleH-ascent)/2, title, scale, ColorTitleText)
	}

	open := true

	// System buttons, right to left: close, then minimize.
	btnY := ws.Y + (FrameTitleH-frameSysBtn)/2
	closeX := ws.X + ws.W - frameSysGap - frameSysBtn

	if ws.Options.Closable {
		if clicked, _ := c.ImageButtonAtEx(ws.ID+"_close", closeX, btnY, frameSysBtn, frameSysBtn,
			f.SysCloseOff, f.SysCloseOn, f.SysCloseOn); clicked {
			ws.Open = false
			open = false
		}
	} else {
		// Nothing to the right of the title, so the minimize button — if any —
		// takes the corner.
		closeX += frameSysGap + frameSysBtn
	}

	if ws.Options.Minimizable && f.SysMiniOff != 0 {
		miniX := closeX - frameSysGap - frameSysBtn
		if clicked, _ := c.ImageButtonAtEx(ws.ID+"_mini", miniX, btnY, frameSysBtn, frameSysBtn,
			f.SysMiniOff, f.SysMiniOn, f.SysMiniOn); clicked {
			ws.Minimized = !ws.Minimized
		}
	}

	if ws.Options.Resizable && f.Grip != 0 {
		c.windowResizeGrip(ws)
	}

	return open
}

// windowResizeGrip draws the corner grip and resizes the window while it is
// dragged. The grip is the whole resize affordance, as it is in the original.
func (c *Context) windowResizeGrip(ws *WindowState) {
	gripX := ws.X + ws.W - frameGrip - 1
	gripY := ws.Y + ws.H - frameGrip - 1

	c.renderer.DrawImage(c.windowFrame.Grip, gripX, gripY, frameGrip, frameGrip, ColorWhite)

	id := ws.ID + "_grip"
	grip := Rect{gripX, gripY, frameGrip, frameGrip}

	if c.input.MouseLeftPressed && grip.Contains(c.input.MouseX, c.input.MouseY) {
		c.activeWidget = id
	}

	if c.activeWidget != id {
		return
	}

	if c.input.MouseLeftDown {
		ws.W += c.input.MouseDeltaX
		ws.H += c.input.MouseDeltaY

		if ws.W < FrameMinW {
			ws.W = FrameMinW
		}

		if ws.H < FrameMinH {
			ws.H = FrameMinH
		}

		ws.Resized = true
	}

	if c.input.MouseLeftReleased {
		c.activeWidget = ""
	}
}

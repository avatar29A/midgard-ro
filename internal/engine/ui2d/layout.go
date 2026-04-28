package ui2d

// Declarative UI layout.
//
// The widget functions in this file build a tree of `Element`s that gets
// measured then drawn into a rect — replacing the imperative cursor/Row/
// Spacer model whose mutable state was the source of every alignment bug
// we hit. Each element knows its desired size; containers compute precise
// child rects in one pass.
//
// Pattern matches korangar's `window! { elements: (...) }` DSL: a tree of
// pure-data nodes plus a small set of container types (VStack, HStack,
// Padding, etc.) that lay out their children deterministically.
//
//	tree := ui2d.VStack(8,
//	    ui2d.Label("Username:"),
//	    ui2d.Sized(0, 28, ui2d.TextInput("user", &username, nil)),
//	    ui2d.Spacer(8),
//	    ui2d.Sized(0, 34, ui2d.Button("login", "Login", onLogin)),
//	)
//	ctx.RenderTree(tree, contentRect)
//
// Element.Measure returns desired (w, h). A returned width of 0 means
// "take whatever the container offers" — VStack always passes its full
// width down regardless, so this is mostly a hint for HStack layouts.

// Element is a UI tree node. Implementations are typically constructed via
// the package-level helpers (VStack, Label, Button, …).
type Element interface {
	Measure() (w, h float32)
	Draw(ctx *Context, rect Rect)
}

// RenderTree measures `root` and draws it into the given rect. This is the
// entry point that screens use after BeginWindow to render their body.
func (c *Context) RenderTree(root Element, rect Rect) {
	if root == nil {
		return
	}
	root.Draw(c, rect)
}

// ---------- Containers ----------

// VStack stacks children vertically with `gap` pixels between them. Each
// child is given the full container width.
func VStack(gap float32, children ...Element) Element {
	return &vstack{gap: gap, children: children}
}

type vstack struct {
	gap      float32
	children []Element
}

func (v *vstack) Measure() (float32, float32) {
	var maxW, totalH float32
	for i, c := range v.children {
		cw, ch := c.Measure()
		if cw > maxW {
			maxW = cw
		}
		totalH += ch
		if i < len(v.children)-1 {
			totalH += v.gap
		}
	}
	return maxW, totalH
}

// Draw stacks children top-to-bottom, sharing remaining vertical space
// equally among any "flex" children (Measure returning h == 0, e.g. a
// Filler element used to push trailing children to the bottom).
func (v *vstack) Draw(ctx *Context, rect Rect) {

	var totalFixed float32
	var flexCount int
	for _, c := range v.children {
		_, ch := c.Measure()
		if ch <= 0 {
			flexCount++
		} else {
			totalFixed += ch
		}
	}
	flexH := float32(0)
	if flexCount > 0 {
		gaps := v.gap * float32(len(v.children)-1)
		if remaining := rect.H - totalFixed - gaps; remaining > 0 {
			flexH = remaining / float32(flexCount)
		}
	}

	y := rect.Y
	for i, c := range v.children {
		_, ch := c.Measure()
		if ch <= 0 {
			ch = flexH
		}
		c.Draw(ctx, Rect{rect.X, y, rect.W, ch})
		y += ch
		if i < len(v.children)-1 {
			y += v.gap
		}
	}
}

// HStack arranges children horizontally with `gap` between. Each child is
// drawn at its desired width; trailing space is left empty.
func HStack(gap float32, children ...Element) Element {
	return &hstack{gap: gap, children: children}
}

type hstack struct {
	gap      float32
	children []Element
}

func (h *hstack) Measure() (float32, float32) {
	var totalW, maxH float32
	for i, c := range h.children {
		cw, ch := c.Measure()
		totalW += cw
		if ch > maxH {
			maxH = ch
		}
		if i < len(h.children)-1 {
			totalW += h.gap
		}
	}
	return totalW, maxH
}

// Draw lays out children left-to-right, sharing remaining horizontal space
// equally among any "flex" children (those whose Measure returns w == 0,
// e.g. raw inputs or the Filler element). Fixed-width children get their
// measured width; flex children split whatever's left.
func (h *hstack) Draw(ctx *Context, rect Rect) {

	var totalFixed float32
	var flexCount int
	for _, c := range h.children {
		cw, _ := c.Measure()
		if cw <= 0 {
			flexCount++
		} else {
			totalFixed += cw
		}
	}
	flexW := float32(0)
	if flexCount > 0 {
		gaps := h.gap * float32(len(h.children)-1)
		if remaining := rect.W - totalFixed - gaps; remaining > 0 {
			flexW = remaining / float32(flexCount)
		}
	}

	x := rect.X
	for i, c := range h.children {
		cw, _ := c.Measure()
		if cw <= 0 {
			cw = flexW
		}
		c.Draw(ctx, Rect{x, rect.Y, cw, rect.H})
		x += cw
		if i < len(h.children)-1 {
			x += h.gap
		}
	}
}

// Padding wraps a child with uniform padding on all sides.
func Padding(p float32, child Element) Element {
	return &padding{t: p, r: p, b: p, l: p, child: child}
}

// PaddingXY wraps with horizontal `x` and vertical `y` padding.
func PaddingXY(x, y float32, child Element) Element {
	return &padding{t: y, r: x, b: y, l: x, child: child}
}

type padding struct {
	t, r, b, l float32
	child      Element
}

func (p *padding) Measure() (float32, float32) {
	cw, ch := p.child.Measure()
	return cw + p.l + p.r, ch + p.t + p.b
}

func (p *padding) Draw(ctx *Context, rect Rect) {
	p.child.Draw(ctx, Rect{
		X: rect.X + p.l,
		Y: rect.Y + p.t,
		W: rect.W - p.l - p.r,
		H: rect.H - p.t - p.b,
	})
}

// Spacer is a fixed-height vertical gap (or fixed-width horizontal gap, in
// an HStack — measure returns 0 width, so use SpacerH for that).
func Spacer(h float32) Element { return &spacer{h: h} }

// SpacerW is a horizontal spacer for HStacks.
func SpacerW(w float32) Element { return &spacer{w: w} }

// Filler is a zero-measured element that flexes to fill remaining space
// in an HStack (or width in a VStack). Use it to push trailing children
// to the right edge: HStack(8, Filler(), Button(...), Button(...)).
func Filler() Element { return &spacer{} }

type spacer struct{ w, h float32 }

func (s *spacer) Measure() (float32, float32) { return s.w, s.h }
func (s *spacer) Draw(ctx *Context, rect Rect) {
}

// Sized forces a child to a specific (w, h). A zero dimension means
// "inherit from the container" — common for inputs that want fixed height
// but flexible width.
func Sized(w, h float32, child Element) Element {
	return &sized{w: w, h: h, child: child}
}

type sized struct {
	w, h  float32
	child Element
}

func (s *sized) Measure() (float32, float32) {
	cw, ch := s.child.Measure()
	if s.w > 0 {
		cw = s.w
	}
	if s.h > 0 {
		ch = s.h
	}
	return cw, ch
}

func (s *sized) Draw(ctx *Context, rect Rect) {
	r := rect
	if s.w > 0 {
		r.W = s.w
	}
	if s.h > 0 {
		r.H = s.h
	}
	s.child.Draw(ctx, r)
}

// Center horizontally centers a child inside the rect at the child's
// desired width. The child still gets its measured width, not the
// container's, so use this for buttons / labels that should be narrower
// than their row.
func Center(child Element) Element {
	return &center{child: child}
}

type center struct{ child Element }

func (c *center) Measure() (float32, float32) { return c.child.Measure() }

func (c *center) Draw(ctx *Context, rect Rect) {
	cw, ch := c.child.Measure()
	if cw <= 0 || cw > rect.W {
		cw = rect.W
	}
	if ch <= 0 || ch > rect.H {
		ch = rect.H
	}
	c.child.Draw(ctx, Rect{
		X: rect.X + (rect.W-cw)/2,
		Y: rect.Y + (rect.H-ch)/2,
		W: cw,
		H: ch,
	})
}

// ---------- Leaves ----------

// Label is body text styled in the RO chrome navy with a fake-bold pass
// (text drawn twice at a 1px x-offset). Use LabelColor to override the
// default navy when you need a soft / dim / error tint.
func Label(text string) Element {
	return &label{text: text, color: ColorTitleText, bold: true}
}

// LabelColor is a Label in a custom color.
func LabelColor(text string, color Color) Element {
	return &label{text: text, color: color}
}

// LabelCenteredEl draws text horizontally centered in its rect.
func LabelCenteredEl(text string) Element {
	return &label{text: text, color: ColorText, centered: true}
}

type label struct {
	text     string
	color    Color
	centered bool
	bold     bool
}

func (l *label) Measure() (float32, float32) {
	// Width is taken from the container in Draw; height is the font line
	// height so VStack spacing accounts for the actual visual size.
	return 0, lineHeightAtScale1
}

func (l *label) Draw(ctx *Context, rect Rect) {
	scale := float32(1.0)
	x := rect.X
	if l.centered {
		w, _ := ctx.renderer.MeasureText(l.text, scale)
		x = rect.X + (rect.W-w)/2
		if x < rect.X {
			x = rect.X
		}
	}
	ctx.renderer.DrawText(x, rect.Y, l.text, scale, l.color)
	if l.bold {
		// Fake-bold: re-draw 1px to the right to thicken strokes without
		// needing a bold TTF face (our pipeline doesn't load one).
		ctx.renderer.DrawText(x+1, rect.Y, l.text, scale, l.color)
	}
}

// lineHeightAtScale1 is the body font's line height. Captured in Measure
// without a Context handle by reading the renderer at first use; falls
// back to a sane default if the font isn't loaded yet.
const lineHeightAtScale1 = 18

// Button is a clickable button with the given label. onClick is called
// once per click; it may be nil if the caller checks state separately.
func Button(id, label string, onClick func()) Element {
	return &button{id: id, label: label, onClick: onClick}
}

type button struct {
	id      string
	label   string
	onClick func()
}

func (b *button) Measure() (float32, float32) { return 0, 32 }

func (b *button) Draw(ctx *Context, rect Rect) {
	if ctx.ButtonAt(b.id, rect.X, rect.Y, rect.W, rect.H, b.label) {
		if b.onClick != nil {
			b.onClick()
		}
	}
}

// TextInput edits a string. The pointed-to value is mutated in place on
// every changed frame; onSubmit (optional) fires when the user presses
// Enter while focused.
func TextInput(id string, value *string, onSubmit func()) Element {
	return &textInput{id: id, value: value, onSubmit: onSubmit, masked: false}
}

// PasswordInput is TextInput rendered with masking.
func PasswordInput(id string, value *string, onSubmit func()) Element {
	return &textInput{id: id, value: value, onSubmit: onSubmit, masked: true}
}

type textInput struct {
	id       string
	value    *string
	onSubmit func()
	masked   bool
}

func (t *textInput) Measure() (float32, float32) { return 0, 28 }

func (t *textInput) Draw(ctx *Context, rect Rect) {
	var newVal string
	var changed, submitted bool
	if t.masked {
		newVal, changed, submitted = ctx.PasswordInputAt(t.id, rect.X, rect.Y, rect.W, rect.H, *t.value)
	} else {
		newVal, changed, submitted = ctx.TextInputAt(t.id, rect.X, rect.Y, rect.W, rect.H, *t.value)
	}
	if changed {
		*t.value = newVal
	}
	if submitted && t.onSubmit != nil {
		t.onSubmit()
	}
}

// ProgressBar renders a horizontal progress bar with an optional label
// (typically a percentage). `fraction` is clamped to [0, 1].
func ProgressBar(fraction float32, label string) Element {
	return &progressBar{fraction: fraction, label: label}
}

type progressBar struct {
	fraction float32
	label    string
}

func (p *progressBar) Measure() (float32, float32) { return 0, 22 }

func (p *progressBar) Draw(ctx *Context, rect Rect) {
	ctx.ProgressBarAt(rect.X, rect.Y, rect.W, rect.H, p.fraction, p.label)
}

// Selectable is a list-row that highlights when `selected` is true and
// fires onSelect when clicked. Use inside a VStack for character lists,
// inventory rows, etc.
func Selectable(id, label string, selected bool, onSelect func()) Element {
	return &selectable{id: id, label: label, selected: selected, onSelect: onSelect}
}

type selectable struct {
	id       string
	label    string
	selected bool
	onSelect func()
}

func (s *selectable) Measure() (float32, float32) { return 0, 22 }

func (s *selectable) Draw(ctx *Context, rect Rect) {
	if ctx.SelectableAt(s.id, rect.X, rect.Y, rect.W, rect.H, s.label, s.selected) {
		if s.onSelect != nil {
			s.onSelect()
		}
	}
}

// ImageEl draws a textured quad (typically a logo or icon).
func ImageEl(texID uint32, w, h float32, tint Color) Element {
	return &imageEl{texID: texID, w: w, h: h, tint: tint}
}

type imageEl struct {
	texID uint32
	w, h  float32
	tint  Color
}

func (i *imageEl) Measure() (float32, float32) { return i.w, i.h }

func (i *imageEl) Draw(ctx *Context, rect Rect) {
	if i.texID == 0 {
		return
	}
	ctx.renderer.DrawImage(i.texID, rect.X, rect.Y, rect.W, rect.H, i.tint)
}

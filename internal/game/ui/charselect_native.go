package ui

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/engine/charsprite"
	"github.com/Faultbox/midgard-ro/internal/engine/ui2d"
	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// The original character select is a single 576x342 texture with the slot
// frames, the stat table's labels and the artwork painted in. Only the values,
// the slot highlight and the buttons are drawn on top. These offsets are the
// original's, cross-checked against roBrowser's transcription.
const (
	charSelWinW = float32(576)
	charSelWinH = float32(342)

	// Where the character stands: the center of the shadow ellipse painted
	// into the slot, measured off the texture. It sits a few pixels left of
	// the slot's own center, which is why centering on the slot looked off.
	charSelShadowX = float32(63.5)
	charSelShadowY = float32(119)

	// Keeps a tall sprite inside the slot frame rather than over its border.
	charSelSlotInset = float32(6)

	// Facing the viewer, which is the pose character select shows.
	charSelFacing = 0

	// Slots. The highlight is drawn 5px left of the slot it marks.
	charSelSlotW      = float32(139)
	charSelSlotH      = float32(144)
	charSelSlotY      = float32(40)
	charSelSlotShiftX = float32(5)

	// Stat table: two columns of rows 16px apart, starting below the slots.
	charSelInfoX   = float32(16)
	charSelInfoY   = float32(204)
	charSelValueX  = float32(52)  // first column, relative to the block
	charSelStatX   = float32(200) // second column
	charSelRowStep = float32(16)
	charSelMapRow  = float32(98)

	// The artwork's first label has its cap 3px below the block's top, and
	// each row is charSelRowStep below the last. Measured off the texture.
	charSelRowTop = float32(3)

	// DrawText positions the line box, whose top sits this far above the
	// glyph caps at this scale. Measured against the painted labels, which the
	// values have to line up with.
	charSelTextLift = float32(4.5)

	// The window's Korean title, painted into the art. The icon to its left
	// stays; the glyphs are covered with a clean column of the title bar and
	// the title redrawn, as on the login window.
	charSelTitleX      = float32(19)
	charSelTitleW      = float32(64)
	charSelTitleY      = float32(1)
	charSelTitleH      = float32(12)
	charSelTitleCleanX = float32(400)

	// Buttons along the bottom edge.
	charSelBtnY      = charSelWinH - 4 - loginBtnH
	charSelOkRight   = float32(50)
	charSelCancelRig = float32(4)
)

// charSelSlotCount is how many slots the screen shows at once. The original
// pages through an account's slots in threes and so do we.
const charSelSlotCount = 3

// Paging arrows, drawn either side of the slot row. The art is the matched
// left/right pair from the creation screen — 38x20, three states — because
// character select's own directory has only a `next`, and one arrow cannot
// say "back".
const (
	charSelArrowW   = float32(38)
	charSelArrowH   = float32(20)
	charSelArrowGap = float32(6)
	charSelArrowY   = charSelSlotY + (charSelSlotH-charSelArrowH)/2
)

// charSelSlotX are the three slots' left edges.
var charSelSlotX = [3]float32{60, 224, 386}

// charSelectSkin holds the original character select art.
type charSelectSkin struct {
	window *TextureInfo
	box    *TextureInfo

	ok, make, cancel *TextureInfo

	// prev and next page the slot row. Optional: they are loaded separately
	// from the rest, because a miss here should cost the paging arrows and
	// not the whole screen.
	prev, next *TextureInfo
}

// loadCharSelectSkin loads the character select art. A miss leaves the skin
// nil and the caller falls back to the themed window.
func (b *UI2DBackend) loadCharSelectSkin() *charSelectSkin {
	if b.charSelSkin != nil {
		return b.charSelSkin
	}

	if b.charSelTried {
		return nil
	}

	b.charSelTried = true

	loaded := make([]*TextureInfo, 0, 5)

	for _, name := range []string{
		`win_select.bmp`, `box_select.bmp`,
		`btn_ok.bmp`, `btn_make.bmp`, `btn_cancel.bmp`,
	} {
		tex, err := b.texCache.Load(loginTexBasePath + name)
		if err != nil {
			logger.Warn("character select art unavailable, falling back to the themed window",
				zap.String("path", loginTexBasePath+name), zap.Error(err))

			return nil
		}

		loaded = append(loaded, tex)
	}

	b.charSelSkin = &charSelectSkin{
		window: loaded[0],
		box:    loaded[1],
		ok:     loaded[2],
		make:   loaded[3],
		cancel: loaded[4],
	}

	// Optional, and deliberately not in the loop above: without these the
	// screen still works, it just cannot page. Losing character select
	// entirely over a missing arrow would be the worse trade.
	b.charSelSkin.prev = b.optionalTexture(makeCharTexBasePath + `arrow_box_l_out.bmp`)
	b.charSelSkin.next = b.optionalTexture(makeCharTexBasePath + `arrow_box_r_out.bmp`)

	return b.charSelSkin
}

// optionalTexture loads art the screen can do without, reporting a miss at
// warn rather than failing. A swallowed miss is what once left the login
// screen black with nothing in the log to say why.
func (b *UI2DBackend) optionalTexture(path string) *TextureInfo {
	tex, err := b.texCache.Load(path)
	if err != nil {
		logger.Warn("optional character select art unavailable",
			zap.String("path", path), zap.Error(err))

		return nil
	}

	return tex
}

// renderNativeCharSelect draws the original character select and reports
// whether it handled the screen.
func (b *UI2DBackend) renderNativeCharSelect(state CharSelectUIState, width, height float32) bool {
	skin := b.loadCharSelectSkin()
	if skin == nil {
		return false
	}

	if !b.charSelPlaced {
		b.charSelX = float32(int((width - charSelWinW) / 2))
		b.charSelY = float32(int((height - charSelWinH) / 2))
		b.charSelPlaced = true
	}

	b.ctx.DragHandle("charselect_titlebar",
		ui2d.Rect{X: b.charSelX, Y: b.charSelY, W: charSelWinW, H: ui2d.FrameTitleH},
		&b.charSelX, &b.charSelY)

	x, y := b.charSelX, b.charSelY
	r := b.ctx.Renderer()

	r.DrawImage(skin.window.ID, x, y, charSelWinW, charSelWinH, ui2d.ColorWhite)
	b.drawCharSelectTitle(skin, x, y)

	// Auto-select the first character once the list arrives.
	if state.IsReady && b.charSelectIdx < 0 && len(state.Characters) > 0 {
		b.charSelectIdx = 0

		if state.OnSelectIndex != nil {
			state.OnSelectIndex(0)
		}
	}

	b.charSelectKeys(state)
	b.drawCharSelectSlots(skin, state, x, y)
	b.drawCharSelectPaging(skin, state, x, y)
	b.drawCharSelectInfo(state, x, y)
	b.drawCharSelectButtons(skin, state, x, y)

	// Status and errors have no home in the artwork, so they go underneath.
	if msg := charSelectMessage(state); msg != "" {
		r.DrawText(x, y+charSelWinH+8, msg, loginTextScale, ui2d.ColorText)
	}

	return true
}

// drawCharSelectTitle replaces the window's baked Korean title with an
// English one.
func (b *UI2DBackend) drawCharSelectTitle(skin *charSelectSkin, x, y float32) {
	r := b.ctx.Renderer()

	u := charSelTitleCleanX / charSelWinW
	r.DrawImageUV(skin.window.ID,
		x+charSelTitleX, y+charSelTitleY, charSelTitleW, charSelTitleH,
		u, charSelTitleY/charSelWinH,
		u+1/charSelWinW, (charSelTitleY+charSelTitleH)/charSelWinH,
		ui2d.ColorWhite)

	ascent := r.FontAscent(loginTextScale)
	r.DrawText(x+charSelTitleX, y+charSelTitleY+(charSelTitleH-ascent)/2,
		"Character Select", loginTextScale, ui2d.ColorTitleText)
}

// charSelectMessage picks what to say under the window, if anything.
func charSelectMessage(state CharSelectUIState) string {
	switch {
	case state.ErrorMessage != "":
		return state.ErrorMessage
	case state.StatusMessage != "":
		return state.StatusMessage
	case !state.IsReady:
		return "Loading character list..."
	case len(state.Characters) == 0:
		return "No characters on this account."
	default:
		return ""
	}
}

// drawCharSelectSlots highlights the selected slot and makes all three
// clickable, whether or not they hold a character.
//
// An empty slot answers too. It used to be skipped outright, which left it
// with no rect and nothing drawn — the screen showed only the shadow ellipse
// painted into the background, and there was no way to say "I want this one".
// Selecting an empty slot is what puts Make under the pointer, which is how
// the original says a slot is free.
func (b *UI2DBackend) drawCharSelectSlots(skin *charSelectSkin, state CharSelectUIState, x, y float32) {
	for pos, slotX := range charSelSlotX {
		// The slot on screen is a position within the page, not the slot
		// itself. charSelectIdx is always the real slot number, so that
		// paging cannot silently point selection at a different character.
		slot := b.charSelectPage*charSelSlotCount + pos
		if slot >= b.charSelectSlotCount(state) {
			continue
		}

		left := x + slotX
		top := y + charSelSlotY
		hasChar := slot < len(state.Characters)

		if slot == b.charSelectIdx {
			b.ctx.Renderer().DrawImage(skin.box.ID,
				left-charSelSlotShiftX, top, charSelSlotW, charSelSlotH, ui2d.ColorWhite)
		}

		if hasChar {
			b.drawCharSelectPortrait(state.Characters[slot], left, top)
		}

		// While the list is still coming, every slot looks empty because there
		// is nothing to draw in it. Offering creation then would offer it on
		// slots that are actually occupied.
		offersCreate := state.IsReady && !hasChar

		// Double click first: it must be seen whether or not the press also
		// counts as a select, and on an empty slot it is the shortcut
		// straight into creation.
		rect := ui2d.Rect{X: left, Y: top, W: charSelSlotW, H: charSelSlotH}
		if b.ctx.DoubleClickedIn(fmt.Sprintf("charselect_slot_dbl_%d", slot), rect) {
			trace.Emit(trace.Char, "slot-doubleclick",
				zap.Int("slot", slot), zap.Bool("empty", !hasChar))

			switch {
			case offersCreate && state.OnCreateSlot != nil:
				state.OnCreateSlot(slot)
			case hasChar && state.OnSelect != nil:
				// Double-clicking someone plays them, which is what the
				// gesture means everywhere else on this screen.
				b.charSelectIdx = slot
				state.OnSelect(slot)
			}
		}

		if b.ctx.InvisibleButtonAt(fmt.Sprintf("charselect_slot_%d", slot),
			left, top, charSelSlotW, charSelSlotH) {
			b.charSelectIdx = slot

			trace.Emit(trace.Char, "slot-click",
				zap.Int("slot", slot), zap.Bool("empty", !hasChar))

			// Only a slot holding a character has one to report.
			if hasChar && state.OnSelectIndex != nil {
				state.OnSelectIndex(slot)
			}
		}
	}
}

// charSelectKeys moves the selection with the arrow keys and acts on it with
// Enter.
//
// The selection is a slot number rather than a position, so moving off the
// edge of a page turns the page under it — otherwise the arrows would stop at
// slot 2 and the other six would only be reachable with the mouse.
func (b *UI2DBackend) charSelectKeys(state CharSelectUIState) {
	if !state.IsReady {
		return
	}

	in := b.ctx.Input()
	if in == nil {
		return
	}

	slots := b.charSelectSlotCount(state)

	switch {
	case in.KeyLeftPressed:
		b.moveCharSelection(-1, slots)
	case in.KeyRightPressed:
		b.moveCharSelection(1, slots)
	}

	if !in.KeyEnterPressed {
		return
	}

	slot := b.charSelectIdx
	if slot < 0 || slot >= slots {
		return
	}

	// Enter does what a double click does, so the two gestures cannot
	// disagree about what a slot means.
	if slot < len(state.Characters) {
		if state.OnSelect != nil {
			trace.Emit(trace.Char, "enter-key", zap.Int("slot", slot), zap.Bool("empty", false))
			state.OnSelect(slot)
		}

		return
	}

	if state.OnCreateSlot != nil {
		trace.Emit(trace.Char, "enter-key", zap.Int("slot", slot), zap.Bool("empty", true))
		state.OnCreateSlot(slot)
	}
}

// moveCharSelection steps the selection by one slot and brings its page with
// it. Stops at both ends rather than wrapping: an account's slots are a row,
// not a ring, and wrapping from the last to the first reads as a mis-click.
func (b *UI2DBackend) moveCharSelection(delta, slots int) {
	next := b.charSelectIdx + delta
	if b.charSelectIdx < 0 {
		next = 0
	}

	if next < 0 || next >= slots {
		return
	}

	b.charSelectIdx = next
	b.charSelectPage = next / charSelSlotCount

	trace.Emit(trace.Char, "select-move", zap.Int("slot", next), zap.Int("page", b.charSelectPage))
}

// charSelectSlotCount is how many slots the screen may page over.
//
// Falls back to what is on screen at once when the account's own count has
// not arrived, so a missing HC_ACCEPT_ENTER2 shows three slots rather than
// none.
func (b *UI2DBackend) charSelectSlotCount(state CharSelectUIState) int {
	if state.CreatableSlots > 0 {
		return state.CreatableSlots
	}

	return charSelSlotCount
}

// charSelectPageCount is how many pages the slots divide into.
func (b *UI2DBackend) charSelectPageCount(state CharSelectUIState) int {
	slots := b.charSelectSlotCount(state)

	return (slots + charSelSlotCount - 1) / charSelSlotCount
}

// drawCharSelectPaging draws the arrows either side of the slot row and moves
// between pages.
//
// Drawn only when there is more than one page and only when the art loaded:
// an account with three slots has nothing to page through, and an arrow that
// cannot be drawn must not still be clickable.
func (b *UI2DBackend) drawCharSelectPaging(skin *charSelectSkin, state CharSelectUIState, x, y float32) {
	pages := b.charSelectPageCount(state)
	if pages <= 1 || skin.prev == nil || skin.next == nil {
		return
	}

	// Clamp first: the account's slot count arrives after the screen has
	// already drawn once, and it can only shrink the range.
	if b.charSelectPage >= pages {
		b.charSelectPage = pages - 1
	}
	if b.charSelectPage < 0 {
		b.charSelectPage = 0
	}

	r := b.ctx.Renderer()
	arrowY := y + charSelArrowY
	prevX := x + charSelSlotX[0] - charSelArrowW - charSelArrowGap
	nextX := x + charSelSlotX[len(charSelSlotX)-1] + charSelSlotW + charSelArrowGap

	if b.charSelectPage > 0 {
		r.DrawImage(skin.prev.ID, prevX, arrowY, charSelArrowW, charSelArrowH, ui2d.ColorWhite)

		if b.ctx.InvisibleButtonAt("charselect_prev", prevX, arrowY, charSelArrowW, charSelArrowH) {
			b.charSelectPage--
			trace.Emit(trace.Char, "page", zap.Int("page", b.charSelectPage))
		}
	}

	if b.charSelectPage < pages-1 {
		r.DrawImage(skin.next.ID, nextX, arrowY, charSelArrowW, charSelArrowH, ui2d.ColorWhite)

		if b.ctx.InvisibleButtonAt("charselect_next", nextX, arrowY, charSelArrowW, charSelArrowH) {
			b.charSelectPage++
			trace.Emit(trace.Char, "page", zap.Int("page", b.charSelectPage))
		}
	}
}

// drawCharSelectInfo fills in the stat table. The labels are painted into the
// window, so only the values are drawn.
func (b *UI2DBackend) drawCharSelectInfo(state CharSelectUIState, x, y float32) {
	if b.charSelectIdx < 0 || b.charSelectIdx >= len(state.Characters) {
		return
	}

	char := state.Characters[b.charSelectIdx]
	r := b.ctx.Renderer()

	row := func(index float32) float32 {
		return y + charSelInfoY + charSelRowTop + index*charSelRowStep - charSelTextLift
	}

	left := x + charSelInfoX + charSelValueX
	right := x + charSelInfoX + charSelStatX

	for i, value := range []string{
		char.GetName(),
		getJobName(char.Class),
		fmt.Sprintf("%d", char.BaseLevel),
		fmt.Sprintf("%d", char.BaseExp),
		fmt.Sprintf("%d/%d", char.HP, char.MaxHP),
		fmt.Sprintf("%d/%d", char.SP, char.MaxSP),
	} {
		r.DrawText(left, row(float32(i)), value, loginTextScale, ui2d.ColorText)
	}

	for i, value := range []uint8{char.Str, char.Agi, char.Vit, char.Int, char.Dex, char.Luk} {
		r.DrawText(right, row(float32(i)), fmt.Sprintf("%d", value), loginTextScale, ui2d.ColorText)
	}

	mapY := y + charSelInfoY + charSelMapRow - charSelTextLift
	r.DrawText(left, mapY, char.GetMapName(), loginTextScale, ui2d.ColorTextDim)
}

// drawCharSelectButtons draws the bottom row. The original shows Make on an
// empty slot and Ok on a filled one, in the same place.
func (b *UI2DBackend) drawCharSelectButtons(skin *charSelectSkin, state CharSelectUIState, x, y float32) {
	btnY := y + charSelBtnY
	actionX := x + charSelWinW - charSelOkRight - loginBtnW
	cancelX := x + charSelWinW - charSelCancelRig - loginBtnW

	hasChar := b.charSelectIdx >= 0 && b.charSelectIdx < len(state.Characters)

	if hasChar {
		if b.skinButton("charselect_ok", actionX, btnY, skin.ok, skin.ok, skin.ok, "Ok") {
			if state.OnSelect != nil {
				state.OnSelect(b.charSelectIdx)
			}
		}
	} else {
		// Character creation is not implemented yet; the button is drawn
		// because the slot is empty and the original offers it there.
		if b.skinButton("charselect_make", actionX, btnY, skin.make, skin.make, skin.make, "Make") {
			trace.Emit(trace.Char, "make-click", zap.Int("slot", b.charSelectIdx))

			if b.charSelectIdx >= 0 && state.OnCreateSlot != nil {
				state.OnCreateSlot(b.charSelectIdx)
			}
		}
	}

	b.skinButton("charselect_cancel", cancelX, btnY, skin.cancel, skin.cancel, skin.cancel, "Cancel")
}

// charSelectPortrait is one character's standing frame, uploaded once.
type charSelectPortrait struct {
	texture       uint32
	width, height float32
}

// portraitFor bakes a character's idle frame into a texture, once per look.
// A character whose sprite will not load simply has an empty slot; the screen
// stays usable, which matters more than the picture.
func (b *UI2DBackend) portraitFor(char *packets.CharInfo) *charSelectPortrait {
	return b.portraitForSpec(charsprite.Spec{
		Job:       int(char.Class),
		Female:    char.Sex == 0,
		HairStyle: int(char.HairStyle),
	}, charSelFacing)
}

// portraitForSpec bakes one look's frame into a texture, once per look and
// facing.
//
// Split from portraitFor so a look that has no character behind it yet can be
// drawn too — which is the whole of the creation screen's preview.
func (b *UI2DBackend) portraitForSpec(spec charsprite.Spec, facing int) *charSelectPortrait {
	if b.charSelPortraits == nil {
		b.charSelPortraits = make(map[portraitKey]*charSelectPortrait)
	}

	key := portraitKey{spec: spec, facing: facing}
	if portrait, ok := b.charSelPortraits[key]; ok {
		return portrait
	}

	// Remember the failure too, so a missing sprite is looked up once rather
	// than every frame.
	b.charSelPortraits[key] = nil

	if b.assetLoader == nil {
		return nil
	}

	assets, err := charsprite.Load(b.assetLoader, spec)
	if err != nil {
		logger.Warn("no character sprite for the select screen",
			zap.Int("job", spec.Job), zap.Bool("female", spec.Female), zap.Error(err))

		return nil
	}

	// The idle frame facing the viewer is the pose the original shows.
	frames := assets.Sheet.Frames[charsprite.ActionIdle*charsprite.Directions+facing]
	if len(frames) == 0 {
		logger.Warn("character sprite has no idle frame",
			zap.Int("job", spec.Job), zap.String("path", assets.BodyPath))

		return nil
	}

	portrait := &charSelectPortrait{
		texture: b.ctx.Renderer().CreateTextureNearest(
			assets.Sheet.Width, assets.Sheet.Height, frames[0].Pixels),
		width:  float32(assets.Sheet.Width),
		height: float32(assets.Sheet.Height),
	}

	b.charSelPortraits[key] = portrait

	return portrait
}

// portraitKey is what a baked portrait is cached under. Facing is part of it
// because the creation screen turns the preview, and each direction is a
// different frame.
type portraitKey struct {
	spec   charsprite.Spec
	facing int
}

// drawCharSelectPortrait stands a character on the shadow painted in its slot.
//
// charsprite pads every frame horizontally centered and bottom aligned, so the
// character's feet are the bottom edge of the quad and its middle is the quad's
// center. Placing that bottom edge on the shadow puts the character on the
// ground rather than at a fixed offset from the slot's top, which left it
// hanging below the shadow.
func (b *UI2DBackend) drawCharSelectPortrait(char *packets.CharInfo, slotLeft, slotTop float32) {
	portrait := b.portraitFor(char)
	if portrait == nil {
		return
	}

	w, h := portrait.width, portrait.height

	// A sprite taller than the room above the shadow is scaled to fit, keeping
	// its proportions so it still stands on the same spot.
	if maxH := charSelShadowY - charSelSlotInset; h > maxH {
		w *= maxH / h
		h = maxH
	}

	if maxW := charSelSlotW - 2*charSelSlotInset; w > maxW {
		h *= maxW / w
		w = maxW
	}

	b.ctx.Renderer().DrawImage(portrait.texture,
		slotLeft+charSelShadowX-w/2, slotTop+charSelShadowY-h, w, h, ui2d.ColorWhite)
}

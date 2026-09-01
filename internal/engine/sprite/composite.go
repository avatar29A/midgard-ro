// Package sprite provides sprite compositing and rendering utilities.
package sprite

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// CompositeResult holds the result of sprite compositing.
type CompositeResult struct {
	Pixels []byte // RGBA pixels
	Width  int    // Image width
	Height int    // Image height

	// OffsetX and OffsetY place the image against the sprite's own origin:
	// they are where its top-left corner sits relative to that origin, and are
	// negative for the usual sprite drawn around it. Callers that stand a
	// sprite on the ground can ignore them, but anything that has to line the
	// origin up with a point on screen — a cursor on the mouse — needs them,
	// because the image is cropped to its content and would otherwise have
	// lost where that origin went.
	OffsetX int
	OffsetY int
}

// CompositeSprites creates a single RGBA image by compositing body and head
// sprites. It uses anchor points to correctly position the head relative to
// the body.
//
// bodyFrame and headFrame are indexed independently because they mean
// different things. The body's frames are animation; the head sprite's three
// frames are RO's head *directions* (straight, and turned each way), which the
// player holds steady rather than cycling. Passing the body's frame index for
// both makes the head swivel on its own — constantly while standing, and out
// of step with the legs while walking.
//
// headSPR/headACT may both be nil, in which case only the body is drawn —
// monster and NPC sprites are single-piece, and a character whose head sprite
// is missing from the archive should still render.
func CompositeSprites(
	bodySPR *formats.SPR, bodyACT *formats.ACT,
	headSPR *formats.SPR, headACT *formats.ACT,
	action, direction, bodyFrame, headFrame int,
) CompositeResult {
	return CompositeWithWeapon(bodySPR, bodyACT, headSPR, headACT, nil, nil,
		action, direction, bodyFrame, headFrame)
}

// CompositeWithWeapon is CompositeSprites with a weapon held in the character's
// hand.
//
// The weapon is a third sprite aligned the same way the head is — its own
// anchor onto the body's — and it runs on the body's frame rather than a pose
// of its own, because a swing is one motion the two halves share. Passing a
// nil weapon is the bare-handed case and costs nothing.
//
// Drawn last, over the body and the head. RO's weapon art is drawn per
// facing, so the sprite for a character turned away already looks like a
// weapon seen from behind; there is nothing to reorder per direction.
func CompositeWithWeapon(
	bodySPR *formats.SPR, bodyACT *formats.ACT,
	headSPR *formats.SPR, headACT *formats.ACT,
	weaponSPR *formats.SPR, weaponACT *formats.ACT,
	action, direction, bodyFrame, headFrame int,
) CompositeResult {
	if bodySPR == nil || bodyACT == nil || len(bodyACT.Actions) == 0 {
		return CompositeResult{}
	}

	// Get body action/frame
	bodyActionIdx := action*8 + direction
	if bodyActionIdx >= len(bodyACT.Actions) {
		bodyActionIdx = direction % len(bodyACT.Actions)
	}
	bodyAction := &bodyACT.Actions[bodyActionIdx]
	if len(bodyAction.Frames) == 0 {
		return CompositeResult{}
	}
	bodyFrameIdx := bodyFrame % len(bodyAction.Frames)
	bodyFrameData := &bodyAction.Frames[bodyFrameIdx]

	// Get head action/frame. The caller chooses which head pose to use; see
	// the note on headFrame above.
	hasHead := headSPR != nil && headACT != nil && len(headACT.Actions) > 0
	var headFrameData *formats.Frame
	if hasHead {
		headActionIdx := action*8 + direction
		if headActionIdx >= len(headACT.Actions) {
			headActionIdx = direction % len(headACT.Actions)
		}
		headAction := &headACT.Actions[headActionIdx]
		if len(headAction.Frames) == 0 {
			hasHead = false
		} else {
			headFrameData = &headAction.Frames[headFrame%len(headAction.Frames)]
		}
	}

	// The weapon runs on the body's own frame: a swing is one motion shared
	// between the two, so it has no pose of its own to choose.
	hasWeapon := weaponSPR != nil && weaponACT != nil && len(weaponACT.Actions) > 0
	var weaponFrameData *formats.Frame
	if hasWeapon {
		weaponActionIdx := action*8 + direction
		if weaponActionIdx >= len(weaponACT.Actions) {
			weaponActionIdx = direction % len(weaponACT.Actions)
		}
		weaponAction := &weaponACT.Actions[weaponActionIdx]
		if len(weaponAction.Frames) == 0 {
			hasWeapon = false
		} else {
			weaponFrameData = &weaponAction.Frames[bodyFrame%len(weaponAction.Frames)]
		}
	}

	// Find body layer bounds
	var bodyMinX, bodyMinY, bodyMaxX, bodyMaxY int
	bodyMinX, bodyMinY = 10000, 10000
	bodyMaxX, bodyMaxY = -10000, -10000

	for _, layer := range bodyFrameData.Layers {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(bodySPR.Images) {
			continue
		}
		img := &bodySPR.Images[layer.SpriteID]
		x, y := int(layer.X), int(layer.Y)
		w, h := int(img.Width), int(img.Height)

		// Layer position is center of sprite
		left := x - w/2
		top := y - h/2
		right := left + w
		bottom := top + h

		if left < bodyMinX {
			bodyMinX = left
		}
		if top < bodyMinY {
			bodyMinY = top
		}
		if right > bodyMaxX {
			bodyMaxX = right
		}
		if bottom > bodyMaxY {
			bodyMaxY = bottom
		}
	}

	// Get body anchor point (where head attaches)
	var bodyAnchorX, bodyAnchorY int
	if len(bodyFrameData.AnchorPoints) > 0 {
		bodyAnchorX = int(bodyFrameData.AnchorPoints[0].X)
		bodyAnchorY = int(bodyFrameData.AnchorPoints[0].Y)
	}

	// Get head anchor point
	var headAnchorX, headAnchorY int
	if hasHead && len(headFrameData.AnchorPoints) > 0 {
		headAnchorX = int(headFrameData.AnchorPoints[0].X)
		headAnchorY = int(headFrameData.AnchorPoints[0].Y)
	}

	// Calculate head offset: head anchor aligns with body anchor
	headOffsetX := bodyAnchorX - headAnchorX
	headOffsetY := bodyAnchorY - headAnchorY

	// Find head layer bounds (relative to head origin + offset)
	var headMinX, headMinY, headMaxX, headMaxY int
	headMinX, headMinY = 10000, 10000
	headMaxX, headMaxY = -10000, -10000

	for _, layer := range headLayers(hasHead, headFrameData) {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(headSPR.Images) {
			continue
		}
		img := &headSPR.Images[layer.SpriteID]
		x, y := int(layer.X)+headOffsetX, int(layer.Y)+headOffsetY
		w, h := int(img.Width), int(img.Height)

		left := x - w/2
		top := y - h/2
		right := left + w
		bottom := top + h

		if left < headMinX {
			headMinX = left
		}
		if top < headMinY {
			headMinY = top
		}
		if right > headMaxX {
			headMaxX = right
		}
		if bottom > headMaxY {
			headMaxY = bottom
		}
	}

	// Weapon alignment.
	//
	// A weapon that carries anchors is aligned onto the body's exactly as the
	// head is. One that does not — which is the usual case, and is true of
	// every dagger and sword checked — is already drawn in the body's own
	// coordinates and wants no offset at all.
	//
	// Offsetting it by the body's anchor regardless is what sent the knife
	// floating fifty-six pixels above the character's head: that anchor is
	// the neck, and it is a correction, not a position.
	var weaponOffsetX, weaponOffsetY int
	if hasWeapon && len(weaponFrameData.AnchorPoints) > 0 {
		weaponOffsetX = bodyAnchorX - int(weaponFrameData.AnchorPoints[0].X)
		weaponOffsetY = bodyAnchorY - int(weaponFrameData.AnchorPoints[0].Y)
	}

	weaponMinX, weaponMinY := 10000, 10000
	weaponMaxX, weaponMaxY := -10000, -10000

	for _, layer := range attachmentLayers(hasWeapon, weaponFrameData) {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(weaponSPR.Images) {
			continue
		}
		img := &weaponSPR.Images[layer.SpriteID]
		x, y := int(layer.X)+weaponOffsetX, int(layer.Y)+weaponOffsetY
		w, h := int(img.Width), int(img.Height)

		left, top := x-w/2, y-h/2
		if left < weaponMinX {
			weaponMinX = left
		}
		if top < weaponMinY {
			weaponMinY = top
		}
		if left+w > weaponMaxX {
			weaponMaxX = left + w
		}
		if top+h > weaponMaxY {
			weaponMaxY = top + h
		}
	}

	// Combine bounds
	minX := bodyMinX
	if headMinX < minX {
		minX = headMinX
	}
	minY := bodyMinY
	if headMinY < minY {
		minY = headMinY
	}
	maxX := bodyMaxX
	if headMaxX > maxX {
		maxX = headMaxX
	}
	maxY := bodyMaxY
	if headMaxY > maxY {
		maxY = headMaxY
	}

	// A weapon reaches well outside the body — a spear more than doubles the
	// width — so the canvas has to grow to hold it or the point is clipped.
	if hasWeapon && weaponMinX < weaponMaxX {
		if weaponMinX < minX {
			minX = weaponMinX
		}
		if weaponMinY < minY {
			minY = weaponMinY
		}
		if weaponMaxX > maxX {
			maxX = weaponMaxX
		}
		if weaponMaxY > maxY {
			maxY = weaponMaxY
		}
	}

	// Handle empty sprites
	if minX >= maxX || minY >= maxY {
		return CompositeResult{}
	}

	// Create canvas
	width := maxX - minX
	height := maxY - minY
	originX := -minX // Offset from canvas origin to sprite origin
	originY := -minY
	pixels := make([]byte, width*height*4)

	// Helper to blit a sprite layer onto canvas
	blitLayer := func(spr *formats.SPR, layer *formats.Layer, offsetX, offsetY int) {
		if layer.SpriteID < 0 || int(layer.SpriteID) >= len(spr.Images) {
			return
		}
		img := &spr.Images[layer.SpriteID]
		imgW, imgH := int(img.Width), int(img.Height)

		// SPR images are already converted to RGBA format
		rgba := img.Pixels
		if len(rgba) == 0 {
			return
		}

		// Layer center position + offset
		cx := int(layer.X) + offsetX + originX
		cy := int(layer.Y) + offsetY + originY

		// Check if layer should be mirrored (horizontal flip)
		mirrored := layer.IsMirrored()

		// Blit with alpha blending
		for py := 0; py < imgH; py++ {
			for px := 0; px < imgW; px++ {
				dx := cx + px - imgW/2
				dy := cy + py - imgH/2
				if dx < 0 || dx >= width || dy < 0 || dy >= height {
					continue
				}

				// Source pixel - flip X if mirrored
				srcX := px
				if mirrored {
					srcX = imgW - 1 - px
				}
				srcIdx := (py*imgW + srcX) * 4
				dstIdx := (dy*width + dx) * 4

				// Source pixel
				sr, sg, sb, sa := rgba[srcIdx], rgba[srcIdx+1], rgba[srcIdx+2], rgba[srcIdx+3]
				if sa == 0 {
					continue // Fully transparent
				}

				// Alpha blend
				if sa == 255 {
					pixels[dstIdx] = sr
					pixels[dstIdx+1] = sg
					pixels[dstIdx+2] = sb
					pixels[dstIdx+3] = sa
				} else {
					// Simple alpha blend
					da := pixels[dstIdx+3]
					outA := sa + da*(255-sa)/255
					if outA > 0 {
						pixels[dstIdx] = byte((int(sr)*int(sa) + int(pixels[dstIdx])*int(da)*(255-int(sa))/255) / int(outA))
						pixels[dstIdx+1] = byte((int(sg)*int(sa) + int(pixels[dstIdx+1])*int(da)*(255-int(sa))/255) / int(outA))
						pixels[dstIdx+2] = byte((int(sb)*int(sa) + int(pixels[dstIdx+2])*int(da)*(255-int(sa))/255) / int(outA))
						pixels[dstIdx+3] = outA
					}
				}
			}
		}
	}

	// Draw body layers first (bottom)
	for _, layer := range bodyFrameData.Layers {
		if layer.SpriteID >= 0 {
			blitLayer(bodySPR, &layer, 0, 0)
		}
	}

	// Draw head layers on top
	for _, layer := range headLayers(hasHead, headFrameData) {
		if layer.SpriteID >= 0 {
			blitLayer(headSPR, &layer, headOffsetX, headOffsetY)
		}
	}

	// The weapon last, in front of both.
	for _, layer := range attachmentLayers(hasWeapon, weaponFrameData) {
		if layer.SpriteID >= 0 {
			blitLayer(weaponSPR, &layer, weaponOffsetX, weaponOffsetY)
		}
	}

	return CompositeResult{
		Pixels:  pixels,
		Width:   width,
		Height:  height,
		OffsetX: minX,
		OffsetY: minY,
	}
}

// attachmentLayers returns a frame's layers, or nothing when the sprite it
// belongs to is absent.
func attachmentLayers(has bool, frame *formats.Frame) []formats.Layer {
	if !has || frame == nil {
		return nil
	}

	return frame.Layers
}

// headLayers returns the head frame's layers, or nothing when the sprite has
// no separate head (monsters, or a character whose head is missing).
func headLayers(hasHead bool, headFrame *formats.Frame) []formats.Layer {
	if !hasHead || headFrame == nil {
		return nil
	}
	return headFrame.Layers
}

// GetActionFrameCount returns the number of frames for an action/direction combo.
func GetActionFrameCount(act *formats.ACT, action, direction int) int {
	actionIdx := action*8 + direction
	if actionIdx >= len(act.Actions) {
		actionIdx = direction % len(act.Actions)
	}
	if actionIdx >= len(act.Actions) {
		return 0
	}
	return len(act.Actions[actionIdx].Frames)
}

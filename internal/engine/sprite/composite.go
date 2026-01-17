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
}

// CompositeSprites creates a single RGBA image by compositing body and head sprites.
// It uses anchor points to correctly position the head relative to the body.
func CompositeSprites(
	bodySPR *formats.SPR, bodyACT *formats.ACT,
	headSPR *formats.SPR, headACT *formats.ACT,
	action, direction, frame int,
) CompositeResult {
	// Get body action/frame
	bodyActionIdx := action*8 + direction
	if bodyActionIdx >= len(bodyACT.Actions) {
		bodyActionIdx = direction % len(bodyACT.Actions)
	}
	bodyAction := &bodyACT.Actions[bodyActionIdx]
	if len(bodyAction.Frames) == 0 {
		return CompositeResult{}
	}
	bodyFrameIdx := frame % len(bodyAction.Frames)
	bodyFrame := &bodyAction.Frames[bodyFrameIdx]

	// Get head action/frame (always use frame 0 for stability)
	headActionIdx := action*8 + direction
	if headActionIdx >= len(headACT.Actions) {
		headActionIdx = direction % len(headACT.Actions)
	}
	headAction := &headACT.Actions[headActionIdx]
	if len(headAction.Frames) == 0 {
		return CompositeResult{}
	}
	// Always use frame 0 for head - it has the matching anchor points
	headFrame := &headAction.Frames[0]

	// Find body layer bounds
	var bodyMinX, bodyMinY, bodyMaxX, bodyMaxY int
	bodyMinX, bodyMinY = 10000, 10000
	bodyMaxX, bodyMaxY = -10000, -10000

	for _, layer := range bodyFrame.Layers {
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
	if len(bodyFrame.AnchorPoints) > 0 {
		bodyAnchorX = int(bodyFrame.AnchorPoints[0].X)
		bodyAnchorY = int(bodyFrame.AnchorPoints[0].Y)
	}

	// Get head anchor point
	var headAnchorX, headAnchorY int
	if len(headFrame.AnchorPoints) > 0 {
		headAnchorX = int(headFrame.AnchorPoints[0].X)
		headAnchorY = int(headFrame.AnchorPoints[0].Y)
	}

	// Calculate head offset: head anchor aligns with body anchor
	headOffsetX := bodyAnchorX - headAnchorX
	headOffsetY := bodyAnchorY - headAnchorY

	// Find head layer bounds (relative to head origin + offset)
	var headMinX, headMinY, headMaxX, headMaxY int
	headMinX, headMinY = 10000, 10000
	headMaxX, headMaxY = -10000, -10000

	for _, layer := range headFrame.Layers {
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
	for _, layer := range bodyFrame.Layers {
		if layer.SpriteID >= 0 {
			blitLayer(bodySPR, &layer, 0, 0)
		}
	}

	// Draw head layers on top
	for _, layer := range headFrame.Layers {
		if layer.SpriteID >= 0 {
			blitLayer(headSPR, &layer, headOffsetX, headOffsetY)
		}
	}

	return CompositeResult{
		Pixels: pixels,
		Width:  width,
		Height: height,
	}
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

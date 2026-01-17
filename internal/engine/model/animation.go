package model

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
	"github.com/Faultbox/midgard-ro/pkg/math"
)

// InterpolateRotKeys interpolates rotation keyframes at the given time.
func InterpolateRotKeys(keys []formats.RSMRotKeyframe, timeMs float32) math.Quat {
	if len(keys) == 0 {
		return math.QuatIdentity()
	}
	if len(keys) == 1 {
		k := keys[0]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// Find surrounding keyframes (assuming keys are sorted by frame)
	var prev, next int
	for i := range keys {
		if float32(keys[i].Frame) > timeMs {
			next = i
			break
		}
		prev = i
		next = i
	}

	// If at or past last frame, return last frame's rotation
	if prev == next {
		k := keys[prev]
		return math.Quat{X: k.Quaternion[0], Y: k.Quaternion[1], Z: k.Quaternion[2], W: k.Quaternion[3]}
	}

	// Interpolate between prev and next
	k0 := keys[prev]
	k1 := keys[next]
	t := float32(0)
	if k1.Frame != k0.Frame {
		t = (timeMs - float32(k0.Frame)) / float32(k1.Frame-k0.Frame)
	}

	q0 := math.Quat{X: k0.Quaternion[0], Y: k0.Quaternion[1], Z: k0.Quaternion[2], W: k0.Quaternion[3]}
	q1 := math.Quat{X: k1.Quaternion[0], Y: k1.Quaternion[1], Z: k1.Quaternion[2], W: k1.Quaternion[3]}
	return q0.Slerp(q1, t)
}

// InterpolateScaleKeys interpolates scale keyframes at the given time.
func InterpolateScaleKeys(keys []formats.RSMScaleKeyframe, timeMs float32) [3]float32 {
	if len(keys) == 0 {
		return [3]float32{1, 1, 1}
	}
	if len(keys) == 1 {
		return keys[0].Scale
	}

	var prev, next int
	for i := range keys {
		if float32(keys[i].Frame) > timeMs {
			next = i
			break
		}
		prev = i
		next = i
	}

	if prev == next {
		return keys[prev].Scale
	}

	k0 := keys[prev]
	k1 := keys[next]
	t := float32(0)
	if k1.Frame != k0.Frame {
		t = (timeMs - float32(k0.Frame)) / float32(k1.Frame-k0.Frame)
	}

	return [3]float32{
		k0.Scale[0] + t*(k1.Scale[0]-k0.Scale[0]),
		k0.Scale[1] + t*(k1.Scale[1]-k0.Scale[1]),
		k0.Scale[2] + t*(k1.Scale[2]-k0.Scale[2]),
	}
}

// HasAnimation checks if an RSM model has any animation keyframes.
// Models with only 1 keyframe are static poses, not animations.
func HasAnimation(rsm *formats.RSM) bool {
	if rsm.AnimLength <= 0 {
		return false
	}
	for i := range rsm.Nodes {
		node := &rsm.Nodes[i]
		if len(node.RotKeys) > 1 || len(node.PosKeys) > 1 || len(node.ScaleKeys) > 1 {
			return true
		}
	}
	return false
}

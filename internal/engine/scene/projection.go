package scene

import "github.com/Faultbox/midgard-ro/pkg/math"

// ProjectToScreen is the projection shared by game effects and visual tools.
func ProjectToScreen(viewProj math.Mat4, x, y, z, width, height float32) (float32, float32) {
	clip := viewProj.MulVec4(math.Vec4{x, y, z, 1})
	if clip[3] <= 0 {
		return -1, -1
	}
	return (clip[0]/clip[3]*0.5 + 0.5) * width, (0.5 - clip[1]/clip[3]*0.5) * height
}

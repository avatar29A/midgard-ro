// Package lighting provides point light support for map rendering.
package lighting

import (
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

// MaxPointLights is the maximum number of point lights supported in shaders.
const MaxPointLights = 32

// PointLight represents a point light source for GPU upload.
type PointLight struct {
	Position  [3]float32 // World position
	Color     [3]float32 // RGB color (0-1 range)
	Range     float32    // Light radius/falloff distance
	Intensity float32    // Light intensity multiplier
}

// PointLightBuffer holds lights for GPU upload.
type PointLightBuffer struct {
	Lights []PointLight
	Count  int
}

// NewPointLightBuffer creates an empty point light buffer.
func NewPointLightBuffer() *PointLightBuffer {
	return &PointLightBuffer{
		Lights: make([]PointLight, 0, MaxPointLights),
	}
}

// ExtractFromRSW extracts point lights from RSW light sources.
// Converts RSW light format to our PointLight format.
func ExtractFromRSW(rsw *formats.RSW) []PointLight {
	if rsw == nil {
		return nil
	}

	rswLights := rsw.GetLights()
	if len(rswLights) == 0 {
		return nil
	}

	lights := make([]PointLight, 0, len(rswLights))
	for _, rswLight := range rswLights {
		// RSW positions need coordinate conversion
		// RSW uses: X right, Y up, Z forward
		// We use the same coordinate system
		light := PointLight{
			Position: [3]float32{
				rswLight.Position[0],
				rswLight.Position[1],
				rswLight.Position[2],
			},
			Color: [3]float32{
				rswLight.Color[0],
				rswLight.Color[1],
				rswLight.Color[2],
			},
			Range:     rswLight.Range,
			Intensity: 1.0, // Default intensity
		}

		// Clamp color values to 0-1 range (some RSW files have values > 1)
		for i := 0; i < 3; i++ {
			if light.Color[i] > 1.0 {
				light.Color[i] = 1.0
			}
			if light.Color[i] < 0.0 {
				light.Color[i] = 0.0
			}
		}

		// Ensure range is positive
		if light.Range <= 0 {
			light.Range = 100.0 // Default range
		}

		lights = append(lights, light)
	}

	return lights
}

// Clear removes all lights from the buffer.
func (b *PointLightBuffer) Clear() {
	b.Lights = b.Lights[:0]
	b.Count = 0
}

// AddLight adds a point light to the buffer.
// Returns false if buffer is full.
func (b *PointLightBuffer) AddLight(light PointLight) bool {
	if b.Count >= MaxPointLights {
		return false
	}
	b.Lights = append(b.Lights, light)
	b.Count++
	return true
}

// SetLights replaces all lights in the buffer.
// Truncates to MaxPointLights if necessary.
func (b *PointLightBuffer) SetLights(lights []PointLight) {
	b.Clear()
	count := len(lights)
	if count > MaxPointLights {
		count = MaxPointLights
	}
	b.Lights = append(b.Lights, lights[:count]...)
	b.Count = count
}

// GetPositions returns positions as a flat float32 slice for GPU upload.
// Format: [x0, y0, z0, x1, y1, z1, ...]
func (b *PointLightBuffer) GetPositions() []float32 {
	result := make([]float32, MaxPointLights*3)
	for i, light := range b.Lights {
		result[i*3+0] = light.Position[0]
		result[i*3+1] = light.Position[1]
		result[i*3+2] = light.Position[2]
	}
	return result
}

// GetColors returns colors as a flat float32 slice for GPU upload.
// Format: [r0, g0, b0, r1, g1, b1, ...]
func (b *PointLightBuffer) GetColors() []float32 {
	result := make([]float32, MaxPointLights*3)
	for i, light := range b.Lights {
		result[i*3+0] = light.Color[0]
		result[i*3+1] = light.Color[1]
		result[i*3+2] = light.Color[2]
	}
	return result
}

// GetRanges returns ranges as a flat float32 slice for GPU upload.
func (b *PointLightBuffer) GetRanges() []float32 {
	result := make([]float32, MaxPointLights)
	for i, light := range b.Lights {
		result[i] = light.Range
	}
	return result
}

// GetIntensities returns intensities as a flat float32 slice for GPU upload.
func (b *PointLightBuffer) GetIntensities() []float32 {
	result := make([]float32, MaxPointLights)
	for i, light := range b.Lights {
		result[i] = light.Intensity
	}
	return result
}

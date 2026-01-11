// Package formats provides parsers for Ragnarok Online file formats.
package formats

// Note: GAT (Ground Altitude Table) is fully implemented in gat.go

// GND represents ground mesh data.
// Contains textures and geometry for the map surface.
type GND struct {
	Width    int
	Height   int
	Textures []string
}

// RSW represents resource/world data.
// Contains objects, lights, sounds, and effects placed on the map.
type RSW struct {
	MapName string
	Objects []RSWObject
}

// RSWObject represents an object placed in the world.
type RSWObject struct {
	Name     string
	Position [3]float32
	Rotation [3]float32
	Scale    [3]float32
}

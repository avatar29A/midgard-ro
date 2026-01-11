// Package formats provides parsers for Ragnarok Online file formats.
package formats

// Note: GAT (Ground Altitude Table) is fully implemented in gat.go
// Note: GND (Ground Mesh) is fully implemented in gnd.go

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

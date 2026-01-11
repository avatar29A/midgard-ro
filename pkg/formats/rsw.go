// Package formats provides parsers for Ragnarok Online file formats.
package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// RSW format errors.
var (
	ErrInvalidRSWMagic       = errors.New("invalid RSW magic: expected 'GRSW'")
	ErrUnsupportedRSWVersion = errors.New("unsupported RSW version")
	ErrTruncatedRSWData      = errors.New("truncated RSW data")
	ErrUnknownObjectType     = errors.New("unknown RSW object type")
)

// RSWVersion represents the RSW file version.
type RSWVersion struct {
	Major       uint8
	Minor       uint8
	BuildNumber uint32 // v2.2+ (uint8 for v2.2-2.4, uint32 for v2.5+)
}

// String returns the version as "Major.Minor".
func (v RSWVersion) String() string {
	if v.BuildNumber > 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.BuildNumber)
	}
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// AtLeast returns true if version is >= major.minor.
func (v RSWVersion) AtLeast(major, minor uint8) bool {
	if v.Major > major {
		return true
	}
	if v.Major == major && v.Minor >= minor {
		return true
	}
	return false
}

// RSWObjectType represents the type of object in the world.
type RSWObjectType int32

const (
	RSWObjectModel  RSWObjectType = 1 // 3D model (RSM file)
	RSWObjectLight  RSWObjectType = 2 // Light source
	RSWObjectSound  RSWObjectType = 3 // Sound source
	RSWObjectEffect RSWObjectType = 4 // Visual effect
)

// String returns a human-readable object type name.
func (t RSWObjectType) String() string {
	switch t {
	case RSWObjectModel:
		return "Model"
	case RSWObjectLight:
		return "Light"
	case RSWObjectSound:
		return "Sound"
	case RSWObjectEffect:
		return "Effect"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

// RSWWater contains water rendering settings.
type RSWWater struct {
	Level      float32 // Water surface height
	Type       int32   // Water texture type
	WaveHeight float32 // Wave amplitude
	WaveSpeed  float32 // Wave animation speed
	WavePitch  float32 // Wave frequency
	AnimSpeed  int32   // Texture animation speed
}

// RSWLight contains global lighting settings.
type RSWLight struct {
	Longitude int32      // Sun horizontal angle
	Latitude  int32      // Sun vertical angle
	Diffuse   [3]float32 // Diffuse light color (RGB)
	Ambient   [3]float32 // Ambient light color (RGB)
	Opacity   float32    // Shadow opacity (v1.7+)
}

// RSWGround contains ground view bounds.
type RSWGround struct {
	Top    int32
	Bottom int32
	Left   int32
	Right  int32
}

// RSWModel represents a 3D model placed in the world.
type RSWModel struct {
	Name      string     // Object instance name
	AnimType  int32      // Animation type
	AnimSpeed float32    // Animation playback speed
	BlockType int32      // Collision type
	ModelName string     // RSM model file name
	NodeName  string     // Node name within model
	Position  [3]float32 // World position (X, Y, Z)
	Rotation  [3]float32 // Rotation angles (X, Y, Z)
	Scale     [3]float32 // Scale factors (X, Y, Z)
}

// RSWLightSource represents a point light in the world.
type RSWLightSource struct {
	Name     string     // Light name
	Position [3]float32 // World position
	Color    [3]float32 // Light color (RGB, 0-1)
	Range    float32    // Light radius
}

// RSWSoundSource represents a sound emitter in the world.
type RSWSoundSource struct {
	Name     string     // Sound name
	File     string     // WAV file name
	Position [3]float32 // World position
	Volume   float32    // Volume (0-1)
	Width    int32      // Trigger area width
	Height   int32      // Trigger area height
	Range    float32    // Audible range
	Cycle    float32    // Loop interval (v2.0+)
}

// RSWEffectSource represents a visual effect in the world.
type RSWEffectSource struct {
	Name     string     // Effect name
	Position [3]float32 // World position
	EffectID int32      // Effect type ID
	Delay    float32    // Delay before playing
	Param    [4]float32 // Effect parameters
}

// RSWObject represents any object in the world.
type RSWObject struct {
	Type   RSWObjectType
	Model  *RSWModel        // Set if Type == RSWObjectModel
	Light  *RSWLightSource  // Set if Type == RSWObjectLight
	Sound  *RSWSoundSource  // Set if Type == RSWObjectSound
	Effect *RSWEffectSource // Set if Type == RSWObjectEffect
}

// RSW represents a parsed Resource World file.
type RSW struct {
	Version  RSWVersion
	IniFile  string // Settings file reference
	GndFile  string // Ground mesh file
	GatFile  string // Altitude file (v1.4+)
	SrcFile  string // Source file (v1.4+)
	Water    RSWWater
	Light    RSWLight
	Ground   RSWGround
	Objects  []RSWObject
	Quadtree [][4]float32 // Scene partitioning (v2.1+)
}

// CountByType returns the count of objects for each type.
func (r *RSW) CountByType() map[RSWObjectType]int {
	counts := make(map[RSWObjectType]int)
	for _, obj := range r.Objects {
		counts[obj.Type]++
	}
	return counts
}

// GetModels returns all model objects.
func (r *RSW) GetModels() []*RSWModel {
	var models []*RSWModel
	for _, obj := range r.Objects {
		if obj.Model != nil {
			models = append(models, obj.Model)
		}
	}
	return models
}

// GetLights returns all light source objects.
func (r *RSW) GetLights() []*RSWLightSource {
	var lights []*RSWLightSource
	for _, obj := range r.Objects {
		if obj.Light != nil {
			lights = append(lights, obj.Light)
		}
	}
	return lights
}

// GetSounds returns all sound source objects.
func (r *RSW) GetSounds() []*RSWSoundSource {
	var sounds []*RSWSoundSource
	for _, obj := range r.Objects {
		if obj.Sound != nil {
			sounds = append(sounds, obj.Sound)
		}
	}
	return sounds
}

// GetEffects returns all effect objects.
func (r *RSW) GetEffects() []*RSWEffectSource {
	var effects []*RSWEffectSource
	for _, obj := range r.Objects {
		if obj.Effect != nil {
			effects = append(effects, obj.Effect)
		}
	}
	return effects
}

// ParseRSW parses a RSW file from raw bytes.
func ParseRSW(data []byte) (*RSW, error) {
	if len(data) < 6 {
		return nil, ErrTruncatedRSWData
	}

	// Check magic "GRSW"
	if string(data[0:4]) != "GRSW" {
		return nil, ErrInvalidRSWMagic
	}

	// Version is stored as [major, minor]
	version := RSWVersion{
		Major: data[4],
		Minor: data[5],
	}

	// Supported versions: 1.2 - 2.6
	if version.Major < 1 || version.Major > 2 || (version.Major == 2 && version.Minor > 6) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedRSWVersion, version)
	}

	rsw := &RSW{
		Version: version,
	}

	offset := 6

	// v2.2+ adds build number
	if version.AtLeast(2, 2) {
		if version.AtLeast(2, 5) {
			// v2.5+ uses uint32 build number + uint8 unknown flag
			rsw.Version.BuildNumber = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			offset++ // skip unknown render flag
		} else {
			// v2.2-2.4 uses uint8 build number
			rsw.Version.BuildNumber = uint32(data[offset])
			offset++
		}
	}

	// Read file references (each 40 bytes, null-terminated)
	rsw.IniFile = readNullString(data[offset : offset+40])
	offset += 40
	rsw.GndFile = readNullString(data[offset : offset+40])
	offset += 40

	// GAT and SRC files added in v1.4+
	if version.AtLeast(1, 4) {
		rsw.GatFile = readNullString(data[offset : offset+40])
		offset += 40
		rsw.SrcFile = readNullString(data[offset : offset+40])
		offset += 40
	}

	r := bytes.NewReader(data[offset:])

	// Water settings (v1.3+ but not v2.6+ where it moved to GND)
	if version.AtLeast(1, 3) && !version.AtLeast(2, 6) {
		if err := binary.Read(r, binary.LittleEndian, &rsw.Water.Level); err != nil {
			return nil, fmt.Errorf("%w: reading water level", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Water.Type); err != nil {
			return nil, fmt.Errorf("%w: reading water type", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Water.WaveHeight); err != nil {
			return nil, fmt.Errorf("%w: reading wave height", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Water.WaveSpeed); err != nil {
			return nil, fmt.Errorf("%w: reading wave speed", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Water.WavePitch); err != nil {
			return nil, fmt.Errorf("%w: reading wave pitch", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Water.AnimSpeed); err != nil {
			return nil, fmt.Errorf("%w: reading water anim speed", ErrTruncatedRSWData)
		}
	}

	// Light settings (v1.5+)
	if version.AtLeast(1, 5) {
		if err := binary.Read(r, binary.LittleEndian, &rsw.Light.Longitude); err != nil {
			return nil, fmt.Errorf("%w: reading light longitude", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Light.Latitude); err != nil {
			return nil, fmt.Errorf("%w: reading light latitude", ErrTruncatedRSWData)
		}
		for i := 0; i < 3; i++ {
			if err := binary.Read(r, binary.LittleEndian, &rsw.Light.Diffuse[i]); err != nil {
				return nil, fmt.Errorf("%w: reading diffuse[%d]", ErrTruncatedRSWData, i)
			}
		}
		for i := 0; i < 3; i++ {
			if err := binary.Read(r, binary.LittleEndian, &rsw.Light.Ambient[i]); err != nil {
				return nil, fmt.Errorf("%w: reading ambient[%d]", ErrTruncatedRSWData, i)
			}
		}
	}

	// Shadow opacity (v1.7+)
	if version.AtLeast(1, 7) {
		if err := binary.Read(r, binary.LittleEndian, &rsw.Light.Opacity); err != nil {
			return nil, fmt.Errorf("%w: reading shadow opacity", ErrTruncatedRSWData)
		}
	}

	// Ground bounds (v1.6+)
	if version.AtLeast(1, 6) {
		if err := binary.Read(r, binary.LittleEndian, &rsw.Ground.Top); err != nil {
			return nil, fmt.Errorf("%w: reading ground top", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Ground.Bottom); err != nil {
			return nil, fmt.Errorf("%w: reading ground bottom", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Ground.Left); err != nil {
			return nil, fmt.Errorf("%w: reading ground left", ErrTruncatedRSWData)
		}
		if err := binary.Read(r, binary.LittleEndian, &rsw.Ground.Right); err != nil {
			return nil, fmt.Errorf("%w: reading ground right", ErrTruncatedRSWData)
		}
	}

	// Read objects
	var objectCount uint32
	if err := binary.Read(r, binary.LittleEndian, &objectCount); err != nil {
		return nil, fmt.Errorf("%w: reading object count", ErrTruncatedRSWData)
	}

	rsw.Objects = make([]RSWObject, 0, objectCount)
	for i := uint32(0); i < objectCount; i++ {
		obj, err := parseRSWObject(r, rsw.Version)
		if err != nil {
			return nil, fmt.Errorf("parsing object %d: %w", i, err)
		}
		rsw.Objects = append(rsw.Objects, obj)
	}

	// Quadtree (v2.1+)
	if version.AtLeast(2, 1) {
		rsw.Quadtree = make([][4]float32, 0)
		for {
			var quad [4]float32
			err := binary.Read(r, binary.LittleEndian, &quad)
			if err != nil {
				break // End of file
			}
			rsw.Quadtree = append(rsw.Quadtree, quad)
		}
	}

	return rsw, nil
}

// parseRSWObject parses a single RSW object.
func parseRSWObject(r *bytes.Reader, version RSWVersion) (RSWObject, error) {
	var obj RSWObject

	if err := binary.Read(r, binary.LittleEndian, &obj.Type); err != nil {
		return RSWObject{}, fmt.Errorf("%w: reading object type", ErrTruncatedRSWData)
	}

	switch obj.Type {
	case RSWObjectModel:
		model, err := parseRSWModel(r, version)
		if err != nil {
			return RSWObject{}, err
		}
		obj.Model = model

	case RSWObjectLight:
		light, err := parseRSWLight(r, version)
		if err != nil {
			return RSWObject{}, err
		}
		obj.Light = light

	case RSWObjectSound:
		sound, err := parseRSWSound(r, version)
		if err != nil {
			return RSWObject{}, err
		}
		obj.Sound = sound

	case RSWObjectEffect:
		effect, err := parseRSWEffect(r, version)
		if err != nil {
			return RSWObject{}, err
		}
		obj.Effect = effect

	default:
		return RSWObject{}, fmt.Errorf("%w: %d", ErrUnknownObjectType, obj.Type)
	}

	return obj, nil
}

// parseRSWModel parses a model object.
func parseRSWModel(r *bytes.Reader, version RSWVersion) (*RSWModel, error) {
	model := &RSWModel{}

	// Name (40 bytes in older versions, varies by version)
	nameBytes := make([]byte, 40)
	if _, err := r.Read(nameBytes); err != nil {
		return nil, fmt.Errorf("%w: reading model name", ErrTruncatedRSWData)
	}
	model.Name = readNullStringBytes(nameBytes)

	if err := binary.Read(r, binary.LittleEndian, &model.AnimType); err != nil {
		return nil, fmt.Errorf("%w: reading anim type", ErrTruncatedRSWData)
	}
	if err := binary.Read(r, binary.LittleEndian, &model.AnimSpeed); err != nil {
		return nil, fmt.Errorf("%w: reading anim speed", ErrTruncatedRSWData)
	}
	if err := binary.Read(r, binary.LittleEndian, &model.BlockType); err != nil {
		return nil, fmt.Errorf("%w: reading block type", ErrTruncatedRSWData)
	}

	// v2.6.162+ adds an unknown byte after block type (collision flags)
	if version.AtLeast(2, 6) && version.BuildNumber >= 162 {
		var unknownByte uint8
		if err := binary.Read(r, binary.LittleEndian, &unknownByte); err != nil {
			return nil, fmt.Errorf("%w: reading model unknown byte", ErrTruncatedRSWData)
		}
	}

	// Model name (80 bytes)
	modelNameBytes := make([]byte, 80)
	if _, err := r.Read(modelNameBytes); err != nil {
		return nil, fmt.Errorf("%w: reading model file name", ErrTruncatedRSWData)
	}
	model.ModelName = readNullStringBytes(modelNameBytes)

	// Node name (80 bytes)
	nodeNameBytes := make([]byte, 80)
	if _, err := r.Read(nodeNameBytes); err != nil {
		return nil, fmt.Errorf("%w: reading node name", ErrTruncatedRSWData)
	}
	model.NodeName = readNullStringBytes(nodeNameBytes)

	// Position, Rotation, Scale
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &model.Position[i]); err != nil {
			return nil, fmt.Errorf("%w: reading position[%d]", ErrTruncatedRSWData, i)
		}
	}
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &model.Rotation[i]); err != nil {
			return nil, fmt.Errorf("%w: reading rotation[%d]", ErrTruncatedRSWData, i)
		}
	}
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &model.Scale[i]); err != nil {
			return nil, fmt.Errorf("%w: reading scale[%d]", ErrTruncatedRSWData, i)
		}
	}

	return model, nil
}

// parseRSWLight parses a light source object.
func parseRSWLight(r *bytes.Reader, _ RSWVersion) (*RSWLightSource, error) {
	light := &RSWLightSource{}

	// Name (80 bytes)
	nameBytes := make([]byte, 80)
	if _, err := r.Read(nameBytes); err != nil {
		return nil, fmt.Errorf("%w: reading light name", ErrTruncatedRSWData)
	}
	light.Name = readNullStringBytes(nameBytes)

	// Position
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &light.Position[i]); err != nil {
			return nil, fmt.Errorf("%w: reading light position[%d]", ErrTruncatedRSWData, i)
		}
	}

	// Color
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &light.Color[i]); err != nil {
			return nil, fmt.Errorf("%w: reading light color[%d]", ErrTruncatedRSWData, i)
		}
	}

	if err := binary.Read(r, binary.LittleEndian, &light.Range); err != nil {
		return nil, fmt.Errorf("%w: reading light range", ErrTruncatedRSWData)
	}

	return light, nil
}

// parseRSWSound parses a sound source object.
func parseRSWSound(r *bytes.Reader, version RSWVersion) (*RSWSoundSource, error) {
	sound := &RSWSoundSource{}

	// Name (80 bytes)
	nameBytes := make([]byte, 80)
	if _, err := r.Read(nameBytes); err != nil {
		return nil, fmt.Errorf("%w: reading sound name", ErrTruncatedRSWData)
	}
	sound.Name = readNullStringBytes(nameBytes)

	// File (80 bytes)
	fileBytes := make([]byte, 80)
	if _, err := r.Read(fileBytes); err != nil {
		return nil, fmt.Errorf("%w: reading sound file", ErrTruncatedRSWData)
	}
	sound.File = readNullStringBytes(fileBytes)

	// Position
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &sound.Position[i]); err != nil {
			return nil, fmt.Errorf("%w: reading sound position[%d]", ErrTruncatedRSWData, i)
		}
	}

	if err := binary.Read(r, binary.LittleEndian, &sound.Volume); err != nil {
		return nil, fmt.Errorf("%w: reading sound volume", ErrTruncatedRSWData)
	}
	if err := binary.Read(r, binary.LittleEndian, &sound.Width); err != nil {
		return nil, fmt.Errorf("%w: reading sound width", ErrTruncatedRSWData)
	}
	if err := binary.Read(r, binary.LittleEndian, &sound.Height); err != nil {
		return nil, fmt.Errorf("%w: reading sound height", ErrTruncatedRSWData)
	}
	if err := binary.Read(r, binary.LittleEndian, &sound.Range); err != nil {
		return nil, fmt.Errorf("%w: reading sound range", ErrTruncatedRSWData)
	}

	// Cycle added in v2.0+
	if version.AtLeast(2, 0) {
		if err := binary.Read(r, binary.LittleEndian, &sound.Cycle); err != nil {
			return nil, fmt.Errorf("%w: reading sound cycle", ErrTruncatedRSWData)
		}
	}

	return sound, nil
}

// parseRSWEffect parses an effect object.
func parseRSWEffect(r *bytes.Reader, _ RSWVersion) (*RSWEffectSource, error) {
	effect := &RSWEffectSource{}

	// Name (80 bytes)
	nameBytes := make([]byte, 80)
	if _, err := r.Read(nameBytes); err != nil {
		return nil, fmt.Errorf("%w: reading effect name", ErrTruncatedRSWData)
	}
	effect.Name = readNullStringBytes(nameBytes)

	// Position
	for i := 0; i < 3; i++ {
		if err := binary.Read(r, binary.LittleEndian, &effect.Position[i]); err != nil {
			return nil, fmt.Errorf("%w: reading effect position[%d]", ErrTruncatedRSWData, i)
		}
	}

	if err := binary.Read(r, binary.LittleEndian, &effect.EffectID); err != nil {
		return nil, fmt.Errorf("%w: reading effect ID", ErrTruncatedRSWData)
	}
	if err := binary.Read(r, binary.LittleEndian, &effect.Delay); err != nil {
		return nil, fmt.Errorf("%w: reading effect delay", ErrTruncatedRSWData)
	}

	// Param (4 floats)
	for i := 0; i < 4; i++ {
		if err := binary.Read(r, binary.LittleEndian, &effect.Param[i]); err != nil {
			return nil, fmt.Errorf("%w: reading effect param[%d]", ErrTruncatedRSWData, i)
		}
	}

	return effect, nil
}

// readNullString extracts a null-terminated string from a byte slice.
func readNullString(data []byte) string {
	if idx := bytes.IndexByte(data, 0); idx >= 0 {
		return string(data[:idx])
	}
	return string(data)
}

// readNullStringBytes is the same as readNullString but takes []byte.
func readNullStringBytes(data []byte) string {
	if idx := bytes.IndexByte(data, 0); idx >= 0 {
		return string(data[:idx])
	}
	return string(data)
}

// ParseRSWFile parses a RSW file from disk.
func ParseRSWFile(path string) (*RSW, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading RSW file: %w", err)
	}
	return ParseRSW(data)
}

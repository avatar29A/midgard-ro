package formats

import (
	"testing"
)

func TestParseRSW_MagicValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name:    "valid magic",
			data:    makeRSWHeader("GRSW", 2, 1),
			wantErr: nil,
		},
		{
			name:    "invalid magic",
			data:    makeRSWHeader("XXXX", 2, 1),
			wantErr: ErrInvalidRSWMagic,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: ErrTruncatedRSWData,
		},
		{
			name:    "truncated data",
			data:    []byte{'G', 'R', 'S'},
			wantErr: ErrTruncatedRSWData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRSW(tt.data)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				}
			}
		})
	}
}

func TestParseRSW_VersionSupport(t *testing.T) {
	tests := []struct {
		name    string
		major   uint8
		minor   uint8
		wantErr bool
	}{
		{"v1.9", 1, 9, false},
		{"v2.1", 2, 1, false},
		{"v2.2", 2, 2, false},
		{"v2.3", 2, 3, false},
		{"v2.4", 2, 4, false},
		{"v2.5", 2, 5, false},
		{"v2.6", 2, 6, false},
		{"v0.1 unsupported", 0, 1, true},
		{"v2.7 unsupported", 2, 7, true},
		{"v3.0 unsupported", 3, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := makeRSWHeader("GRSW", tt.major, tt.minor)
			_, err := ParseRSW(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("version %d.%d: got error=%v, wantErr=%v", tt.major, tt.minor, err, tt.wantErr)
			}
		})
	}
}

func TestRSWVersion_String(t *testing.T) {
	tests := []struct {
		version RSWVersion
		want    string
	}{
		{RSWVersion{2, 1, 0}, "2.1"},
		{RSWVersion{2, 6, 197}, "2.6.197"},
		{RSWVersion{1, 9, 0}, "1.9"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRSWVersion_AtLeast(t *testing.T) {
	tests := []struct {
		version RSWVersion
		major   uint8
		minor   uint8
		want    bool
	}{
		{RSWVersion{2, 1, 0}, 2, 1, true},
		{RSWVersion{2, 1, 0}, 2, 0, true},
		{RSWVersion{2, 1, 0}, 1, 9, true},
		{RSWVersion{2, 1, 0}, 2, 2, false},
		{RSWVersion{2, 1, 0}, 3, 0, false},
		{RSWVersion{2, 6, 0}, 2, 5, true},
		{RSWVersion{1, 9, 0}, 2, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.version.String(), func(t *testing.T) {
			if got := tt.version.AtLeast(tt.major, tt.minor); got != tt.want {
				t.Errorf("AtLeast(%d, %d) = %v, want %v", tt.major, tt.minor, got, tt.want)
			}
		})
	}
}

func TestRSWObjectType_String(t *testing.T) {
	tests := []struct {
		objType RSWObjectType
		want    string
	}{
		{RSWObjectModel, "Model"},
		{RSWObjectLight, "Light"},
		{RSWObjectSound, "Sound"},
		{RSWObjectEffect, "Effect"},
		{RSWObjectType(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.objType.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRSW_V21_Structure(t *testing.T) {
	// Create a minimal v2.1 RSW file
	data := makeMinimalRSW(2, 1, 0)

	rsw, err := ParseRSW(data)
	if err != nil {
		t.Fatalf("ParseRSW failed: %v", err)
	}

	if rsw.Version.Major != 2 || rsw.Version.Minor != 1 {
		t.Errorf("version mismatch: got %s", rsw.Version)
	}

	if rsw.GndFile != "test.gnd" {
		t.Errorf("GndFile = %q, want %q", rsw.GndFile, "test.gnd")
	}

	if rsw.GatFile != "test.gat" {
		t.Errorf("GatFile = %q, want %q", rsw.GatFile, "test.gat")
	}
}

func TestParseRSW_V22_BuildNumber(t *testing.T) {
	// v2.2 has uint8 build number
	data := makeMinimalRSW(2, 2, 42)

	rsw, err := ParseRSW(data)
	if err != nil {
		t.Fatalf("ParseRSW failed: %v", err)
	}

	if rsw.Version.BuildNumber != 42 {
		t.Errorf("BuildNumber = %d, want 42", rsw.Version.BuildNumber)
	}
}

func TestParseRSW_V25_BuildNumber(t *testing.T) {
	// v2.5 has uint32 build number + uint8 flag
	data := makeMinimalRSW(2, 5, 12345)

	rsw, err := ParseRSW(data)
	if err != nil {
		t.Fatalf("ParseRSW failed: %v", err)
	}

	if rsw.Version.BuildNumber != 12345 {
		t.Errorf("BuildNumber = %d, want 12345", rsw.Version.BuildNumber)
	}
}

func TestParseRSW_V26_NoWater(t *testing.T) {
	// v2.6 has no water section (moved to GND)
	data := makeMinimalRSW(2, 6, 197)

	rsw, err := ParseRSW(data)
	if err != nil {
		t.Fatalf("ParseRSW failed: %v", err)
	}

	// Water should be zero/default since v2.6 doesn't include water
	if rsw.Water.Level != 0 {
		t.Errorf("Water.Level = %f, want 0 (v2.6 has no water section)", rsw.Water.Level)
	}
}

func TestRSW_CountByType(t *testing.T) {
	rsw := &RSW{
		Objects: []RSWObject{
			{Type: RSWObjectModel, Model: &RSWModel{}},
			{Type: RSWObjectModel, Model: &RSWModel{}},
			{Type: RSWObjectLight, Light: &RSWLightSource{}},
			{Type: RSWObjectSound, Sound: &RSWSoundSource{}},
			{Type: RSWObjectEffect, Effect: &RSWEffectSource{}},
			{Type: RSWObjectEffect, Effect: &RSWEffectSource{}},
		},
	}

	counts := rsw.CountByType()

	if counts[RSWObjectModel] != 2 {
		t.Errorf("Model count = %d, want 2", counts[RSWObjectModel])
	}
	if counts[RSWObjectLight] != 1 {
		t.Errorf("Light count = %d, want 1", counts[RSWObjectLight])
	}
	if counts[RSWObjectSound] != 1 {
		t.Errorf("Sound count = %d, want 1", counts[RSWObjectSound])
	}
	if counts[RSWObjectEffect] != 2 {
		t.Errorf("Effect count = %d, want 2", counts[RSWObjectEffect])
	}
}

func TestRSW_GetModels(t *testing.T) {
	model1 := &RSWModel{Name: "model1"}
	model2 := &RSWModel{Name: "model2"}

	rsw := &RSW{
		Objects: []RSWObject{
			{Type: RSWObjectModel, Model: model1},
			{Type: RSWObjectLight, Light: &RSWLightSource{}},
			{Type: RSWObjectModel, Model: model2},
		},
	}

	models := rsw.GetModels()
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}

	if models[0].Name != "model1" || models[1].Name != "model2" {
		t.Error("models not returned in correct order")
	}
}

func TestRSW_GetLights(t *testing.T) {
	light := &RSWLightSource{Name: "light1", Range: 100}

	rsw := &RSW{
		Objects: []RSWObject{
			{Type: RSWObjectModel, Model: &RSWModel{}},
			{Type: RSWObjectLight, Light: light},
		},
	}

	lights := rsw.GetLights()
	if len(lights) != 1 {
		t.Fatalf("got %d lights, want 1", len(lights))
	}

	if lights[0].Name != "light1" || lights[0].Range != 100 {
		t.Error("light data mismatch")
	}
}

// Helper functions for creating test data

func makeRSWHeader(magic string, major, minor uint8) []byte {
	// Minimum header that passes magic check
	data := make([]byte, 500)
	copy(data[0:4], magic)
	data[4] = major
	data[5] = minor
	return data
}

//nolint:unparam // major is always 2 in tests as we only test v2.x RSW files
func makeMinimalRSW(major, minor uint8, buildNum uint32) []byte {
	// Create a minimal valid RSW file
	data := make([]byte, 500)
	offset := 0

	// Magic
	copy(data[offset:], "GRSW")
	offset += 4

	// Version
	data[offset] = major
	offset++
	data[offset] = minor
	offset++

	// Build number (v2.2+)
	if major == 2 && minor >= 2 {
		if minor >= 5 {
			// v2.5+ uses uint32 build number + uint8 flag
			data[offset] = byte(buildNum)
			data[offset+1] = byte(buildNum >> 8)
			data[offset+2] = byte(buildNum >> 16)
			data[offset+3] = byte(buildNum >> 24)
			offset += 4
			offset++ // unknown flag
		} else {
			// v2.2-2.4 uses uint8 build number
			data[offset] = byte(buildNum)
			offset++
		}
	}

	// File references (40 bytes each)
	// ini file (empty)
	offset += 40

	// gnd file
	copy(data[offset:], "test.gnd")
	offset += 40

	// gat file (v1.4+)
	copy(data[offset:], "test.gat")
	offset += 40

	// src file (v1.4+)
	offset += 40

	// Water section (v1.3+ but not v2.6+)
	if !(major == 2 && minor >= 6) {
		// water_level: float32
		offset += 4
		// water_type: int32
		offset += 4
		// wave_height: float32
		offset += 4
		// wave_speed: float32
		offset += 4
		// wave_pitch: float32
		offset += 4
		// anim_speed: int32
		offset += 4
	}

	// Light section (v1.5+)
	// longitude: int32
	offset += 4
	// latitude: int32
	offset += 4
	// diffuse: float32[3]
	offset += 12
	// ambient: float32[3]
	offset += 12

	// Opacity (v1.7+)
	offset += 4

	// Ground bounds (v1.6+)
	// offset += 16 (not needed, object count is at the end and already zeroed)
	_ = offset // silence unused variable warning

	// Object count: 0
	// (already zeroed)

	return data
}

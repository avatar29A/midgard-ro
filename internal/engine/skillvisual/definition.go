// Package skillvisual holds definitions shared by the client and visual tools.
package skillvisual

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefinitionPath = "internal/engine/skillvisual/definitions/effects/soul_strike.yaml"

//go:embed definitions/effects/soul_strike.yaml
var builtIn []byte

type SoulDefinition struct {
	SchemaVersion   int     `yaml:"schema_version" json:"schemaVersion"`
	ID              string  `yaml:"id" json:"id"`
	Kind            string  `yaml:"kind" json:"kind"`
	Generator       string  `yaml:"generator" json:"generator"`
	ContractVersion int     `yaml:"contract_version" json:"contractVersion"`
	HalfSize        float32 `yaml:"half_size" json:"halfSize"`
	Rise            float32 `yaml:"rise" json:"rise"`
	Spread          float32 `yaml:"spread" json:"spread"`
}

func ParseSoulDefinition(data []byte) (SoulDefinition, error) {
	var d SoulDefinition
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return d, fmt.Errorf("expected one definition document")
	}
	if d.SchemaVersion != 1 || d.ID != "soul_strike.default" || d.Kind != "procedural" || d.Generator != "procedural.soul_strike" || d.ContractVersion != 1 {
		return d, fmt.Errorf("unsupported Soul Strike definition/generator contract")
	}
	for _, v := range []float32{d.HalfSize, d.Rise, d.Spread} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < 0 || v > 120 {
			return d, fmt.Errorf("soul strike parameters must be finite, 0..120 world units")
		}
	}
	if d.HalfSize == 0 {
		return d, fmt.Errorf("half_size must be positive")
	}
	return d, nil
}

func BuiltInSoulDefinition() SoulDefinition {
	d, err := ParseSoulDefinition(builtIn)
	if err != nil {
		panic(err)
	}
	return d
}

// LoadSoulDefinition uses the same developer override in the game and Studio.
// A shipped client outside a checkout uses its embedded definition. Invalid
// local data is an error, never an unnoticed fallback in Studio.
func LoadSoulDefinition(root string) (d SoulDefinition, digest, source string, err error) {
	source = filepath.Join(root, DefinitionPath)
	data, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		data, err, source = builtIn, nil, "embedded:"+DefinitionPath
	}
	if err != nil {
		return d, "", source, err
	}
	d, err = ParseSoulDefinition(data)
	sum := sha256.Sum256(data)
	return d, hex.EncodeToString(sum[:]), source, err
}

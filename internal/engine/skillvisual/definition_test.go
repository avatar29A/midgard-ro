package skillvisual

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinitionContractAndOverride(t *testing.T) {
	for _, data := range []string{strings.Replace(string(builtIn), "half_size: 18", "half_size: .nan", 1), string(builtIn) + "unknown: true\n", strings.Replace(string(builtIn), "procedural.soul_strike", "missing", 1), string(builtIn) + "---\n{}\n"} {
		if _, err := ParseSoulDefinition([]byte(data)); err == nil {
			t.Fatal("accepted invalid definition", data)
		}
	}
	root := t.TempDir()
	p := filepath.Join(root, DefinitionPath)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(builtIn), "half_size: 18", "half_size: 24", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	d, digest, source, err := LoadSoulDefinition(root)
	if err != nil || d.HalfSize != 24 || len(digest) != 64 || source != p {
		t.Fatal(d, digest, source, err)
	}
}

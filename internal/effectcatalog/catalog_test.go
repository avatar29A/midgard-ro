package effectcatalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Faultbox/midgard-ro/internal/assetworkbench"
)

func TestEditorialSearchBindingsAndPagination(t *testing.T) {
	root := t.TempDir()
	archive, err := filepath.Abs("../../pkg/grf/testdata/test.grf")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "config.yaml"), []byte("data:\n  grf_paths: ["+archive+"]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("../..", MetadataPath))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, MetadataPath)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := assetworkbench.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	found, err := Load(root, c, "душа", "", "curated", 0, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Entries) != 1 || found.Entries[0].PreviewSkill != 13 || found.Entries[0].Resources[0].Available {
		t.Fatal(found)
	}
	row := found.Entries[0]
	if len(row.Skills) != 1 || row.Skills[0].ID != 13 {
		t.Fatal("component sharing must not imply the full effect is reused", row)
	}
	first, err := Load(root, c, "", "", "curated", 0, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(root, c, "", "", "curated", 0, 5, 50)
	if err != nil {
		t.Fatal(err)
	}
	if first.Matched != 12 || first.Next != 5 || len(second.Entries) != 7 || second.Next != -1 {
		t.Fatal(first, second)
	}
	for _, a := range first.Entries {
		for _, b := range second.Entries {
			if a.ID == b.ID {
				t.Fatal("duplicate page")
			}
		}
	}
	lightning, err := Load(root, c, "EF_LIGHTBOLT", "", "bindings", 20, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(lightning.Entries) != 1 || lightning.Entries[0].Resources[0].Path != `data\texture\effect\lightning.str` {
		t.Fatal(lightning)
	}
	missing, err := Load(root, c, "not-a-real-effect", "", "all", 0, 0, 50)
	if err != nil || missing.Matched != 0 {
		t.Fatal(missing, err)
	}
}

// Package assetworkbench exposes the game's resource readers to developer tools.
package assetworkbench

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/Faultbox/midgard-ro/pkg/encoding"
	"github.com/Faultbox/midgard-ro/pkg/grf"
)

type ArchiveInfo struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}
type Entry struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Archive  int    `json:"archive"`
	Source   string `json:"source"`
	Bytes    uint32 `json:"bytes"`
	Variants []int  `json:"variants"`
	raw      string
}
type Catalog struct {
	Archives    []ArchiveInfo `json:"archives"`
	Fingerprint string        `json:"fingerprint"`
	Total       int           `json:"total"`
	readers     []*grf.Archive
	entries     map[string]Entry
	ids         []string
}
type SearchResult struct {
	Archives    []ArchiveInfo `json:"archives"`
	Fingerprint string        `json:"fingerprint"`
	Total       int           `json:"total"`
	Matched     int           `json:"matched"`
	Entries     []Entry       `json:"entries"`
	Next        int           `json:"next"`
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func Open(root string) (*Catalog, error) {
	var cfg struct {
		Data struct {
			GRFPaths []string `yaml:"grf_paths"`
		} `yaml:"data"`
	}
	data, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read config.yaml: %w", err)
	}
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Data.GRFPaths) == 0 || len(cfg.Data.GRFPaths) > 16 {
		return nil, fmt.Errorf("config.yaml needs data.grf_paths (1–16 archives)")
	}
	c := &Catalog{entries: map[string]Entry{}}
	fingerprint := ""
	for i, p := range cfg.Data.GRFPaths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		p = filepath.Clean(p)
		a, err := grf.Open(p)
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		c.readers = append(c.readers, a)
		stat, err := os.Stat(p)
		if err != nil {
			c.Close()
			return nil, err
		}
		list := a.List()
		c.Archives = append(c.Archives, ArchiveInfo{i, p, len(list), stat.Size()})
		fingerprint += fmt.Sprintf("%s:%d:%d\n", p, stat.Size(), stat.ModTime().UnixNano())
		for _, raw := range list {
			id := hash([]byte(raw))
			old := c.entries[id]
			variants := append(old.Variants, i)
			display := raw
			if !utf8.ValidString(raw) {
				display = encoding.EUCKRStringToUTF8(raw)
			}
			display = strings.ToValidUTF8(display, "�")
			meta, _ := a.Entry(raw)
			c.entries[id] = Entry{id, display, strings.TrimPrefix(strings.ToLower(filepath.Ext(raw)), "."), i, p, meta.UncompressedSize, variants, raw}
		}
	}
	c.Total = len(c.entries)
	c.Fingerprint = hash([]byte(fingerprint))
	for id := range c.entries {
		c.ids = append(c.ids, id)
	}
	sort.Slice(c.ids, func(i, j int) bool {
		a, b := c.entries[c.ids[i]], c.entries[c.ids[j]]
		if a.Path == b.Path {
			return a.ID < b.ID
		}
		return a.Path < b.Path
	})
	return c, nil
}
func (c *Catalog) Close() {
	for _, a := range c.readers {
		a.Close()
	}
}
func (c *Catalog) entry(id string, archive int) (Entry, error) {
	e, ok := c.entries[id]
	if !ok {
		return Entry{}, fmt.Errorf("resource ID not found")
	}
	if archive >= 0 {
		if archive >= len(c.readers) {
			return Entry{}, fmt.Errorf("archive index out of range")
		}
		meta, ok := c.readers[archive].Entry(e.raw)
		if !ok {
			return Entry{}, fmt.Errorf("resource not in selected archive")
		}
		e.Archive = archive
		e.Source = c.Archives[archive].Path
		e.Bytes = meta.UncompressedSize
	}
	return e, nil
}
func (c *Catalog) Search(query, kind string, archive, offset, limit int) SearchResult {
	result := SearchResult{c.Archives, c.Fingerprint, c.Total, 0, []Entry{}, -1}
	query = strings.ToLower(query)
	for _, id := range c.ids {
		e, err := c.entry(id, archive)
		if err != nil {
			continue
		}
		if kind != "" && e.Type != kind {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Path), query) {
			continue
		}
		if result.Matched >= offset && len(result.Entries) < limit {
			result.Entries = append(result.Entries, e)
		}
		result.Matched++
	}
	if offset+len(result.Entries) < result.Matched {
		result.Next = offset + len(result.Entries)
	}
	return result
}
func (c *Catalog) read(e Entry) ([]byte, error) { return c.readers[e.Archive].ReadLimit(e.raw, 32<<20) }
func (c *Catalog) pair(e Entry, ext string, archive int) (Entry, error) {
	return c.entry(hash([]byte(strings.TrimSuffix(e.raw, filepath.Ext(e.raw))+ext)), archive)
}

// ReadPath resolves a game path with the same UTF-8/EUC-KR fallback and archive
// precedence as assets.Manager, retaining provenance for an exact frame.
func (c *Catalog) ReadPath(path string) ([]byte, Dependency, error) {
	for _, candidate := range []string{path, string(encoding.UTF8ToEUCKR(path))} {
		for i := len(c.readers) - 1; i >= 0; i-- {
			if _, ok := c.readers[i].Entry(candidate); !ok {
				continue
			}
			data, err := c.readers[i].ReadLimit(candidate, 32<<20)
			if err != nil {
				return nil, Dependency{}, err
			}
			return data, Dependency{Path: path, Source: c.Archives[i].Path, SHA256: hash(data)}, nil
		}
	}
	return nil, Dependency{}, fmt.Errorf("resource not found: %s", path)
}

// ResolvePath returns the exact browser identity using the client's encoding fallback.
func (c *Catalog) ResolvePath(path string) (Entry, error) {
	for _, candidate := range []string{path, string(encoding.UTF8ToEUCKR(path))} {
		if e, err := c.entry(hash([]byte(normalizedIdentity(candidate))), -1); err == nil {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("resource not found: %s", path)
}
func (c *Catalog) ReadID(id string) ([]byte, Dependency, Entry, error) {
	e, err := c.entry(id, -1)
	if err != nil {
		return nil, Dependency{}, e, err
	}
	data, err := c.read(e)
	return data, Dependency{e.Path, e.Source, hash(data)}, e, err
}

func normalizedIdentity(path string) string {
	b := []byte(strings.ReplaceAll(path, "\\", "/"))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

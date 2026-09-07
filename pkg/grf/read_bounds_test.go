package grf

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMalformedTableReturnsError(t *testing.T) {
	original, err := os.ReadFile(testGRFPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"offset", "zlib", "size", "count"} {
		t.Run(name, func(t *testing.T) {
			data := append([]byte(nil), original...)
			offset := int(binary.LittleEndian.Uint32(data[30:34])) + 46
			switch name {
			case "offset":
				binary.LittleEndian.PutUint32(data[30:34], 0xfffffff0)
			case "zlib":
				data[offset+8] = 0
			case "size":
				binary.LittleEndian.PutUint32(data[offset+4:offset+8], 0xffffffff)
			case "count":
				binary.LittleEndian.PutUint32(data[38:42], 0)
			}
			path := filepath.Join(t.TempDir(), "bad.grf")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			a, err := Open(path)
			if err == nil {
				a.Close()
				t.Fatal("accepted corrupt table")
			}
		})
	}
}
func TestConcurrentReadAndLimits(t *testing.T) {
	a, err := Open(testGRFPath())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.ReadLimit("data/test.txt", 2); err == nil {
		t.Fatal("read limit ignored")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				for path, want := range map[string]string{"data/test.txt": "Hello, GRF!", "data/subfolder/nested/file.txt": "Nested file content"} {
					data, err := a.Read(path)
					if err != nil || string(data) != want {
						t.Errorf("%s: %q %v", path, data, err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	entry, _ := a.Entry("data/test.txt")
	entry.Flags = 4
	stored, _ := a.Entry("data/test.txt")
	if stored.Flags == entry.Flags {
		t.Fatal("Entry exposed mutable metadata")
	}
	if _, err := a.Read("data/test.txt"); err != nil {
		t.Fatal("Entry exposed mutable metadata", err)
	}
	a.fileList["data/test.txt"].Flags |= 4
	if _, err := a.Read("data/test.txt"); err == nil {
		t.Fatal("header encryption flag ignored")
	}
}

package assetworkbench

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"golang.org/x/image/bmp"

	"github.com/Faultbox/midgard-ro/internal/engine/sprite"
	"github.com/Faultbox/midgard-ro/pkg/encoding"
	"github.com/Faultbox/midgard-ro/pkg/formats"
)

func writeGRF(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	var payload, table bytes.Buffer
	names := []string{}
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		data := files[n]
		table.Write(encoding.UTF8ToEUCKR(n))
		table.WriteByte(0)
		for j := 0; j < 3; j++ {
			binary.Write(&table, binary.LittleEndian, uint32(len(data)))
		}
		table.WriteByte(1)
		binary.Write(&table, binary.LittleEndian, uint32(payload.Len()))
		payload.Write(data)
	}
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	zw.Write(table.Bytes())
	zw.Close()
	header := make([]byte, 46)
	copy(header, "Master of Magic")
	binary.LittleEndian.PutUint32(header[30:34], uint32(payload.Len()))
	binary.LittleEndian.PutUint32(header[38:42], uint32(len(files)+7))
	binary.LittleEndian.PutUint32(header[42:46], 0x200)
	var out bytes.Buffer
	out.Write(header)
	out.Write(payload.Bytes())
	binary.Write(&out, binary.LittleEndian, uint32(compressed.Len()))
	binary.Write(&out, binary.LittleEndian, uint32(table.Len()))
	out.Write(compressed.Bytes())
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestArchivePriorityAndClientFrameParity(t *testing.T) {
	root := t.TempDir()
	spr, err := os.ReadFile("../../pkg/formats/testdata/test.spr")
	if err != nil {
		t.Fatal(err)
	}
	act, err := os.ReadFile("../../pkg/formats/testdata/test.act")
	if err != nil {
		t.Fatal(err)
	}
	name := "data/sprite/몬스터/test"
	patched := append([]byte(nil), spr...)
	patched[len(patched)-1024+4] = 123
	writeGRF(t, filepath.Join(root, "base.grf"), map[string][]byte{name + ".spr": spr, name + ".act": act})
	writeGRF(t, filepath.Join(root, "patch.grf"), map[string][]byte{name + ".spr": patched})
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("data:\n  grf_paths: [base.grf, patch.grf]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	found := c.Search("몬스터/test", "spr", -1, 0, 50)
	if len(found.Entries) != 1 || found.Entries[0].Archive != 1 || len(found.Entries[0].Variants) != 2 {
		t.Fatalf("priority/encoding: %+v", found)
	}
	if r := c.Search("몬스터/test", "spr", 0, 0, 50); r.Entries[0].Archive != 0 {
		t.Fatal("explicit archive ignored")
	}
	ids := c.Search("test.act", "act", -1, 0, 50)
	p, err := c.Render(ids.Entries[0].ID, -1, 0, 0, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Dependencies[1].SHA256 != hash(patched) || p.Dependencies[1].Source != filepath.Join(root, "patch.grf") {
		t.Fatal("SPR override lost")
	}
	raw, err := base64.StdEncoding.DecodeString(p.PNG)
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	s, err := formats.ParseSPR(patched)
	if err != nil {
		t.Fatal(err)
	}
	a, err := formats.ParseACT(act)
	if err != nil {
		t.Fatal(err)
	}
	for frame := 0; frame < p.FrameCount; frame++ {
		expected := sprite.CompositeSprites(s, a, nil, nil, 0, 0, frame, 0)
		for y := 0; y < expected.Height; y++ {
			for x := 0; x < expected.Width; x++ {
				i := (y*expected.Width + x) * 4
				want := color.NRGBA{expected.Pixels[i], expected.Pixels[i+1], expected.Pixels[i+2], expected.Pixels[i+3]}
				got := color.NRGBAModel.Convert(atlas.At((frame%p.Columns)*p.Width+p.OriginX+expected.OffsetX+x, (frame/p.Columns)*p.Height+p.OriginY+expected.OffsetY+y)).(color.NRGBA)
				if got != want {
					t.Fatalf("frame %d pixel %d,%d: got %v want game %v", frame, x, y, got, want)
				}
			}
		}
	}
	if _, err := c.Render(ids.Entries[0].ID, -1, 999, 0, true, true); err == nil {
		t.Fatal("invalid action accepted")
	}
	if _, err := c.Render(ids.Entries[0].ID, -1, 0, -1, false, true); err == nil {
		t.Fatal("negative frame accepted")
	}
}

// This must run the production executable: importing JPEG/BMP encoders into
// the test binary would also register their decoders and conceal this bug.
func TestToolExecutableDecodesBMPAndJPEG(t *testing.T) {
	root := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{40, 120, 200, 255})
		}
	}
	var bmpData, jpgData bytes.Buffer
	if err := bmp.Encode(&bmpData, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpgData, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	writeGRF(t, filepath.Join(root, "images.grf"), map[string][]byte{"data/test.bmp": bmpData.Bytes(), "data/test.jpg": jpgData.Bytes()})
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("data:\n  grf_paths: [images.grf]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "grfworkbench")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", executable, "../../cmd/grfworkbench")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", output, err)
	}
	for _, ext := range []string{"bmp", "jpg"} {
		t.Run(ext, func(t *testing.T) {
			req, _ := json.Marshal(map[string]any{"op": "render", "id": hash([]byte("data/test." + ext)), "archive": -1, "action": 0, "frame": 0, "colorKey": true})
			cmd := exec.Command(executable)
			cmd.Dir = root
			cmd.Stdin = bytes.NewReader(req)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("render .%s: %s (%v)", ext, output, err)
			}
			var p Preview
			if err := json.Unmarshal(output, &p); err != nil {
				t.Fatal(err)
			}
			if p.Width != 8 || p.Height != 6 {
				t.Fatalf("dimensions: %dx%d", p.Width, p.Height)
			}
			data, err := base64.StdEncoding.DecodeString(p.PNG)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			got := color.NRGBAModel.Convert(decoded.At(3, 3)).(color.NRGBA)
			if got.A != 255 || got.B < 190 || got.R > 50 {
				t.Fatalf("unexpected decoded pixel: %v", got)
			}
		})
	}
}

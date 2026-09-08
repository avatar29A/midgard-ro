package skillstudio

import (
	"encoding/base64"
	"fmt"

	"github.com/Faultbox/midgard-ro/internal/assetworkbench"
	"github.com/Faultbox/midgard-ro/internal/game/states"
)

type AudioClip struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	MIME   string `json:"mimeType"`
	Data   string `json:"data"`
}
type AudioSet struct {
	Clips    []AudioClip `json:"clips"`
	Warnings []string    `json:"warnings"`
}

func AudioClips(root string, ids ...uint16) (AudioSet, error) {
	out := AudioSet{Clips: []AudioClip{}, Warnings: []string{}}
	catalog, err := assetworkbench.Open(root)
	if err != nil {
		return out, err
	}
	defer catalog.Close()
	id := uint16(13)
	if len(ids) > 0 {
		id = ids[0]
	}
	total := 0
	for _, path := range states.PreviewSoundPaths(id) {
		data, dep, err := catalog.ReadPath(path)
		if err != nil {
			out.Warnings = append(out.Warnings, err.Error())
			continue
		}
		if len(data) > 1<<20 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("sound exceeds 1 MiB: %s", path))
			continue
		}
		if total+len(data) > 3<<20 || len(out.Clips) >= 8 {
			out.Warnings = append(out.Warnings, "audio set exceeds preview limit")
			break
		}
		total += len(data)
		out.Clips = append(out.Clips, AudioClip{path, dep.SHA256, "audio/wav", base64.StdEncoding.EncodeToString(data)})
	}
	return out, nil
}

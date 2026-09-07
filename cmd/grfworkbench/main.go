package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Faultbox/midgard-ro/internal/assetworkbench"
)

func main() {
	if err := run(); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"error": err.Error()})
		os.Exit(1)
	}
}
func run() (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("resource parser: %v", p)
		}
	}()
	var req struct {
		Op       string `json:"op"`
		ID       string `json:"id"`
		Query    string `json:"query"`
		Type     string `json:"type"`
		Archive  int    `json:"archive"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
		Action   int    `json:"action"`
		Frame    int    `json:"frame"`
		Animated bool   `json:"animated"`
		ColorKey bool   `json:"colorKey"`
	}
	req.Archive = -1
	req.Limit = 50
	d := json.NewDecoder(io.LimitReader(os.Stdin, 65536))
	d.DisallowUnknownFields()
	if err = d.Decode(&req); err != nil {
		return err
	}
	if req.Archive < -1 || req.Offset < 0 || req.Limit < 1 || req.Limit > 100 || req.Action < 0 || req.Frame < 0 {
		return fmt.Errorf("invalid request bounds")
	}
	c, err := assetworkbench.Open(".")
	if err != nil {
		return err
	}
	defer c.Close()
	var result any
	switch req.Op {
	case "search":
		result = c.Search(req.Query, req.Type, req.Archive, req.Offset, req.Limit)
	case "inspect":
		result, err = c.Inspect(req.ID, req.Archive)
	case "render":
		result, err = c.Render(req.ID, req.Archive, req.Action, req.Frame, req.Animated, req.ColorKey)
	default:
		return fmt.Errorf("unknown operation")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

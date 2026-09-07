// skillstudio is a bounded, persistent JSON-lines renderer worker. Stdout is
// protocol-only. Closing stdin closes all native resources on the GL thread.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/Faultbox/midgard-ro/internal/skillstudio"
)

func init() { runtime.LockOSThread() }
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return err
	}
	defer sdl.Quit()
	for _, a := range [][2]int{{int(sdl.GL_CONTEXT_MAJOR_VERSION), 4}, {int(sdl.GL_CONTEXT_MINOR_VERSION), 1}, {int(sdl.GL_CONTEXT_PROFILE_MASK), sdl.GL_CONTEXT_PROFILE_CORE}, {int(sdl.GL_DEPTH_SIZE), 24}, {int(sdl.GL_DOUBLEBUFFER), 0}} {
		if err := sdl.GLSetAttribute(sdl.GLattr(a[0]), a[1]); err != nil {
			return err
		}
	}
	w, err := sdl.CreateWindow("Skill Studio worker", 0, 0, 64, 64, sdl.WINDOW_OPENGL|sdl.WINDOW_HIDDEN)
	if err != nil {
		return err
	}
	defer func() { _ = w.Destroy() }()
	ctx, err := w.GLCreateContext()
	if err != nil {
		return err
	}
	defer sdl.GLDeleteContext(ctx)
	if err = gl.Init(); err != nil {
		return err
	}
	renderer, err := skillstudio.New(".")
	if err != nil {
		return err
	}
	defer renderer.Close()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 65536)
	out := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		sdl.PumpEvents()
		var req skillstudio.Request
		dec := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		dec.DisallowUnknownFields()
		if err = dec.Decode(&req); err != nil {
			if err = out.Encode(map[string]string{"error": err.Error()}); err != nil {
				return err
			}
			continue
		}
		frame, renderErr := renderer.Render(req)
		if renderErr != nil {
			err = out.Encode(map[string]string{"error": renderErr.Error()})
		} else {
			err = out.Encode(frame)
		}
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

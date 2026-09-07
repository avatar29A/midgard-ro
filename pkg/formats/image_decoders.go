package formats

// DecodeImage owns its decoder registry. Standalone tools must not depend on
// unrelated game packages registering these formats as a side effect.
import (
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
)

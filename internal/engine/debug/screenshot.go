// Package debug provides debug visualization utilities.
package debug

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// ScreenshotCapture handles screenshot capture functionality.
type ScreenshotCapture struct {
	outputDir string
	prefix    string
}

// NewScreenshotCapture creates a new screenshot capture handler.
func NewScreenshotCapture(outputDir, prefix string) *ScreenshotCapture {
	return &ScreenshotCapture{
		outputDir: outputDir,
		prefix:    prefix,
	}
}

// SetOutputDir sets the output directory for screenshots.
func (sc *ScreenshotCapture) SetOutputDir(dir string) {
	sc.outputDir = dir
}

// CaptureFromPixels captures a screenshot from raw pixel data.
// pixels should be in RGBA format with width*height*4 bytes.
// The image is flipped vertically since OpenGL has origin at bottom-left.
func (sc *ScreenshotCapture) CaptureFromPixels(pixels []byte, width, height int) (string, error) {
	if len(pixels) != width*height*4 {
		return "", fmt.Errorf("pixel data size mismatch: expected %d, got %d", width*height*4, len(pixels))
	}

	// Create output directory if needed
	if sc.outputDir != "" {
		if err := os.MkdirAll(sc.outputDir, 0755); err != nil {
			return "", fmt.Errorf("creating output dir: %w", err)
		}
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.png", sc.prefix, timestamp)
	if sc.outputDir != "" {
		filename = filepath.Join(sc.outputDir, filename)
	}

	// Create image (flip vertically during copy)
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	rowSize := width * 4
	for y := 0; y < height; y++ {
		srcY := height - 1 - y // Flip Y
		srcOffset := srcY * rowSize
		dstOffset := y * img.Stride

		copy(img.Pix[dstOffset:dstOffset+rowSize], pixels[srcOffset:srcOffset+rowSize])
	}

	// Save to file
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return "", fmt.Errorf("encoding PNG: %w", err)
	}

	return filename, nil
}

// CaptureFromImage captures a screenshot from an existing image.
func (sc *ScreenshotCapture) CaptureFromImage(img image.Image) (string, error) {
	// Create output directory if needed
	if sc.outputDir != "" {
		if err := os.MkdirAll(sc.outputDir, 0755); err != nil {
			return "", fmt.Errorf("creating output dir: %w", err)
		}
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.png", sc.prefix, timestamp)
	if sc.outputDir != "" {
		filename = filepath.Join(sc.outputDir, filename)
	}

	// Save to file
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return "", fmt.Errorf("encoding PNG: %w", err)
	}

	return filename, nil
}

// GenerateFilename generates a screenshot filename without saving.
func (sc *ScreenshotCapture) GenerateFilename() string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.png", sc.prefix, timestamp)
	if sc.outputDir != "" {
		filename = filepath.Join(sc.outputDir, filename)
	}
	return filename
}

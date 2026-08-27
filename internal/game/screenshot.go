package game

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
)

// Screenshots are encoded off the render thread.
//
// A retina frame is 2560×1440 — fifteen megabytes of pixels — and encoding
// it as PNG took most of a second on the thread that draws the game. An
// unattended run capturing every second spent more time on its own captures
// than on the map it was capturing: a 1.3 s load measured as 7.3 s. Reading
// the pixels back is quick; everything after that happens here, in order, so
// latest.png is always the newest.

// screenshotJob is one frame waiting to be written.
type screenshotJob struct {
	pixels        []byte // RGBA, top row first
	width, height int
	dir, name     string
}

// screenshotWriter serializes the encoding and writing of captured frames.
type screenshotWriter struct {
	jobs chan screenshotJob
	wg   sync.WaitGroup
	once sync.Once
}

// screenshotQueue is how many frames may wait to be written before captures
// are dropped rather than let the queue grow without bound.
const screenshotQueue = 4

// start brings the worker up on first use.
func (w *screenshotWriter) start() {
	w.once.Do(func() {
		w.jobs = make(chan screenshotJob, screenshotQueue)
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			for job := range w.jobs {
				writeScreenshot(job)
			}
		}()
	})
}

// enqueue hands a frame to the worker. It never blocks the caller: a frame
// that finds the queue full is dropped with a warning.
func (w *screenshotWriter) enqueue(job screenshotJob) bool {
	w.start()
	select {
	case w.jobs <- job:
		return true
	default:
		logger.Warn("screenshot dropped, previous captures still being written",
			zap.String("name", job.name))
		return false
	}
}

// flush waits for queued frames to be written, up to a limit, so a capture
// requested just before exit still reaches the disk.
func (w *screenshotWriter) flush(limit time.Duration) {
	if w.jobs == nil {
		return
	}
	close(w.jobs)
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(limit):
		logger.Warn("gave up waiting for screenshots to be written")
	}
}

// writeScreenshot encodes one frame as PNG and writes it, plus latest.png.
//
// Best-speed compression: the files are a QA aid read once and deleted, and
// the default level costs three times as long for a fifth less disk.
func writeScreenshot(job screenshotJob) {
	if err := os.MkdirAll(job.dir, 0o755); err != nil {
		logger.Warn("failed to create screenshot dir", zap.Error(err))
		return
	}

	img := &image.RGBA{Pix: job.pixels, Stride: job.width * 4, Rect: image.Rect(0, 0, job.width, job.height)}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}

	savePath := filepath.Join(job.dir, job.name)
	if err := encodePNG(&enc, savePath, img); err != nil {
		logger.Warn("failed to write screenshot", zap.String("path", savePath), zap.Error(err))
		return
	}

	// Also save as "latest.png" for easy access
	latestPath := filepath.Join(job.dir, "latest.png")
	if err := encodePNG(&enc, latestPath, img); err != nil {
		logger.Warn("failed to write latest.png", zap.Error(err))
	}

	logger.Info("screenshot saved", zap.String("path", savePath))
}

func encodePNG(enc *png.Encoder, path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := enc.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// screenshotName is the file a capture taken now is saved as.
func screenshotName(now time.Time) string {
	return fmt.Sprintf("screenshot-%s.png", now.Format("20060102-150405"))
}

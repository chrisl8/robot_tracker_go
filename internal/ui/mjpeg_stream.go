//go:build gocv

package ui

import (
	"fmt"
	"net/http"
	"sync"
)

// mjpegStream is a custom MJPEG streamer that flushes each frame immediately
// to avoid buffering lag. It replaces the hybridgroup/mjpeg library which never
// called Flush(), causing ~8 seconds of stream delay.
type mjpegStream struct {
	mu      sync.Mutex
	frame   []byte
	clients map[chan []byte]struct{}
}

func newMJPEGStream() *mjpegStream {
	return &mjpegStream{
		clients: make(map[chan []byte]struct{}),
	}
}

// updateJPEG stores the latest JPEG frame and sends it to all connected clients.
// Uses a non-blocking drain-and-replace pattern so slow clients never block the
// capture pipeline — they simply skip to the newest frame.
func (s *mjpegStream) updateJPEG(jpeg []byte) {
	s.mu.Lock()
	s.frame = jpeg
	for ch := range s.clients {
		// Drain any stale frame sitting in the channel
		select {
		case <-ch:
		default:
		}
		// Non-blocking send of the new frame
		select {
		case ch <- jpeg:
		default:
		}
	}
	s.mu.Unlock()
}

const mjpegBoundary = "mjpegboundary"

// serveHTTP writes an MJPEG multipart stream, flushing after every frame.
func (s *mjpegStream) serveHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := make(chan []byte, 1)

	// Seed the channel with the current frame so the client gets an image immediately
	s.mu.Lock()
	if len(s.frame) > 0 {
		ch <- s.frame
	}
	s.clients[ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace;boundary="+mjpegBoundary)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case <-r.Context().Done():
			return
		case frame := <-ch:
			header := fmt.Sprintf("\r\n--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", mjpegBoundary, len(frame))
			if _, err := w.Write([]byte(header)); err != nil {
				return
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

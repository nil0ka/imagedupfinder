package server

import (
	"bytes"
	"container/list"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	thumbDefaultSize = 512
	thumbMinSize     = 64
	thumbMaxSize     = 2048
	thumbCacheBudget = 100 << 20 // 100 MiB of encoded thumbnails
	thumbJPEGQuality = 80
)

// thumbEntry is one cached, encoded thumbnail. fileSize and modTime identify
// the source file state the thumbnail was rendered from.
type thumbEntry struct {
	key         string
	data        []byte
	contentType string
	fileSize    int64
	modTime     time.Time
}

// thumbCache is a byte-budgeted LRU cache of encoded thumbnails.
type thumbCache struct {
	mu    sync.Mutex
	max   int64
	size  int64
	ll    *list.List // front = most recently used
	items map[string]*list.Element
}

func newThumbCache(max int64) *thumbCache {
	return &thumbCache{
		max:   max,
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

// get returns the cached entry for key if it was rendered from a source file
// with the same size and modification time, or nil otherwise.
func (c *thumbCache) get(key string, fileSize int64, modTime time.Time) *thumbEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return nil
	}
	e := el.Value.(*thumbEntry)
	if e.fileSize != fileSize || !e.modTime.Equal(modTime) {
		c.removeLocked(el)
		return nil
	}
	c.ll.MoveToFront(el)
	return e
}

func (c *thumbCache) put(e *thumbEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[e.key]; ok {
		c.removeLocked(el)
	}
	c.items[e.key] = c.ll.PushFront(e)
	c.size += int64(len(e.data))
	for c.size > c.max && c.ll.Len() > 1 {
		c.removeLocked(c.ll.Back())
	}
}

func (c *thumbCache) removeLocked(el *list.Element) {
	e := el.Value.(*thumbEntry)
	c.ll.Remove(el)
	delete(c.items, e.key)
	c.size -= int64(len(e.data))
}

func (s *Server) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	s.recordActivity()

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	// Same access rule as /api/image: only serve files this tool has scanned.
	known, err := s.storage.ImageExists(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !known {
		http.Error(w, "path is not a scanned image", http.StatusNotFound)
		return
	}

	size := thumbDefaultSize
	if v := r.URL.Query().Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid size", http.StatusBadRequest)
			return
		}
		size = min(max(n, thumbMinSize), thumbMaxSize)
	}

	stat, err := os.Stat(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	etag := fmt.Sprintf(`"%x-%x-%x"`, stat.Size(), stat.ModTime().UnixNano(), size)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	key := fmt.Sprintf("%s\x00%d", path, size)
	entry := s.thumbs.get(key, stat.Size(), stat.ModTime())
	if entry == nil {
		data, contentType, err := renderThumbnail(path, size)
		if err != nil {
			http.Error(w, "failed to render thumbnail", http.StatusInternalServerError)
			return
		}
		entry = &thumbEntry{
			key:         key,
			data:        data,
			contentType: contentType,
			fileSize:    stat.Size(),
			modTime:     stat.ModTime(),
		}
		s.thumbs.put(entry)
	}

	w.Header().Set("Content-Type", entry.contentType)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Write(entry.data)
}

// renderThumbnail decodes the image at path and re-encodes it scaled down to
// fit within maxDim×maxDim. Formats that may carry transparency are encoded
// as PNG, the rest as JPEG. Re-encoding server-side also makes formats
// browsers cannot display natively (e.g. TIFF) viewable in the UI.
func renderThumbnail(path string, maxDim int) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	src, format, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > maxDim || h > maxDim {
		scale := float64(maxDim) / float64(max(w, h))
		w = max(int(float64(w)*scale+0.5), 1)
		h = max(int(float64(h)*scale+0.5), 1)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	switch format {
	case "png", "gif", "webp":
		if err := png.Encode(&buf, dst); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	default:
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbJPEGQuality}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}
}

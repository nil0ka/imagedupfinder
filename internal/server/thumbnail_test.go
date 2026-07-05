package server

import (
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"imagedupfinder/internal/models"
)

// writeTestImage encodes a solid image of the given size to path.
func writeTestImage(t *testing.T, path string, width, height int, encode func(*os.File, image.Image) error) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func registerImage(t *testing.T, s *Server, path string) {
	t.Helper()
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.storage.SaveImages([]*models.ImageInfo{
		{Path: path, Hash: 1, Width: 10, Height: 10, Format: "png", FileSize: stat.Size(), ModTime: stat.ModTime(), Score: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandleThumbnail_RejectsUnknownPath(t *testing.T) {
	s := newTestServer(t)

	secret := filepath.Join(t.TempDir(), "secret.png")
	writeTestImage(t, secret, 10, 10, func(f *os.File, img image.Image) error { return png.Encode(f, img) })

	req := httptest.NewRequest("GET", "/api/thumbnail?path="+url.QueryEscape(secret), nil)
	rec := httptest.NewRecorder()
	s.handleThumbnail(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unscanned path, got %d", rec.Code)
	}
}

func TestHandleThumbnail_ResizesAndPreservesFormatFamily(t *testing.T) {
	s := newTestServer(t)
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		file            string
		encode          func(*os.File, image.Image) error
		wantContentType string
	}{
		{"png stays png", "big.png", func(f *os.File, img image.Image) error { return png.Encode(f, img) }, "image/png"},
		{"jpeg stays jpeg", "big.jpg", func(f *os.File, img image.Image) error { return jpeg.Encode(f, img, nil) }, "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.file)
			writeTestImage(t, path, 200, 100, tt.encode)
			registerImage(t, s, path)

			req := httptest.NewRequest("GET", "/api/thumbnail?path="+url.QueryEscape(path)+"&size=64", nil)
			rec := httptest.NewRecorder()
			s.handleThumbnail(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != tt.wantContentType {
				t.Errorf("Content-Type = %q, want %q", got, tt.wantContentType)
			}

			thumb, _, err := image.Decode(rec.Body)
			if err != nil {
				t.Fatalf("thumbnail is not a decodable image: %v", err)
			}
			b := thumb.Bounds()
			if b.Dx() != 64 || b.Dy() != 32 {
				t.Errorf("thumbnail size = %dx%d, want 64x32", b.Dx(), b.Dy())
			}
		})
	}
}

func TestHandleThumbnail_SmallImageNotUpscaled(t *testing.T) {
	s := newTestServer(t)

	path := filepath.Join(t.TempDir(), "small.png")
	writeTestImage(t, path, 10, 10, func(f *os.File, img image.Image) error { return png.Encode(f, img) })
	registerImage(t, s, path)

	req := httptest.NewRequest("GET", "/api/thumbnail?path="+url.QueryEscape(path), nil)
	rec := httptest.NewRecorder()
	s.handleThumbnail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	thumb, _, err := image.Decode(rec.Body)
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if b := thumb.Bounds(); b.Dx() != 10 || b.Dy() != 10 {
		t.Errorf("thumbnail size = %dx%d, want 10x10 (no upscaling)", b.Dx(), b.Dy())
	}
}

func TestHandleThumbnail_ETagRevalidation(t *testing.T) {
	s := newTestServer(t)

	path := filepath.Join(t.TempDir(), "img.png")
	writeTestImage(t, path, 20, 20, func(f *os.File, img image.Image) error { return png.Encode(f, img) })
	registerImage(t, s, path)

	req := httptest.NewRequest("GET", "/api/thumbnail?path="+url.QueryEscape(path), nil)
	rec := httptest.NewRecorder()
	s.handleThumbnail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	req = httptest.NewRequest("GET", "/api/thumbnail?path="+url.QueryEscape(path), nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	s.handleThumbnail(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("expected 304 for matching ETag, got %d", rec.Code)
	}
}

func TestThumbCache_InvalidatesOnFileChange(t *testing.T) {
	c := newThumbCache(1 << 20)
	now := time.Now()

	c.put(&thumbEntry{key: "k", data: []byte("data"), fileSize: 100, modTime: now})

	if e := c.get("k", 100, now); e == nil {
		t.Fatal("expected cache hit for unchanged file")
	}
	if e := c.get("k", 200, now); e != nil {
		t.Error("expected miss after size change")
	}
	if e := c.get("k", 100, now); e != nil {
		t.Error("stale entry must be evicted after invalidation")
	}
}

func TestThumbCache_EvictsOverBudget(t *testing.T) {
	c := newThumbCache(10)
	now := time.Now()

	c.put(&thumbEntry{key: "a", data: make([]byte, 6), fileSize: 1, modTime: now})
	c.put(&thumbEntry{key: "b", data: make([]byte, 6), fileSize: 1, modTime: now})

	if e := c.get("a", 1, now); e != nil {
		t.Error("oldest entry should have been evicted over budget")
	}
	if e := c.get("b", 1, now); e == nil {
		t.Error("newest entry should survive eviction")
	}
	if c.size != 6 {
		t.Errorf("cache size = %d, want 6", c.size)
	}
}

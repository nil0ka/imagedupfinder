package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"imagedupfinder/internal/models"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath, 0, 0)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	t.Cleanup(func() { s.storage.Close() })
	return s
}

func TestHandleImage_RejectsUnknownPath(t *testing.T) {
	s := newTestServer(t)

	// A real file that exists on disk but was never scanned must not be served
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/image?path="+url.QueryEscape(secret), nil)
	rec := httptest.NewRecorder()
	s.handleImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unscanned path, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Error("response must not contain file contents")
	}
}

func TestHandleImage_ServesScannedPath(t *testing.T) {
	s := newTestServer(t)

	imgPath := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(imgPath, []byte("fake image data"), 0644); err != nil {
		t.Fatal(err)
	}
	err := s.storage.SaveImages([]*models.ImageInfo{
		{Path: imgPath, Hash: 1, Width: 10, Height: 10, Format: "jpeg", FileSize: 15, ModTime: time.Now(), Score: 100},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/image?path="+url.QueryEscape(imgPath), nil)
	rec := httptest.NewRecorder()
	s.handleImage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for scanned path, got %d", rec.Code)
	}
}

func TestHandleClean_RejectsUnknownPath(t *testing.T) {
	s := newTestServer(t)

	// A real file that was never scanned must not be deletable
	victim := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(victim, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	body := `{"paths":["` + victim + `"],"permanent":true}`
	req := httptest.NewRequest("POST", "/api/clean", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleClean(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not a scanned image") {
		t.Errorf("expected per-path error, got: %s", rec.Body.String())
	}
	if _, err := os.Stat(victim); err != nil {
		t.Error("unscanned file must not be deleted")
	}
}

func TestRequireLocalOrigin(t *testing.T) {
	s := newTestServer(t)
	handler := s.requireLocalOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"localhost host", "localhost:8080", "", http.StatusOK},
		{"loopback host", "127.0.0.1:8080", "", http.StatusOK},
		{"ipv6 loopback host", "[::1]:8080", "", http.StatusOK},
		{"local origin", "localhost:8080", "http://localhost:8080", http.StatusOK},
		{"dns rebinding host", "evil.example.com", "", http.StatusForbidden},
		{"lan ip host", "192.168.1.5:8080", "", http.StatusForbidden},
		{"cross-site origin", "localhost:8080", "https://evil.example.com", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/groups", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("host=%q origin=%q: got %d, want %d", tt.host, tt.origin, rec.Code, tt.want)
			}
		})
	}
}

package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"imagedupfinder/internal/fileutil"
	"imagedupfinder/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// Server represents the web server
type Server struct {
	storage     *storage.Storage
	port        int
	idleTimeout time.Duration
	httpServer  *http.Server
	thumbs      *thumbCache

	// Idle timeout management
	mu            sync.Mutex
	lastActivity  time.Time
	tabActive     bool
	activeClients int
	shutdownChan  chan struct{}
}

// New creates a new Server
func New(dbPath string, port int, idleTimeout time.Duration) (*Server, error) {
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		return nil, err
	}

	s := &Server{
		storage:      store,
		port:         port,
		idleTimeout:  idleTimeout,
		thumbs:       newThumbCache(thumbCacheBudget),
		lastActivity: time.Now(),
		tabActive:    false,
		shutdownChan: make(chan struct{}),
	}

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/groups", s.handleGroups)
	mux.HandleFunc("/api/clean", s.handleClean)
	mux.HandleFunc("/api/image", s.handleImage)
	mux.HandleFunc("/api/thumbnail", s.handleThumbnail)

	// WebSocket for connection monitoring
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s.httpServer = &http.Server{
		// Bind to loopback only: this server can read and delete local files,
		// so it must never be reachable from other machines.
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler: s.requireLocalOrigin(mux),
	}

	// Start idle timeout checker
	if s.idleTimeout > 0 {
		go s.idleTimeoutChecker()
	}

	// Handle shutdown signals
	go s.handleShutdownSignals()

	err = s.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) handleShutdownSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\nShutting down server...")
	case <-s.shutdownChan:
		fmt.Println("\nIdle timeout reached. Shutting down server...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.httpServer.Shutdown(ctx)
	s.storage.Close()
}

func (s *Server) idleTimeoutChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			// Don't timeout if tab is active or there are active WebSocket clients
			if s.tabActive || s.activeClients > 0 {
				s.lastActivity = time.Now()
				s.mu.Unlock()
				continue
			}

			idle := time.Since(s.lastActivity)
			s.mu.Unlock()

			if idle >= s.idleTimeout {
				close(s.shutdownChan)
				return
			}
		case <-s.shutdownChan:
			return
		}
	}
}

func (s *Server) recordActivity() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

func (s *Server) setTabActive(active bool) {
	s.mu.Lock()
	s.tabActive = active
	if active {
		s.lastActivity = time.Now()
	}
	s.mu.Unlock()
}

// isLoopbackHost reports whether a host (optionally host:port) refers to the
// local machine.
func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// requireLocalOrigin rejects requests whose Host header is not local
// (DNS rebinding) or whose Origin header is from another site (CSRF).
// This also guards the WebSocket handshake.
func (s *Server) requireLocalOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !isLoopbackHost(u.Host) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// API Handlers

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	s.recordActivity()

	groups, err := s.storage.GetDuplicateGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.recordActivity()

	var req struct {
		Paths     []string `json:"paths"`
		Permanent bool     `json:"permanent,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var results []map[string]interface{}

	for _, path := range req.Paths {
		result := map[string]interface{}{"path": path}
		results = append(results, result)

		// Only operate on files this tool has scanned; otherwise the API
		// could be used to delete arbitrary files on the machine.
		known, err := s.storage.ImageExists(path)
		if err != nil {
			result["error"] = err.Error()
			continue
		}
		if !known {
			result["error"] = "path is not a scanned image"
			continue
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File doesn't exist, just remove from DB
			s.storage.DeleteImage(path)
			result["status"] = "not_found"
		} else if req.Permanent {
			// Delete file permanently
			err := os.Remove(path)
			if err != nil {
				result["error"] = err.Error()
			} else {
				result["status"] = "deleted"
				s.storage.DeleteImage(path)
			}
		} else {
			// Move to trash (default)
			err := fileutil.MoveToTrash(path)
			if err != nil {
				result["error"] = err.Error()
			} else {
				result["status"] = "trashed"
				s.storage.DeleteImage(path)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	s.recordActivity()

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	// Only serve files this tool has scanned; otherwise this endpoint would
	// allow reading arbitrary files on the machine.
	known, err := s.storage.ImageExists(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !known {
		http.Error(w, "path is not a scanned image", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, path)
}

package artifact

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type Server struct {
	Address, Root string
	ready         atomic.Bool
}

func (s *Server) NeedLeaderElection() bool { return true }
func (s *Server) Ready(_ *http.Request) error {
	if !s.ready.Load() {
		return errors.New("artifact server is not ready")
	}
	return nil
}

func (s *Server) Start(ctx context.Context) error {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.EscapedPath(), "/")
		decoded, err := filepath.Localize(name)
		if err != nil || decoded == "." || decoded == "" || filepath.IsAbs(decoded) {
			http.NotFound(w, r)
			return
		}
		target := filepath.Join(s.Root, decoded)
		rel, err := filepath.Rel(s.Root, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		info, err := os.Lstat(target)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, target)
	})
	server := &http.Server{Addr: s.Address, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 5 * time.Minute, IdleTimeout: 90 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	s.ready.Store(true)
	defer s.ready.Store(false)
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

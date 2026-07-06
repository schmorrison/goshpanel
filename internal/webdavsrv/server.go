package webdavsrv

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/files"
	"golang.org/x/net/webdav"
)

// Server serves the files sandbox over WebDAV.
type Server struct {
	addr string
	root *files.Service
	log  *slog.Logger
}

// New creates a WebDAV server.
func New(addr string, root *files.Service, log *slog.Logger) *Server {
	return &Server{addr: addr, root: root, log: log}
}

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	handler := &webdav.Handler{
		FileSystem: webdav.Dir(s.root.Root()),
		LockSystem: webdav.NewMemLS(),
	}
	srv := &http.Server{Addr: s.addr, Handler: handler}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	if s.log != nil {
		s.log.Info("webdav listening", "addr", ln.Addr().String())
	}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	return srv.Serve(ln)
}

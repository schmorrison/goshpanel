package sftpserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"github.com/schmorrison/goshpanel/internal/crypto"
	"github.com/schmorrison/goshpanel/internal/files"
	"github.com/schmorrison/goshpanel/internal/store"
	"golang.org/x/crypto/ssh"
)

// Server is an SSH/SFTP listener rooted at the files sandbox.
type Server struct {
	addr     string
	hostKey  ssh.Signer
	files    *files.Service
	store    *store.Store
	log      *slog.Logger
	listener net.Listener
}

// New creates an SFTP server. hostKeyPath is loaded or generated on first run.
func New(addr, hostKeyPath string, fileSvc *files.Service, st *store.Store, log *slog.Logger) (*Server, error) {
	signer, err := loadOrCreateHostKey(hostKeyPath)
	if err != nil {
		return nil, err
	}
	return &Server{
		addr:    addr,
		hostKey: signer,
		files:   fileSvc,
		store:   st,
		log:     log,
	}, nil
}

// Addr returns the listen address.
func (s *Server) Addr() string { return s.addr }

// Run accepts connections until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	if s.log != nil {
		s.log.Info("sftp listening", "addr", ln.Addr().String())
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				if s.log != nil {
					s.log.Warn("sftp accept", "err", err)
				}
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	config := &ssh.ServerConfig{
		PasswordCallback: s.passwordAuth,
		ServerVersion:    "SSH-2.0-GoshPanel",
	}
	config.AddHostKey(s.hostKey)
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		ch, reqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := req.Type == "subsystem" && len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp"
				_ = req.Reply(ok, nil)
			}
		}(reqs)
		rootFS := &sandboxFS{svc: s.files}
		srv := sftp.NewRequestServer(ch, sftp.Handlers{
			FileGet:  rootFS,
			FilePut:  rootFS,
			FileCmd:  rootFS,
			FileList: rootFS,
		})
		if err := srv.Serve(); err != nil && err != io.EOF {
			if s.log != nil {
				s.log.Warn("sftp session", "user", sshConn.User(), "err", err)
			}
		}
	}
}

func (s *Server) passwordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	u, err := s.store.UserByUsername(conn.User())
	if err != nil {
		crypto.VerifyPassword("argon2id$3$65536$4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", string(password))
		return nil, fmt.Errorf("authentication failed")
	}
	if !crypto.VerifyPassword(u.PasswordHash, string(password)) {
		return nil, fmt.Errorf("authentication failed")
	}
	return nil, nil
}

func loadOrCreateHostKey(path string) (ssh.Signer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil {
		signer, err := ssh.ParsePrivateKey(data)
		if err == nil {
			return signer, nil
		}
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	pem, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.Bytes, 0o600); err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(priv)
}

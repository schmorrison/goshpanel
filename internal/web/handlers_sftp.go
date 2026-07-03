package web

import (
	"net/http"
)

type sftpData struct {
	Enabled  bool
	Addr     string
	FilesRoot string
	HostKey  string
}

func (s *Server) handleSFTPPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "sftp.html", "SFTP", "sftp", sftpData{
		Enabled:   s.cfg.SFTPEnabled,
		Addr:      s.cfg.SFTPAddr,
		FilesRoot: s.files.Root(),
		HostKey:   s.cfg.SFTPHostKeyPath,
	})
}

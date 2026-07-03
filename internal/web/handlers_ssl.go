package web

import (
	"net/http"

	"github.com/schmorrison/goshpanel/internal/ssl"
)

type sslData struct {
	Certs []ssl.CertInfo
	Dir   string
}

func (s *Server) handleSSLPage(w http.ResponseWriter, r *http.Request) {
	certs, err := ssl.ScanDir(s.cfg.SSLCertDir)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "ssl.html", "SSL / TLS", "ssl", sslData{Certs: certs, Dir: s.cfg.SSLCertDir})
}

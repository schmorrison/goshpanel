package web

import (
	"fmt"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/docker"
	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/installers"
)

type installersData struct {
	Specs []installers.Spec
	Flash string
}

func (s *Server) handleInstallersPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "installers.html", "App Installers", "installers", installersData{
		Specs: installers.List(),
		Flash: r.URL.Query().Get("flash"),
	})
}

func (s *Server) handleInstallerRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	spec, ok := installers.SpecByID(id)
	if !ok {
		redirectError(w, r, "/installers", fmt.Errorf("unknown installer %q", id))
		return
	}
	domain := r.FormValue("domain")
	if err := domains.ValidateName(domain); err != nil {
		redirectError(w, r, "/installers", err)
		return
	}
	yaml, err := installers.ComposeYAML(id, domain)
	if err != nil {
		redirectError(w, r, "/installers", err)
		return
	}
	stackName := installers.StackName(id, domain)
	if _, err := s.store.DockerStackByName(stackName); err != nil {
		if _, err := s.store.CreateDockerStack(stackName, yaml); err != nil {
			redirectError(w, r, "/installers", err)
			return
		}
	}
	root := fmt.Sprintf("%s/%s", s.files.Root(), domain)
	if _, err := s.store.DomainByName(domain); err != nil {
		if _, err := s.store.CreateDomain(domain, root, installers.UpstreamHostPort(spec)); err != nil {
			redirectError(w, r, "/installers", err)
			return
		}
	}
	if s.docker == nil {
		redirectFlash(w, r, "/installers", fmt.Sprintf("Installed %s on %s (stack %s) — Docker disabled; start containers manually", spec.Name, domain, stackName))
		if s.orch != nil {
			_ = s.orch.ApplyCaddy(r.Context())
		}
		s.audit(r, "installers.run", id+" "+domain)
		return
	}
	workdir := docker.StackWorkdir(s.cfg.DataDir, stackName)
	if _, err := s.docker.ComposeUp(r.Context(), workdir, yaml); err != nil {
		redirectError(w, r, "/installers", err)
		return
	}
	if s.orch != nil {
		_ = s.orch.ApplyCaddy(r.Context())
	}
	s.audit(r, "installers.run", id+" "+domain)
	redirectFlash(w, r, "/installers", fmt.Sprintf("Installed %s on %s (stack %s)", spec.Name, domain, stackName))
}

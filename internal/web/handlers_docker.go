package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/schmorrison/goshpanel/internal/docker"
	"github.com/schmorrison/goshpanel/internal/store"
)

type dockerData struct {
	Available  bool
	Containers []docker.Container
	Images     []docker.Image
	Stacks     []store.DockerStack
	Logs       string
	LogsID     string
	Output     string
	Error      string
}

func (s *Server) handleDockerPage(w http.ResponseWriter, r *http.Request) {
	data := dockerData{Available: s.docker != nil}
	if s.docker == nil {
		s.render(w, r, "docker.html", "Docker", "docker", data)
		return
	}
	var err error
	data.Containers, err = s.docker.Containers(r.Context())
	if err != nil {
		data.Error = err.Error()
	}
	data.Images, _ = s.docker.Images(r.Context())
	data.Stacks, _ = s.store.DockerStacks()
	if id := r.URL.Query().Get("logs"); id != "" {
		data.LogsID = id
		data.Logs, _ = s.docker.Logs(r.Context(), id, 200)
	}
	s.render(w, r, "docker.html", "Docker", "docker", data)
}

func (s *Server) handleDockerRun(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		redirectError(w, r, "/docker", moduleDisabled("docker"))
		return
	}
	image := r.FormValue("image")
	name := r.FormValue("name")
	ports := splitCSV(r.FormValue("ports"))
	env := splitCSV(r.FormValue("env"))
	id, err := s.docker.Run(r.Context(), image, name, ports, env)
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	s.audit(r, "docker.run", name+" "+image)
	redirectFlash(w, r, "/docker", "Started container "+docker.ShortID(id))
}

func (s *Server) handleDockerAction(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		redirectError(w, r, "/docker", moduleDisabled("docker"))
		return
	}
	id := r.FormValue("id")
	if id == "" {
		redirectError(w, r, "/docker", errRequiredFields)
		return
	}
	var err error
	switch r.PathValue("action") {
	case "start":
		err = s.docker.Start(r.Context(), id)
	case "stop":
		err = s.docker.Stop(r.Context(), id)
	case "restart":
		err = s.docker.Restart(r.Context(), id)
	case "remove":
		err = s.docker.Remove(r.Context(), id)
	default:
		redirectError(w, r, "/docker", errors.New("unknown docker action"))
		return
	}
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	s.audit(r, "docker."+r.PathValue("action"), id)
	redirectFlash(w, r, "/docker", "Container "+r.PathValue("action")+" OK")
}

func (s *Server) handleDockerLogs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/docker?logs="+r.URL.Query().Get("id"), http.StatusSeeOther)
}

func (s *Server) handleDockerStackCreate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	yaml := r.FormValue("compose_yaml")
	if name == "" || yaml == "" {
		redirectError(w, r, "/docker", errRequiredFields)
		return
	}
	if _, err := s.store.CreateDockerStack(name, yaml); err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	s.audit(r, "docker.stack.create", name)
	redirectFlash(w, r, "/docker", "Stack saved: "+name)
}

func (s *Server) handleDockerStackUp(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		redirectError(w, r, "/docker", moduleDisabled("docker"))
		return
	}
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	st, err := s.store.DockerStackByID(id)
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	workdir := docker.StackWorkdir(s.cfg.DataDir, st.Name)
	out, err := s.docker.ComposeUp(r.Context(), workdir, st.ComposeYAML)
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	s.audit(r, "docker.stack.up", st.Name)
	redirectFlash(w, r, "/docker", "Stack up: "+st.Name+" — "+out)
}

func (s *Server) handleDockerStackDown(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		redirectError(w, r, "/docker", moduleDisabled("docker"))
		return
	}
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	st, err := s.store.DockerStackByID(id)
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	workdir := docker.StackWorkdir(s.cfg.DataDir, st.Name)
	out, err := s.docker.ComposeDown(r.Context(), workdir)
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	s.audit(r, "docker.stack.down", st.Name)
	redirectFlash(w, r, "/docker", "Stack down: "+st.Name+" — "+out)
}

func (s *Server) handleDockerStackDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	if err := s.store.DeleteDockerStack(id); err != nil {
		redirectError(w, r, "/docker", err)
		return
	}
	s.audit(r, "docker.stack.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/docker", "Stack definition removed")
}

func splitCSV(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

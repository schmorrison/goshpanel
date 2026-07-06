package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/schmorrison/goshpanel/internal/docker"
	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/orchestrator"
)

func (s *Server) executeFleetControl(r *http.Request, req fleet.ControlRequest) fleet.ControlResponse {
	switch req.Action {
	case fleet.ActionPing:
		return fleet.ControlResponse{OK: true, Message: "pong"}
	case fleet.ActionBackupCreate:
		name, err := s.backups.Create()
		if err != nil {
			return fleet.ControlResponse{Message: err.Error()}
		}
		return fleet.ControlResponse{OK: true, Message: "backup created: " + name}
	case fleet.ActionDockerPing:
		if s.docker == nil {
			return fleet.ControlResponse{Message: "docker disabled"}
		}
		if err := s.docker.Ping(r.Context()); err != nil {
			return fleet.ControlResponse{Message: err.Error()}
		}
		return fleet.ControlResponse{OK: true, Message: "docker daemon OK"}
	case fleet.ActionDockerStackUp:
		return s.fleetDockerStack(r, req.Params["stack"], true)
	case fleet.ActionDockerStackDown:
		return s.fleetDockerStack(r, req.Params["stack"], false)
	case fleet.ActionApplyAll:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		for _, res := range s.orch.ApplyAll(r.Context()) {
			if !res.OK {
				return fleet.ControlResponse{Message: res.Component + ": " + res.Message}
			}
		}
		return fleet.ControlResponse{OK: true, Message: "applied all components"}
	case fleet.ActionApplyCaddy:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplyCaddy(r.Context()))
	case fleet.ActionApplyDNS:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplyCoreDNS(r.Context()))
	case fleet.ActionApplyMaddy:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplyMaddy(r.Context()))
	case fleet.ActionApplySystemd:
		if s.orch == nil {
			return fleet.ControlResponse{Message: "orchestrator disabled"}
		}
		return toControlRes(s.orch.ApplySystemd(r.Context()))
	default:
		return fleet.ControlResponse{Message: "unknown action " + req.Action}
	}
}

func (s *Server) fleetDockerStack(r *http.Request, stackName string, up bool) fleet.ControlResponse {
	if s.docker == nil {
		return fleet.ControlResponse{Message: "docker disabled"}
	}
	stackName = strings.TrimSpace(stackName)
	if stackName == "" {
		return fleet.ControlResponse{Message: "stack name required in params"}
	}
	st, err := s.store.DockerStackByName(stackName)
	if err != nil {
		return fleet.ControlResponse{Message: "unknown stack " + stackName}
	}
	workdir := docker.StackWorkdir(s.cfg.DataDir, st.Name)
	var out string
	if up {
		out, err = s.docker.ComposeUp(r.Context(), workdir, st.ComposeYAML)
	} else {
		out, err = s.docker.ComposeDown(r.Context(), workdir)
	}
	if err != nil {
		return fleet.ControlResponse{Message: err.Error()}
	}
	action := "down"
	if up {
		action = "up"
	}
	return fleet.ControlResponse{OK: true, Message: fmt.Sprintf("stack %s %s: %s", stackName, action, out)}
}

func toControlRes(res orchestrator.Result) fleet.ControlResponse {
	return fleet.ControlResponse{OK: res.OK, Message: res.Message}
}

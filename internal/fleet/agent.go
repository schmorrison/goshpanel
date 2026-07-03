package fleet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/config"
)

// AgentCredentials are persisted after a worker enrolls with the controller.
type AgentCredentials struct {
	NodeName      string    `json:"node_name"`
	NodeToken     string    `json:"node_token"`
	ControllerURL string    `json:"controller_url"`
	EnrolledAt    time.Time `json:"enrolled_at"`
}

// AgentPath returns where worker fleet credentials are stored.
func AgentPath(dataDir string) string {
	return filepath.Join(dataDir, "fleet", "agent.json")
}

// LoadAgentCredentials reads persisted worker credentials, if any.
func LoadAgentCredentials(dataDir string) (AgentCredentials, error) {
	data, err := os.ReadFile(AgentPath(dataDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AgentCredentials{}, ErrNotEnrolled
		}
		return AgentCredentials{}, err
	}
	var creds AgentCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return AgentCredentials{}, fmt.Errorf("decode agent credentials: %w", err)
	}
	if creds.NodeName == "" || creds.NodeToken == "" || creds.ControllerURL == "" {
		return AgentCredentials{}, errors.New("incomplete agent credentials")
	}
	return creds, nil
}

// SaveAgentCredentials writes worker credentials after enrollment.
func SaveAgentCredentials(dataDir string, creds AgentCredentials) error {
	if creds.NodeName == "" || creds.NodeToken == "" || creds.ControllerURL == "" {
		return errors.New("incomplete agent credentials")
	}
	path := AgentPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// WorkerRuntime is the effective fleet worker configuration after merging env
// vars with persisted agent credentials.
type WorkerRuntime struct {
	NodeName      string
	NodeToken     string
	ControllerURL string
	BaseURL       string
	Enrolled      bool
}

// ResolveWorkerRuntime merges config env vars with agent.json for workers.
func ResolveWorkerRuntime(cfg config.Config) WorkerRuntime {
	rt := WorkerRuntime{
		NodeName:      cfg.FleetNodeName,
		NodeToken:     cfg.FleetToken,
		ControllerURL: cfg.FleetControllerURL,
		BaseURL:       workerBaseURL(cfg),
	}
	if creds, err := LoadAgentCredentials(cfg.DataDir); err == nil {
		if rt.NodeName == "" || rt.NodeName == cfg.FleetNodeName {
			rt.NodeName = creds.NodeName
		}
		if rt.NodeToken == "" {
			rt.NodeToken = creds.NodeToken
		}
		if rt.ControllerURL == "" {
			rt.ControllerURL = creds.ControllerURL
		}
		rt.Enrolled = true
	}
	return rt
}

// WorkerNeedsEnroll reports whether a worker should call the enroll API.
func WorkerNeedsEnroll(cfg config.Config, rt WorkerRuntime) bool {
	if cfg.FleetEnrollToken == "" {
		return false
	}
	if rt.NodeToken != "" && rt.ControllerURL != "" && rt.Enrolled {
		return false
	}
	return cfg.FleetControllerURL != "" || rt.ControllerURL != ""
}

func workerBaseURL(cfg config.Config) string {
	if u := strings.TrimSpace(cfg.FleetPublicURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = ":4674"
	}
	host := "127.0.0.1"
	port := "4674"
	if strings.HasPrefix(addr, ":") {
		port = strings.TrimPrefix(addr, ":")
	} else if strings.Contains(addr, ":") {
		host, port, _ = strings.Cut(addr, ":")
		if host == "" {
			host = "127.0.0.1"
		}
	} else {
		host = addr
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

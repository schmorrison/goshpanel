// Package fleet implements multi-instance GoshPanel management: a controller
// polls or receives telemetry from worker nodes and sends remote control commands.
package fleet

import (
	"encoding/json"
	"time"

	"github.com/schmorrison/goshpanel/internal/system"
)

// Mode is how this instance participates in a fleet.
type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeController Mode = "controller"
	ModeWorker     Mode = "worker"
	ModeBoth       Mode = "both" // controller that also reports as a worker
)

// Telemetry is the JSON payload every node exposes and reports.
type Telemetry struct {
	NodeName    string        `json:"node_name"`
	Version     string        `json:"version"`
	CollectedAt time.Time     `json:"collected_at"`
	Stats       system.Stats  `json:"stats"`
	Counts      ModuleCounts  `json:"counts"`
	Orchestrator []ComponentStatus `json:"orchestrator,omitempty"`
}

// ModuleCounts summarizes panel state for fleet dashboards.
type ModuleCounts struct {
	Domains    int `json:"domains"`
	Mailboxes  int `json:"mailboxes"`
	CronJobs   int `json:"cron_jobs"`
	Databases  int `json:"databases"`
	Functions  int `json:"functions"`
	DockerStacks int `json:"docker_stacks"`
	Users      int `json:"users"`
}

// ComponentStatus is orchestrator apply state for one daemon.
type ComponentStatus struct {
	Component  string     `json:"component"`
	ConfigPath string     `json:"config_path"`
	LastApplied *time.Time `json:"last_applied,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

// ControlRequest is sent to a remote node.
type ControlRequest struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

// ControlResponse is returned after a remote action.
type ControlResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// IngestRequest is pushed from a worker to the controller.
type IngestRequest struct {
	NodeName  string    `json:"node_name"`
	Telemetry Telemetry `json:"telemetry"`
}

// Supported control actions.
const (
	ActionApplyAll       = "apply_all"
	ActionApplyCaddy     = "apply_caddy"
	ActionApplyDNS       = "apply_coredns"
	ActionApplyMaddy     = "apply_maddy"
	ActionApplySystemd   = "apply_systemd"
	ActionPing           = "ping"
	ActionBackupCreate   = "backup_create"
	ActionDockerPing     = "docker_ping"
	ActionDockerStackUp  = "docker_stack_up"
	ActionDockerStackDown = "docker_stack_down"
)

// ParseControlRequest decodes a control request body.
func ParseControlRequest(data []byte) (ControlRequest, error) {
	var req ControlRequest
	err := json.Unmarshal(data, &req)
	return req, err
}

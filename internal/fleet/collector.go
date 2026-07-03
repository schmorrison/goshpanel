package fleet

import (
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/system"
	"github.com/schmorrison/goshpanel/internal/version"
)

// Collector gathers telemetry from local panel state.
type Collector struct {
	store    *store.Store
	nodeName string
	diskPath string
}

// NewCollector creates a telemetry collector.
func NewCollector(st *store.Store, nodeName, diskPath string) *Collector {
	return &Collector{store: st, nodeName: nodeName, diskPath: diskPath}
}

// Snapshot builds the current telemetry payload.
func (c *Collector) Snapshot() (Telemetry, error) {
	t := Telemetry{
		NodeName:    c.nodeName,
		Version:     version.Version,
		CollectedAt: time.Now().UTC().Truncate(time.Second),
		Stats:       system.Snapshot(c.diskPath),
	}
	t.Counts = ModuleCounts{}
	if n, err := c.store.Domains(); err == nil {
		t.Counts.Domains = len(n)
	}
	if n, err := c.store.Mailboxes(); err == nil {
		t.Counts.Mailboxes = len(n)
	}
	if n, err := c.store.CronJobs(); err == nil {
		t.Counts.CronJobs = len(n)
	}
	if n, err := c.store.DatabaseConns(); err == nil {
		t.Counts.Databases = len(n)
	}
	if n, err := c.store.MicroFunctions(); err == nil {
		t.Counts.Functions = len(n)
	}
	if n, err := c.store.DockerStacks(); err == nil {
		t.Counts.DockerStacks = len(n)
	}
	if n, err := c.store.Users(); err == nil {
		t.Counts.Users = len(n)
	}
	if statuses, err := c.store.OrchestratorStatuses(); err == nil {
		for _, s := range statuses {
			cs := ComponentStatus{Component: s.Component, ConfigPath: s.ConfigPath, LastError: s.LastError}
			if s.LastApplied != nil {
				tm := *s.LastApplied
				cs.LastApplied = &tm
			}
			t.Orchestrator = append(t.Orchestrator, cs)
		}
	}
	return t, nil
}

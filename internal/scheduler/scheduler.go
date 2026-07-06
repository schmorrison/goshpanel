package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/schmorrison/goshpanel/internal/backups"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/ddns"
	"github.com/schmorrison/goshpanel/internal/fn"
	"github.com/schmorrison/goshpanel/internal/health"
	"github.com/schmorrison/goshpanel/internal/store"
)

// Start runs periodic background jobs until ctx is cancelled.
func Start(ctx context.Context, cfg config.Config, st *store.Store, backups *backups.Service, fns *fn.Service, log *slog.Logger) {
	interval := time.Duration(cfg.FleetIntervalSeconds) * time.Second
	if interval < 30*time.Second {
		interval = time.Minute
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	run(ctx, cfg, st, backups, fns, log)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			run(ctx, cfg, st, backups, fns, log)
		}
	}
}

func run(ctx context.Context, cfg config.Config, st *store.Store, backupSvc *backups.Service, fns *fn.Service, log *slog.Logger) {
	runHealthChecks(ctx, st, log)
	runBackupSchedules(ctx, st, backupSvc, log)
	runFunctionSchedules(ctx, st, fns, log)
	runDDNS(ctx, st, log)
}

func runHealthChecks(ctx context.Context, st *store.Store, log *slog.Logger) {
	checks, err := st.HealthChecks()
	if err != nil {
		return
	}
	for _, c := range checks {
		if !c.Enabled {
			continue
		}
		res := health.Probe(ctx, c)
		status := "down"
		if res.OK {
			status = "up"
		}
		_ = st.UpdateHealthCheckResult(c.ID, status, res.Message)
		if !res.OK && log != nil {
			log.Warn("health check failed", "name", c.Name, "url", c.URL, "msg", res.Message)
			_ = st.CreateAlert("health", c.Name+": "+res.Message)
		}
	}
}

func runBackupSchedules(ctx context.Context, st *store.Store, backupSvc *backups.Service, log *slog.Logger) {
	if backupSvc == nil {
		return
	}
	schedules, err := st.BackupSchedules()
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, sch := range schedules {
		if !sch.Enabled {
			continue
		}
		if sch.LastRunAt != nil && now.Sub(*sch.LastRunAt) < time.Duration(sch.IntervalHours)*time.Hour {
			continue
		}
		name, err := backupSvc.Create()
		if err != nil {
			if log != nil {
				log.Error("scheduled backup", "err", err)
			}
			continue
		}
		_ = st.TouchBackupSchedule(sch.ID)
		if log != nil {
			log.Info("scheduled backup created", "name", name)
		}
	}
}

func runFunctionSchedules(ctx context.Context, st *store.Store, fns *fn.Service, log *slog.Logger) {
	if fns == nil {
		return
	}
	schedules, err := st.FunctionSchedules()
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, sch := range schedules {
		if !sch.Enabled {
			continue
		}
		if sch.LastRunAt != nil && now.Sub(*sch.LastRunAt) < time.Duration(sch.IntervalMinutes)*time.Minute {
			continue
		}
		if _, err := fns.InvokeByID(ctx, sch.FunctionID); err != nil && log != nil {
			log.Warn("function schedule", "id", sch.FunctionID, "err", err)
		}
		_ = st.TouchFunctionSchedule(sch.ID)
	}
}

func runDDNS(ctx context.Context, st *store.Store, log *slog.Logger) {
	configs, err := st.DDNSConfigs()
	if err != nil {
		return
	}
	ip, err := ddns.PublicIP(ctx)
	if err != nil {
		return
	}
	for _, c := range configs {
		if err := ddns.Update(ctx, c, ip); err != nil && log != nil {
			log.Warn("ddns update", "host", c.Hostname, "err", err)
		}
	}
}

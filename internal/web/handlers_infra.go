package web

import (
	"net/http"
	"strconv"

	"github.com/schmorrison/goshpanel/internal/backups"
	"github.com/schmorrison/goshpanel/internal/cron"
	"github.com/schmorrison/goshpanel/internal/logs"
	"github.com/schmorrison/goshpanel/internal/runner"
	"github.com/schmorrison/goshpanel/internal/store"
)

// --- cron ---

func (s *Server) handleCronPage(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.CronJobs()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "cron.html", "Cron Jobs", "cron", jobs)
}

func (s *Server) handleCronCreate(w http.ResponseWriter, r *http.Request) {
	schedule := r.FormValue("schedule")
	command := r.FormValue("command")
	if err := cron.ValidateSchedule(schedule); err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	if command == "" {
		redirectFlash(w, r, "/cron", "Command is required")
		return
	}
	if _, err := s.store.CreateCronJob(schedule, command, r.FormValue("comment")); err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	s.audit(r, "cron.create", schedule+" "+command)
	redirectFlash(w, r, "/cron", "Cron job added")
}

func (s *Server) handleCronDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	if err := s.store.DeleteCronJob(id); err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	s.audit(r, "cron.delete", strconv.FormatInt(id, 10))
	redirectFlash(w, r, "/cron", "Cron job removed")
}

func (s *Server) handleCrontab(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.CronJobs()
	if err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="crontab"`)
	w.Write([]byte(cron.RenderCrontab(jobs)))
}

func (s *Server) handleCronApply(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.CronJobs()
	if err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	if err := cron.Apply(r.Context(), jobs); err != nil {
		redirectError(w, r, "/cron", err)
		return
	}
	s.audit(r, "cron.apply", "installed crontab")
	redirectFlash(w, r, "/cron", "Crontab installed for the panel user")
}

// --- backups ---

type backupsData struct {
	Backups []backups.Info
	SrcDir  string
}

func (s *Server) handleBackupsPage(w http.ResponseWriter, r *http.Request) {
	list, err := s.backups.List()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "backups.html", "Backups", "backups", backupsData{Backups: list, SrcDir: s.files.Root()})
}

func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	name, err := s.backups.Create()
	if err != nil {
		redirectError(w, r, "/backups", err)
		return
	}
	s.audit(r, "backups.create", name)
	redirectFlash(w, r, "/backups", "Backup created: "+name)
}

func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if err := s.backups.Restore(name); err != nil {
		redirectError(w, r, "/backups", err)
		return
	}
	s.audit(r, "backups.restore", name)
	redirectFlash(w, r, "/backups", "Restored "+name)
}

func (s *Server) handleBackupDelete(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if err := s.backups.Delete(name); err != nil {
		redirectError(w, r, "/backups", err)
		return
	}
	s.audit(r, "backups.delete", name)
	redirectFlash(w, r, "/backups", "Deleted "+name)
}

func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	f, info, err := s.backups.Open(name)
	if err != nil {
		redirectError(w, r, "/backups", err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", `attachment; filename="`+info.Name()+`"`)
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// --- logs ---

type logsData struct {
	Sources  []logs.Source
	Selected string
	Lines    []string
	Count    int
}

func (s *Server) handleLogsPage(w http.ResponseWriter, r *http.Request) {
	data := logsData{Sources: s.logs.Sources(), Count: 200}
	if n, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && n > 0 && n <= 5000 {
		data.Count = n
	}
	if p := r.URL.Query().Get("path"); p != "" {
		data.Selected = p
		lines, err := s.logs.Tail(p, data.Count)
		if err != nil {
			redirectError(w, r, "/logs", err)
			return
		}
		data.Lines = lines
	}
	s.render(w, r, "logs.html", "Log Viewer", "logs", data)
}

// --- terminal (command runner) ---

type terminalData struct {
	Enabled bool
	Workdir string
	Command string
	Result  *runner.Result
}

func (s *Server) handleTerminalPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "terminal.html", "Terminal", "terminal", terminalData{
		Enabled: s.runner.Enabled(), Workdir: s.files.Root(),
	})
}

func (s *Server) handleTerminalRun(w http.ResponseWriter, r *http.Request) {
	command := r.FormValue("command")
	res, err := s.runner.Run(r.Context(), command)
	data := terminalData{Enabled: s.runner.Enabled(), Workdir: s.files.Root(), Command: command}
	if err != nil {
		res = runner.Result{Command: command, Output: err.Error(), ExitCode: -1}
	}
	data.Result = &res
	s.audit(r, "terminal.run", command)
	s.render(w, r, "terminal.html", "Terminal", "terminal", data)
}

// --- security ---

type securityData struct {
	Users   []store.User
	IPRules []store.IPRule
	Audit   []store.AuditEntry
}

func (s *Server) handleSecurityPage(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	rules, err := s.store.IPRules()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	audit, err := s.store.AuditEntries(100)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "security.html", "Security", "security", securityData{
		Users: users, IPRules: rules, Audit: audit,
	})
}

package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/schmorrison/goshpanel/internal/dns"
	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/store"
)

type domainRow struct {
	Domain  store.Domain
	Records []store.DNSRecord
}

type domainsData struct {
	Rows        []domainRow
	RecordTypes []string
}

func (s *Server) handleDomainsPage(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Domains()
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	rows := make([]domainRow, 0, len(list))
	for _, d := range list {
		recs, err := s.store.DNSRecords(d.ID)
		if err != nil {
			redirectError(w, r, "/", err)
			return
		}
		rows = append(rows, domainRow{Domain: d, Records: recs})
	}
	s.render(w, r, "domains.html", "Domains & DNS", "domains", domainsData{
		Rows: rows, RecordTypes: dns.RecordTypes,
	})
}

func (s *Server) handleDomainCreate(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.FormValue("name")))
	if err := domains.ValidateName(name); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	if _, err := s.store.CreateDomain(name, r.FormValue("root"), r.FormValue("upstream")); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	s.audit(r, "domains.create", name)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/domains", "Added domain "+name)
}

func (s *Server) handleDomainDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	if err := s.store.DeleteDomain(id); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	s.audit(r, "domains.delete", strconv.FormatInt(id, 10))
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/domains", "Domain removed")
}

func (s *Server) handleCaddyfile(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Domains()
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Caddyfile"`)
	w.Write([]byte(domains.RenderCaddyfile(list, s.cfg.CaddyAccessLog)))
}

func (s *Server) handleZoneFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	d, err := s.store.DomainByID(id)
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	recs, err := s.store.DNSRecords(id)
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+d.Name+`.zone"`)
	w.Write([]byte(dns.RenderZone(d, recs)))
}

func (s *Server) handleDNSCreate(w http.ResponseWriter, r *http.Request) {
	domainID, err := formID(r, "domain_id")
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	ttl, _ := strconv.Atoi(r.FormValue("ttl"))
	prio, _ := strconv.Atoi(r.FormValue("priority"))
	rec := store.DNSRecord{
		DomainID: domainID,
		Type:     r.FormValue("type"),
		Name:     strings.TrimSpace(r.FormValue("name")),
		Value:    strings.TrimSpace(r.FormValue("value")),
		TTL:      ttl,
		Priority: prio,
	}
	if err := dns.Validate(rec); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	if _, err := s.store.CreateDNSRecord(rec); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	s.audit(r, "dns.create", rec.Type+" "+rec.Name)
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/domains", "Record added")
}

func (s *Server) handleDNSDelete(w http.ResponseWriter, r *http.Request) {
	id, err := formID(r, "id")
	if err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	if err := s.store.DeleteDNSRecord(id); err != nil {
		redirectError(w, r, "/domains", err)
		return
	}
	s.audit(r, "dns.delete", strconv.FormatInt(id, 10))
	s.maybeAutoApply(r.Context())
	redirectFlash(w, r, "/domains", "Record removed")
}

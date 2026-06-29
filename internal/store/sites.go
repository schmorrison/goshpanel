package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Site represents a reverse-proxy virtual host managed by the panel.
type Site struct {
	ID       int64
	Domain   string
	Upstream string
	Enabled  bool
}

// SiteRepository persists virtual host definitions.
type SiteRepository struct {
	db *sql.DB
}

// NewSiteRepository returns a site repository backed by db.
func NewSiteRepository(db *sql.DB) *SiteRepository {
	return &SiteRepository{db: db}
}

// List returns all configured sites.
func (r *SiteRepository) List(ctx context.Context) ([]Site, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, domain, upstream, enabled
		FROM sites
		ORDER BY domain COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var site Site
		var enabled int
		if err := rows.Scan(&site.ID, &site.Domain, &site.Upstream, &enabled); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		site.Enabled = enabled == 1
		sites = append(sites, site)
	}

	return sites, rows.Err()
}

// Create inserts a new site.
func (r *SiteRepository) Create(ctx context.Context, domain, upstream string) (Site, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO sites (domain, upstream, enabled)
		VALUES (?, ?, 1)
	`, domain, upstream)
	if err != nil {
		return Site{}, fmt.Errorf("create site: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Site{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

// Delete removes a site by id.
func (r *SiteRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	return nil
}

// FindByID returns a site by id.
func (r *SiteRepository) FindByID(ctx context.Context, id int64) (Site, error) {
	var site Site
	var enabled int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, domain, upstream, enabled
		FROM sites
		WHERE id = ?
	`, id).Scan(&site.ID, &site.Domain, &site.Upstream, &enabled)
	if err != nil {
		return Site{}, fmt.Errorf("find site: %w", err)
	}
	site.Enabled = enabled == 1
	return site, nil
}

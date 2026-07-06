package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DockerStack is a saved docker-compose project.
type DockerStack struct {
	ID          int64
	Name        string
	ComposeYAML string
	CreatedAt   time.Time
}

// CreateDockerStack inserts a compose stack definition.
func (s *Store) CreateDockerStack(name, composeYAML string) (DockerStack, error) {
	st := DockerStack{Name: name, ComposeYAML: composeYAML, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO docker_stacks (name, compose_yaml, created_at) VALUES (?, ?, ?)`,
		st.Name, st.ComposeYAML, st.CreatedAt)
	if err != nil {
		return DockerStack{}, fmt.Errorf("create docker stack: %w", err)
	}
	st.ID, _ = res.LastInsertId()
	return st, nil
}

// DockerStacks lists all stacks.
func (s *Store) DockerStacks() ([]DockerStack, error) {
	rows, err := s.db.Query(`SELECT id, name, compose_yaml, created_at FROM docker_stacks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list docker stacks: %w", err)
	}
	defer rows.Close()
	var out []DockerStack
	for rows.Next() {
		var st DockerStack
		if err := rows.Scan(&st.ID, &st.Name, &st.ComposeYAML, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// DockerStackByID returns one stack.
func (s *Store) DockerStackByID(id int64) (DockerStack, error) {
	var st DockerStack
	err := s.db.QueryRow(`SELECT id, name, compose_yaml, created_at FROM docker_stacks WHERE id = ?`, id).
		Scan(&st.ID, &st.Name, &st.ComposeYAML, &st.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DockerStack{}, ErrNotFound
	}
	return st, err
}

// DockerStackByName returns one stack by name.
func (s *Store) DockerStackByName(name string) (DockerStack, error) {
	var st DockerStack
	err := s.db.QueryRow(`SELECT id, name, compose_yaml, created_at FROM docker_stacks WHERE name = ?`, name).
		Scan(&st.ID, &st.Name, &st.ComposeYAML, &st.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DockerStack{}, ErrNotFound
	}
	return st, err
}

// DeleteDockerStack removes a stack definition.
func (s *Store) DeleteDockerStack(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM docker_stacks WHERE id = ?`, id))
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrProjectNotFound is returned when a project is not found.
	ErrProjectNotFound = errors.New("project not found")
	// ErrProjectExists is returned when a project already exists.
	ErrProjectExists = errors.New("project already exists")
)

// CreateProject creates a new project in the database.
func (d *DB) CreateProject(ctx context.Context, project Project) error {
	query := `
	INSERT INTO projects (id, name, openai_api_key, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := d.db.ExecContext(
		ctx,
		query,
		project.ID,
		project.Name,
		project.OpenAIAPIKey,
		project.CreatedAt,
		project.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	return nil
}

// GetProjectByID retrieves a project by ID.
func (d *DB) GetProjectByID(ctx context.Context, id string) (Project, error) {
	query := `
	SELECT id, name, openai_api_key, created_at, updated_at
	FROM projects
	WHERE id = ?
	`

	var project Project
	err := d.db.QueryRowContext(ctx, query, id).Scan(
		&project.ID,
		&project.Name,
		&project.OpenAIAPIKey,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// GetProjectByName retrieves a project by name.
func (d *DB) GetProjectByName(ctx context.Context, name string) (Project, error) {
	query := `
	SELECT id, name, openai_api_key, created_at, updated_at
	FROM projects
	WHERE name = ?
	`

	var project Project
	err := d.db.QueryRowContext(ctx, query, name).Scan(
		&project.ID,
		&project.Name,
		&project.OpenAIAPIKey,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// UpdateProject updates a project in the database.
func (d *DB) UpdateProject(ctx context.Context, project Project) error {
	project.UpdatedAt = time.Now()

	query := `
	UPDATE projects
	SET name = ?, openai_api_key = ?, updated_at = ?
	WHERE id = ?
	`

	result, err := d.db.ExecContext(
		ctx,
		query,
		project.Name,
		project.OpenAIAPIKey,
		project.UpdatedAt,
		project.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrProjectNotFound
	}

	return nil
}

// DeleteProject deletes a project from the database.
func (d *DB) DeleteProject(ctx context.Context, id string) error {
	query := `
	DELETE FROM projects
	WHERE id = ?
	`

	result, err := d.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrProjectNotFound
	}

	return nil
}

// ListProjects retrieves all projects from the database.
func (d *DB) ListProjects(ctx context.Context) ([]Project, error) {
	query := `
	SELECT id, name, openai_api_key, created_at, updated_at
	FROM projects
	ORDER BY name ASC
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var projects []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.OpenAIAPIKey,
			&project.CreatedAt,
			&project.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return projects, nil
}

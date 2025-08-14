package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

var (
	// ErrProjectNotFound is returned when a project is not found.
	ErrProjectNotFound = errors.New("project not found")
	// ErrProjectExists is returned when a project already exists.
	ErrProjectExists = errors.New("project already exists")
)

// GetProjectByName retrieves a project by name.
func (d *DB) GetProjectByName(ctx context.Context, name string) (Project, error) {
	query := `
	SELECT id, name, openai_api_key, is_active, deactivated_at, created_at, updated_at
	FROM projects
	WHERE name = ?
	`

	var project Project
	var deactivatedAt sql.NullTime
	err := d.db.QueryRowContext(ctx, query, name).Scan(
		&project.ID,
		&project.Name,
		&project.OpenAIAPIKey,
		&project.IsActive,
		&deactivatedAt,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, fmt.Errorf("failed to get project: %w", err)
	}

	if deactivatedAt.Valid {
		project.DeactivatedAt = &deactivatedAt.Time
	}

	return project, nil
}

// ToProxyProject converts a database.Project to a proxy.Project
func ToProxyProject(dbProject Project) proxy.Project {
	return proxy.Project{
		ID:            dbProject.ID,
		Name:          dbProject.Name,
		OpenAIAPIKey:  dbProject.OpenAIAPIKey,
		IsActive:      dbProject.IsActive,
		DeactivatedAt: dbProject.DeactivatedAt,
		CreatedAt:     dbProject.CreatedAt,
		UpdatedAt:     dbProject.UpdatedAt,
	}
}

// ToDBProject converts a proxy.Project to a database.Project
func ToDBProject(proxyProject proxy.Project) Project {
	return Project{
		ID:            proxyProject.ID,
		Name:          proxyProject.Name,
		OpenAIAPIKey:  proxyProject.OpenAIAPIKey,
		IsActive:      proxyProject.IsActive,
		DeactivatedAt: proxyProject.DeactivatedAt,
		CreatedAt:     proxyProject.CreatedAt,
		UpdatedAt:     proxyProject.UpdatedAt,
	}
}

// Rename CRUD methods for DB store
func (d *DB) DBListProjects(ctx context.Context) ([]Project, error) {
	query := `
	SELECT id, name, openai_api_key, is_active, deactivated_at, created_at, updated_at
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
		var deactivatedAt sql.NullTime
		if err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.OpenAIAPIKey,
			&project.IsActive,
			&deactivatedAt,
			&project.CreatedAt,
			&project.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		if deactivatedAt.Valid {
			project.DeactivatedAt = &deactivatedAt.Time
		}
		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return projects, nil
}

func (d *DB) DBCreateProject(ctx context.Context, project Project) error {
	query := `
	INSERT INTO projects (id, name, openai_api_key, is_active, deactivated_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.ExecContext(
		ctx,
		query,
		project.ID,
		project.Name,
		project.OpenAIAPIKey,
		project.IsActive,
		project.DeactivatedAt,
		project.CreatedAt,
		project.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	return nil
}

func (d *DB) DBGetProjectByID(ctx context.Context, projectID string) (Project, error) {
	query := `
	SELECT id, name, openai_api_key, is_active, deactivated_at, created_at, updated_at
	FROM projects
	WHERE id = ?
	`

	var project Project
	var deactivatedAt sql.NullTime
	err := d.db.QueryRowContext(ctx, query, projectID).Scan(
		&project.ID,
		&project.Name,
		&project.OpenAIAPIKey,
		&project.IsActive,
		&deactivatedAt,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, fmt.Errorf("failed to get project: %w", err)
	}

	if deactivatedAt.Valid {
		project.DeactivatedAt = &deactivatedAt.Time
	}

	return project, nil
}

func (d *DB) DBUpdateProject(ctx context.Context, project Project) error {
	project.UpdatedAt = time.Now()

	query := `
	UPDATE projects
	SET name = ?, openai_api_key = ?, is_active = ?, deactivated_at = ?, updated_at = ?
	WHERE id = ?
	`

	result, err := d.db.ExecContext(
		ctx,
		query,
		project.Name,
		project.OpenAIAPIKey,
		project.IsActive,
		project.DeactivatedAt,
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

func (d *DB) DBDeleteProject(ctx context.Context, projectID string) error {
	query := `
	DELETE FROM projects
	WHERE id = ?
	`

	result, err := d.db.ExecContext(ctx, query, projectID)
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

// --- proxy.ProjectStore interface adapters ---
func (d *DB) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	dbProjects, err := d.DBListProjects(ctx)
	if err != nil {
		return nil, err
	}
	var out []proxy.Project
	for _, p := range dbProjects {
		out = append(out, ToProxyProject(p))
	}
	return out, nil
}

func (d *DB) CreateProject(ctx context.Context, p proxy.Project) error {
	return d.DBCreateProject(ctx, ToDBProject(p))
}

func (d *DB) GetProjectByID(ctx context.Context, id string) (proxy.Project, error) {
	dbP, err := d.DBGetProjectByID(ctx, id)
	if err != nil {
		return proxy.Project{}, err
	}
	return ToProxyProject(dbP), nil
}

func (d *DB) UpdateProject(ctx context.Context, p proxy.Project) error {
	return d.DBUpdateProject(ctx, ToDBProject(p))
}

func (d *DB) DeleteProject(ctx context.Context, id string) error {
	return d.DBDeleteProject(ctx, id)
}

// GetAPIKeyForProject retrieves the OpenAI API key for a project by ID
func (d *DB) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	query := `SELECT openai_api_key FROM projects WHERE id = ?`
	var apiKey string
	err := d.db.QueryRowContext(ctx, query, projectID).Scan(&apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrProjectNotFound
		}
		return "", fmt.Errorf("failed to get API key for project: %w", err)
	}
	return apiKey, nil
}

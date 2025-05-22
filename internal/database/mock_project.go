package database

import (
	"context"
	"errors"
	"sync"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

// MockProjectStore is an in-memory implementation of ProjectStore for testing and development
type MockProjectStore struct {
	projects map[string]Project
	apiKeys  map[string]string // Project ID -> API Key mapping
	mutex    sync.RWMutex
}

// NewMockProjectStore creates a new MockProjectStore
func NewMockProjectStore() *MockProjectStore {
	return &MockProjectStore{
		projects: make(map[string]Project),
		apiKeys:  make(map[string]string),
	}
}

// CreateProject creates a new project in the store
func (m *MockProjectStore) DBCreateProject(ctx context.Context, project Project) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.projects[project.ID]; exists {
		return errors.New("project already exists")
	}

	m.projects[project.ID] = project
	m.apiKeys[project.ID] = project.OpenAIAPIKey
	return nil
}

// GetProjectByID retrieves a project by ID
func (m *MockProjectStore) DBGetProjectByID(ctx context.Context, projectID string) (Project, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return Project{}, errors.New("project not found")
	}

	return project, nil
}

// UpdateProject updates a project in the store
func (m *MockProjectStore) DBUpdateProject(ctx context.Context, project Project) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.projects[project.ID]; !exists {
		return errors.New("project not found")
	}

	m.projects[project.ID] = project
	m.apiKeys[project.ID] = project.OpenAIAPIKey
	return nil
}

// DeleteProject deletes a project from the store
func (m *MockProjectStore) DBDeleteProject(ctx context.Context, projectID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.projects[projectID]; !exists {
		return errors.New("project not found")
	}

	delete(m.projects, projectID)
	delete(m.apiKeys, projectID)
	return nil
}

// ListProjects retrieves all projects from the store
func (m *MockProjectStore) DBListProjects(ctx context.Context) ([]Project, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	projects := make([]Project, 0, len(m.projects))
	for _, p := range m.projects {
		projects = append(projects, p)
	}
	return projects, nil
}

// CreateMockProject creates a new project in the mock store with the given parameters
func (m *MockProjectStore) CreateMockProject(projectID, name, apiKey string) (Project, error) {
	if projectID == "" {
		return Project{}, errors.New("project ID cannot be empty")
	}
	if name == "" {
		return Project{}, errors.New("project name cannot be empty")
	}
	if apiKey == "" {
		return Project{}, errors.New("API key cannot be empty")
	}

	project := Project{
		ID:           projectID,
		Name:         name,
		OpenAIAPIKey: apiKey,
	}

	err := m.DBCreateProject(context.Background(), project)
	return project, err
}

// GetAPIKeyForProject retrieves the API key for a project
func (m *MockProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	apiKey, exists := m.apiKeys[projectID]
	if !exists {
		return "", errors.New("project not found")
	}

	return apiKey, nil
}

// --- proxy.ProjectStore interface adapters ---
func (m *MockProjectStore) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	dbProjects, err := m.DBListProjects(ctx)
	if err != nil {
		return nil, err
	}
	var out []proxy.Project
	for _, p := range dbProjects {
		out = append(out, ToProxyProject(p))
	}
	return out, nil
}

func (m *MockProjectStore) CreateProject(ctx context.Context, p proxy.Project) error {
	return m.DBCreateProject(ctx, ToDBProject(p))
}

func (m *MockProjectStore) GetProjectByID(ctx context.Context, id string) (proxy.Project, error) {
	dbP, err := m.DBGetProjectByID(ctx, id)
	if err != nil {
		return proxy.Project{}, err
	}
	return ToProxyProject(dbP), nil
}

func (m *MockProjectStore) UpdateProject(ctx context.Context, p proxy.Project) error {
	return m.DBUpdateProject(ctx, ToDBProject(p))
}

func (m *MockProjectStore) DeleteProject(ctx context.Context, id string) error {
	return m.DBDeleteProject(ctx, id)
}

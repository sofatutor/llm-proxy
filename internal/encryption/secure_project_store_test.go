package encryption

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

// mockProjectStore is a mock implementation of proxy.ProjectStore for testing.
type mockProjectStore struct {
	projects       map[string]proxy.Project
	getAPIKeyError error
	createError    error
	updateError    error
	deleteError    error
	listError      error
	getByIDError   error
}

func newMockProjectStore() *mockProjectStore {
	return &mockProjectStore{
		projects: make(map[string]proxy.Project),
	}
}

func (m *mockProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	if m.getAPIKeyError != nil {
		return "", m.getAPIKeyError
	}
	p, ok := m.projects[projectID]
	if !ok {
		return "", errors.New("project not found")
	}
	return p.APIKey, nil
}

func (m *mockProjectStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	p, ok := m.projects[projectID]
	if !ok {
		return false, errors.New("project not found")
	}
	return p.IsActive, nil
}

func (m *mockProjectStore) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	result := make([]proxy.Project, 0, len(m.projects))
	for _, p := range m.projects {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockProjectStore) CreateProject(ctx context.Context, project proxy.Project) error {
	if m.createError != nil {
		return m.createError
	}
	m.projects[project.ID] = project
	return nil
}

func (m *mockProjectStore) GetProjectByID(ctx context.Context, projectID string) (proxy.Project, error) {
	if m.getByIDError != nil {
		return proxy.Project{}, m.getByIDError
	}
	p, ok := m.projects[projectID]
	if !ok {
		return proxy.Project{}, errors.New("project not found")
	}
	return p, nil
}

func (m *mockProjectStore) UpdateProject(ctx context.Context, project proxy.Project) error {
	if m.updateError != nil {
		return m.updateError
	}
	if _, ok := m.projects[project.ID]; !ok {
		return errors.New("project not found")
	}
	m.projects[project.ID] = project
	return nil
}

func (m *mockProjectStore) DeleteProject(ctx context.Context, projectID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	if _, ok := m.projects[projectID]; !ok {
		return errors.New("project not found")
	}
	delete(m.projects, projectID)
	return nil
}

func TestNewSecureProjectStore(t *testing.T) {
	mock := newMockProjectStore()

	t.Run("with encryptor", func(t *testing.T) {
		key, _ := GenerateKey()
		enc, _ := NewEncryptor(key)
		store := NewSecureProjectStore(mock, enc)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})

	t.Run("nil encryptor uses NullEncryptor", func(t *testing.T) {
		store := NewSecureProjectStore(mock, nil)
		if store == nil {
			t.Error("expected store, got nil")
		}
		// Verify it behaves like NullEncryptor
		apiKey := "test-api-key"
		project := proxy.Project{
			ID:        "proj-1",
			Name:      "Test Project",
			APIKey:    apiKey,
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		ctx := context.Background()
		if err := store.CreateProject(ctx, project); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		// API key should not be encrypted (NullEncryptor)
		if mock.projects["proj-1"].APIKey != apiKey {
			t.Errorf("API key should not be encrypted with NullEncryptor")
		}
	})
}

func TestSecureProjectStore_CreateAndGetProject(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	originalAPIKey := "sk-test-api-key-12345"
	project := proxy.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		APIKey:    originalAPIKey,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create project
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Verify stored API key is encrypted
	storedProject := mock.projects["proj-1"]
	if storedProject.APIKey == originalAPIKey {
		t.Error("API key should be encrypted in storage")
	}
	if !IsEncrypted(storedProject.APIKey) {
		t.Error("stored API key should have encryption prefix")
	}

	// Get project - should decrypt
	retrieved, err := store.GetProjectByID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if retrieved.APIKey != originalAPIKey {
		t.Errorf("GetProjectByID returned wrong API key: got %q, want %q", retrieved.APIKey, originalAPIKey)
	}

	// GetAPIKeyForProject should also decrypt
	apiKey, err := store.GetAPIKeyForProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetAPIKeyForProject failed: %v", err)
	}
	if apiKey != originalAPIKey {
		t.Errorf("GetAPIKeyForProject returned wrong API key: got %q, want %q", apiKey, originalAPIKey)
	}
}

func TestSecureProjectStore_ListProjects(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	// Create multiple projects
	apiKeys := []string{"key-1", "key-2", "key-3"}
	for i, apiKey := range apiKeys {
		project := proxy.Project{
			ID:        fmt.Sprintf("proj-%d", i+1),
			Name:      fmt.Sprintf("Project %d", i+1),
			APIKey:    apiKey,
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := store.CreateProject(ctx, project); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// List projects - should decrypt all API keys
	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(projects) != len(apiKeys) {
		t.Errorf("expected %d projects, got %d", len(apiKeys), len(projects))
	}

	// Verify all API keys are decrypted
	for _, p := range projects {
		found := false
		for _, key := range apiKeys {
			if p.APIKey == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("project %s has encrypted API key: %s", p.ID, p.APIKey)
		}
	}
}

func TestSecureProjectStore_UpdateProject(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	// Create project
	project := proxy.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		APIKey:    "old-key",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Update with new plaintext key
	project.APIKey = "new-key"
	if err := store.UpdateProject(ctx, project); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify new key is encrypted in storage
	storedProject := mock.projects["proj-1"]
	if storedProject.APIKey == "new-key" {
		t.Error("API key should be encrypted in storage after update")
	}
	if !IsEncrypted(storedProject.APIKey) {
		t.Error("stored API key should have encryption prefix after update")
	}

	// Get and verify decryption
	retrieved, err := store.GetProjectByID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if retrieved.APIKey != "new-key" {
		t.Errorf("GetProjectByID returned wrong API key after update: got %q, want %q", retrieved.APIKey, "new-key")
	}
}

func TestSecureProjectStore_DeleteProject(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	// Create and delete
	project := proxy.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		APIKey:    "test-key",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := store.DeleteProject(ctx, "proj-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, ok := mock.projects["proj-1"]; ok {
		t.Error("project should be deleted")
	}
}

func TestSecureProjectStore_GetProjectActive(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	// Create active project
	project := proxy.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		APIKey:    "test-key",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	active, err := store.GetProjectActive(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetProjectActive failed: %v", err)
	}
	if !active {
		t.Error("project should be active")
	}
}

func TestSecureProjectStore_ErrorHandling(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	ctx := context.Background()

	t.Run("GetAPIKeyForProject error", func(t *testing.T) {
		mock := newMockProjectStore()
		mock.getAPIKeyError = errors.New("db error")
		store := NewSecureProjectStore(mock, enc)

		_, err := store.GetAPIKeyForProject(ctx, "proj-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("ListProjects error", func(t *testing.T) {
		mock := newMockProjectStore()
		mock.listError = errors.New("db error")
		store := NewSecureProjectStore(mock, enc)

		_, err := store.ListProjects(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CreateProject error", func(t *testing.T) {
		mock := newMockProjectStore()
		mock.createError = errors.New("db error")
		store := NewSecureProjectStore(mock, enc)

		err := store.CreateProject(ctx, proxy.Project{APIKey: "key"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetProjectByID error", func(t *testing.T) {
		mock := newMockProjectStore()
		mock.getByIDError = errors.New("db error")
		store := NewSecureProjectStore(mock, enc)

		_, err := store.GetProjectByID(ctx, "proj-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("UpdateProject error", func(t *testing.T) {
		mock := newMockProjectStore()
		mock.updateError = errors.New("db error")
		// Need to add project first since update checks existence
		mock.projects["proj-1"] = proxy.Project{ID: "proj-1", APIKey: "old-key"}
		store := NewSecureProjectStore(mock, enc)

		err := store.UpdateProject(ctx, proxy.Project{ID: "proj-1", APIKey: "new-key"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("DeleteProject error", func(t *testing.T) {
		mock := newMockProjectStore()
		mock.deleteError = errors.New("db error")
		mock.projects["proj-1"] = proxy.Project{ID: "proj-1"}
		store := NewSecureProjectStore(mock, enc)

		err := store.DeleteProject(ctx, "proj-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSecureProjectStore_BackwardCompatibility(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	// Simulate existing unencrypted data (legacy)
	legacyAPIKey := "sk-legacy-unencrypted-key"
	mock.projects["proj-legacy"] = proxy.Project{
		ID:       "proj-legacy",
		Name:     "Legacy Project",
		APIKey:   legacyAPIKey, // Stored in plaintext (legacy)
		IsActive: true,
	}

	// GetProjectByID should handle unencrypted data
	project, err := store.GetProjectByID(ctx, "proj-legacy")
	if err != nil {
		t.Fatalf("GetProjectByID failed: %v", err)
	}
	if project.APIKey != legacyAPIKey {
		t.Errorf("GetProjectByID should return unencrypted key as-is: got %q, want %q", project.APIKey, legacyAPIKey)
	}

	// GetAPIKeyForProject should also handle unencrypted data
	apiKey, err := store.GetAPIKeyForProject(ctx, "proj-legacy")
	if err != nil {
		t.Fatalf("GetAPIKeyForProject failed: %v", err)
	}
	if apiKey != legacyAPIKey {
		t.Errorf("GetAPIKeyForProject should return unencrypted key as-is: got %q, want %q", apiKey, legacyAPIKey)
	}

	// ListProjects should handle mixed encrypted/unencrypted data
	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	for _, p := range projects {
		if p.ID == "proj-legacy" && p.APIKey != legacyAPIKey {
			t.Errorf("ListProjects should return unencrypted key as-is")
		}
	}
}

func TestSecureProjectStore_UpdateAlreadyEncrypted(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)
	mock := newMockProjectStore()
	store := NewSecureProjectStore(mock, enc)
	ctx := context.Background()

	// Create project
	originalAPIKey := "test-api-key"
	project := proxy.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		APIKey:    originalAPIKey,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Get the encrypted version
	encryptedKey := mock.projects["proj-1"].APIKey

	// Update with already encrypted key (should not double-encrypt)
	project.APIKey = encryptedKey
	if err := store.UpdateProject(ctx, project); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify it wasn't double-encrypted
	retrieved, err := store.GetProjectByID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if retrieved.APIKey != originalAPIKey {
		t.Errorf("GetProjectByID returned wrong API key: got %q, want %q", retrieved.APIKey, originalAPIKey)
	}
}

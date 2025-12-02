// Package encryption provides a secure database wrapper that encrypts/decrypts sensitive fields.
package encryption

import (
	"context"
	"fmt"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

// SecureProjectStore wraps a ProjectStore and encrypts/decrypts API keys.
type SecureProjectStore struct {
	store     proxy.ProjectStore
	encryptor FieldEncryptor
}

// NewSecureProjectStore creates a new SecureProjectStore.
// The encryptor is used to encrypt API keys before storing and decrypt after retrieval.
// If encryptor is nil, a NullEncryptor is used (no encryption).
func NewSecureProjectStore(store proxy.ProjectStore, encryptor FieldEncryptor) *SecureProjectStore {
	if encryptor == nil {
		encryptor = NewNullEncryptor()
	}
	return &SecureProjectStore{
		store:     store,
		encryptor: encryptor,
	}
}

// GetAPIKeyForProject retrieves and decrypts the API key for a project.
func (s *SecureProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	encryptedKey, err := s.store.GetAPIKeyForProject(ctx, projectID)
	if err != nil {
		return "", err
	}

	// Decrypt the API key
	decryptedKey, err := s.encryptor.Decrypt(encryptedKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return decryptedKey, nil
}

// GetProjectActive returns whether a project is active.
func (s *SecureProjectStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	return s.store.GetProjectActive(ctx, projectID)
}

// ListProjects retrieves all projects and decrypts their API keys.
func (s *SecureProjectStore) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	// Decrypt API keys for each project
	for i := range projects {
		decryptedKey, err := s.encryptor.Decrypt(projects[i].OpenAIAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt API key for project %s: %w", projects[i].ID, err)
		}
		projects[i].OpenAIAPIKey = decryptedKey
	}

	return projects, nil
}

// CreateProject encrypts the API key and creates the project.
func (s *SecureProjectStore) CreateProject(ctx context.Context, project proxy.Project) error {
	// Encrypt the API key before storing
	encryptedKey, err := s.encryptor.Encrypt(project.OpenAIAPIKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %w", err)
	}

	project.OpenAIAPIKey = encryptedKey
	return s.store.CreateProject(ctx, project)
}

// GetProjectByID retrieves a project and decrypts its API key.
func (s *SecureProjectStore) GetProjectByID(ctx context.Context, projectID string) (proxy.Project, error) {
	project, err := s.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return proxy.Project{}, err
	}

	// Decrypt the API key
	decryptedKey, err := s.encryptor.Decrypt(project.OpenAIAPIKey)
	if err != nil {
		return proxy.Project{}, fmt.Errorf("failed to decrypt API key: %w", err)
	}

	project.OpenAIAPIKey = decryptedKey
	return project, nil
}

// UpdateProject encrypts the API key and updates the project.
func (s *SecureProjectStore) UpdateProject(ctx context.Context, project proxy.Project) error {
	// Only encrypt if the API key is not already encrypted
	if !IsEncrypted(project.OpenAIAPIKey) {
		encryptedKey, err := s.encryptor.Encrypt(project.OpenAIAPIKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt API key: %w", err)
		}
		project.OpenAIAPIKey = encryptedKey
	}

	return s.store.UpdateProject(ctx, project)
}

// DeleteProject deletes a project.
func (s *SecureProjectStore) DeleteProject(ctx context.Context, projectID string) error {
	return s.store.DeleteProject(ctx, projectID)
}

// Compile-time interface check
var _ proxy.ProjectStore = (*SecureProjectStore)(nil)

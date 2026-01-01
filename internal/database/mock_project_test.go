package database

import (
	"context"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/stretchr/testify/assert"
)

func TestMockProjectStore_BasicCRUD(t *testing.T) {
	store := NewMockProjectStore()
	ctx := context.Background()
	project := proxy.Project{ID: "p1", Name: "Test Project", APIKey: "key1"}

	// CreateProject
	err := store.CreateProject(ctx, project)
	assert.NoError(t, err)

	// Duplicate CreateProject
	err = store.CreateProject(ctx, project)
	assert.Error(t, err)

	// GetProjectByID
	got, err := store.GetProjectByID(ctx, "p1")
	assert.NoError(t, err)
	assert.Equal(t, project, got)

	// GetProjectByID (not found)
	_, err = store.GetProjectByID(ctx, "notfound")
	assert.Error(t, err)

	// UpdateProject
	project.Name = "Updated Name"
	err = store.UpdateProject(ctx, project)
	assert.NoError(t, err)
	got, _ = store.GetProjectByID(ctx, "p1")
	assert.Equal(t, "Updated Name", got.Name)

	// UpdateProject (not found)
	err = store.UpdateProject(ctx, proxy.Project{ID: "notfound"})
	assert.Error(t, err)

	// DeleteProject
	err = store.DeleteProject(ctx, "p1")
	assert.NoError(t, err)

	// DeleteProject (not found)
	err = store.DeleteProject(ctx, "p1")
	assert.Error(t, err)
}

func TestMockProjectStore_ListProjects(t *testing.T) {
	store := NewMockProjectStore()
	ctx := context.Background()
	// Empty list
	projects, err := store.ListProjects(ctx)
	assert.NoError(t, err)
	assert.Len(t, projects, 0)
	// Add projects
	err = store.CreateProject(ctx, proxy.Project{ID: "p1", Name: "A", APIKey: "k1"})
	assert.NoError(t, err)
	err = store.CreateProject(ctx, proxy.Project{ID: "p2", Name: "B", APIKey: "k2"})
	assert.NoError(t, err)
	projects, err = store.ListProjects(ctx)
	assert.NoError(t, err)
	assert.Len(t, projects, 2)
}

func TestMockProjectStore_CreateMockProject(t *testing.T) {
	store := NewMockProjectStore()
	// Empty fields
	_, err := store.CreateMockProject("", "n", "k")
	assert.Error(t, err)
	_, err = store.CreateMockProject("id", "", "k")
	assert.Error(t, err)
	_, err = store.CreateMockProject("id", "n", "")
	assert.Error(t, err)
	// Success
	p, err := store.CreateMockProject("id", "n", "k")
	assert.NoError(t, err)
	assert.Equal(t, "id", p.ID)
	assert.Equal(t, "n", p.Name)
	assert.Equal(t, "k", p.APIKey)
}

func TestMockProjectStore_GetAPIKeyForProject(t *testing.T) {
	store := NewMockProjectStore()
	ctx := context.Background()
	err := store.CreateProject(ctx, proxy.Project{ID: "p1", Name: "N", APIKey: "k1"})
	assert.NoError(t, err)
	key, err := store.GetAPIKeyForProject(ctx, "p1")
	assert.NoError(t, err)
	assert.Equal(t, "k1", key)
	_, err = store.GetAPIKeyForProject(ctx, "notfound")
	assert.Error(t, err)
}

func TestMockProjectStore_GetProjectActive(t *testing.T) {
	store := NewMockProjectStore()
	ctx := context.Background()

	// Create an active project
	err := store.CreateProject(ctx, proxy.Project{ID: "p1", Name: "Active Project", APIKey: "k1", IsActive: true})
	assert.NoError(t, err)

	// Create an inactive project
	err = store.CreateProject(ctx, proxy.Project{ID: "p2", Name: "Inactive Project", APIKey: "k2", IsActive: false})
	assert.NoError(t, err)

	// Test GetProjectActive for active project
	active, err := store.GetProjectActive(ctx, "p1")
	assert.NoError(t, err)
	assert.True(t, active)

	// Test GetProjectActive for inactive project
	active, err = store.GetProjectActive(ctx, "p2")
	assert.NoError(t, err)
	assert.False(t, active)

	// Test GetProjectActive for non-existent project
	_, err = store.GetProjectActive(ctx, "notfound")
	assert.Error(t, err)
}

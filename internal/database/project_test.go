package database

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

// TestProjectCRUD tests project CRUD operations.
func TestProjectCRUD(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project
	project := proxy.Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		OpenAIAPIKey: "test-api-key",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}

	// Test CreateProject
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test GetProjectByID
	retrievedProject, err := db.GetProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get project by ID: %v", err)
	}
	if retrievedProject.ID != project.ID {
		t.Fatalf("Expected project ID %s, got %s", project.ID, retrievedProject.ID)
	}
	if retrievedProject.Name != project.Name {
		t.Fatalf("Expected project name %s, got %s", project.Name, retrievedProject.Name)
	}
	if retrievedProject.OpenAIAPIKey != project.OpenAIAPIKey {
		t.Fatalf("Expected project API key %s, got %s", project.OpenAIAPIKey, retrievedProject.OpenAIAPIKey)
	}

	// Test GetProjectByName
	retrievedByName, err := db.GetProjectByName(ctx, project.Name)
	if err != nil {
		t.Fatalf("Failed to get project by name: %v", err)
	}
	if retrievedByName.ID != project.ID {
		t.Fatalf("Expected project ID %s, got %s", project.ID, retrievedByName.ID)
	}

	// Test GetProjectByID with non-existent ID
	_, err = db.GetProjectByID(ctx, "non-existent")
	if err != ErrProjectNotFound {
		t.Fatalf("Expected ErrProjectNotFound, got %v", err)
	}

	// Test UpdateProject
	updatedProject := retrievedProject
	updatedProject.Name = "Updated Project"
	updatedProject.OpenAIAPIKey = "updated-api-key"

	err = db.UpdateProject(ctx, updatedProject)
	if err != nil {
		t.Fatalf("Failed to update project: %v", err)
	}

	retrievedAfterUpdate, err := db.GetProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get project after update: %v", err)
	}
	if retrievedAfterUpdate.Name != updatedProject.Name {
		t.Fatalf("Expected updated name %s, got %s", updatedProject.Name, retrievedAfterUpdate.Name)
	}
	if retrievedAfterUpdate.OpenAIAPIKey != updatedProject.OpenAIAPIKey {
		t.Fatalf("Expected updated API key %s, got %s", updatedProject.OpenAIAPIKey, retrievedAfterUpdate.OpenAIAPIKey)
	}

	// Test UpdateProject with non-existent ID
	nonExistentProject := proxy.Project{
		ID:           "non-existent",
		Name:         "Non-existent Project",
		OpenAIAPIKey: "test-api-key",
		UpdatedAt:    time.Now(),
	}
	err = db.UpdateProject(ctx, nonExistentProject)
	if err != ErrProjectNotFound {
		t.Fatalf("Expected ErrProjectNotFound, got %v", err)
	}

	// Create a second project for ListProjects test
	project2 := proxy.Project{
		ID:           "test-project-id-2",
		Name:         "Test Project 2",
		OpenAIAPIKey: "test-api-key-2",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	err = db.CreateProject(ctx, project2)
	if err != nil {
		t.Fatalf("Failed to create second project: %v", err)
	}

	// Test ListProjects
	projects, err := db.ListProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("Expected 2 projects, got %d", len(projects))
	}

	// Test DeleteProject
	err = db.DeleteProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	// Verify project was deleted
	_, err = db.GetProjectByID(ctx, project.ID)
	if err != ErrProjectNotFound {
		t.Fatalf("Expected ErrProjectNotFound after deletion, got %v", err)
	}

	// Test DeleteProject with non-existent ID
	err = db.DeleteProject(ctx, "non-existent")
	if err != ErrProjectNotFound {
		t.Fatalf("Expected ErrProjectNotFound, got %v", err)
	}
}

func TestProjectCRUD_Errors(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()

	project := proxy.Project{
		ID:           "dup-id",
		Name:         "Dup Project",
		OpenAIAPIKey: "key",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.CreateProject(ctx, project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	// Duplicate ID
	dup := project
	dup.Name = "Other Name"
	if err := db.CreateProject(ctx, dup); err == nil {
		t.Error("expected error for duplicate project ID")
	}
	// Duplicate Name
	project2 := proxy.Project{
		ID:           "other-id",
		Name:         project.Name,
		OpenAIAPIKey: "key2",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.CreateProject(ctx, project2); err == nil {
		t.Error("expected error for duplicate project Name")
	}
	// ListProjects on empty DB
	db2, cleanup2 := testDB(t)
	defer cleanup2()
	projects, err := db2.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
	// Update non-existent project
	p := proxy.Project{ID: "nope", Name: "nope", OpenAIAPIKey: "k", UpdatedAt: time.Now()}
	if err := db.UpdateProject(ctx, p); err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
	// Delete non-existent project
	if err := db.DeleteProject(ctx, "nope"); err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestGetProjectByName_NotFound(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	_, err := db.GetProjectByName(ctx, "does-not-exist")
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestListProjects_Multiple(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		p := proxy.Project{
			ID:           "id-" + strconv.Itoa(i),
			Name:         "Project-" + strconv.Itoa(i),
			OpenAIAPIKey: "key",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := db.CreateProject(ctx, p); err != nil {
			t.Fatalf("Failed to create project: %v", err)
		}
	}
	projects, err := db.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 5 {
		t.Errorf("expected 5 projects, got %d", len(projects))
	}
}

func TestUpdateProject_InvalidInput(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	p := proxy.Project{ID: "", Name: "", OpenAIAPIKey: "", UpdatedAt: time.Now()}
	if err := db.UpdateProject(ctx, p); err == nil {
		t.Error("expected error for empty ID in UpdateProject")
	}
}

func TestDeleteProject_InvalidInput(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	if err := db.DeleteProject(ctx, ""); err == nil {
		t.Error("expected error for empty ID in DeleteProject")
	}
}

func TestListProjects_Empty(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	projects, err := db.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestGetProjectByName_EmptyName(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	_, err := db.GetProjectByName(ctx, "")
	if err == nil {
		t.Error("expected error for empty name in GetProjectByName")
	}
}

func TestUpdateProject_EmptyID(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	p := proxy.Project{ID: "", Name: "Name", OpenAIAPIKey: "key", UpdatedAt: time.Now()}
	if err := db.UpdateProject(ctx, p); err == nil {
		t.Error("expected error for empty ID in UpdateProject")
	}
}

func TestDeleteProject_EmptyID(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	if err := db.DeleteProject(ctx, ""); err == nil {
		t.Error("expected error for empty ID in DeleteProject")
	}
}

func TestListProjects_LongNames(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	longName := make([]byte, 300)
	for i := range longName {
		longName[i] = 'a'
	}
	p := proxy.Project{ID: "long", Name: string(longName), OpenAIAPIKey: "key", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = db.CreateProject(ctx, p)
	projects, err := db.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	found := false
	for _, proj := range projects {
		if proj.ID == "long" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find project with long name")
	}
}

func TestGetAPIKeyForProject_DB(t *testing.T) {
	t.Skip("Not implemented: DB.GetAPIKeyForProject is a stub. TODO: implement and enable test.")
	// db, cleanup := testDB(t)
	// defer cleanup()
	// ctx := context.Background()
	// // Insert a project
	// p := Project{ID: "p1", Name: "Test", OpenAIAPIKey: "key1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	// if err := db.CreateProject(ctx, p); err != nil {
	// 	t.Fatalf("Failed to create project: %v", err)
	// }
	// // Should succeed
	// key, err := db.GetAPIKeyForProject(ctx, "p1")
	// if err != nil || key != "key1" {
	// 	t.Errorf("expected key1, got %v, err=%v", key, err)
	// }
	// // Should fail for non-existent
	// _, err = db.GetAPIKeyForProject(ctx, "nope")
	// if err == nil {
	// 	t.Error("expected error for non-existent project")
	// }
}

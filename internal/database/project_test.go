package database

import (
	"context"
	"testing"
	"time"
)

// TestProjectCRUD tests project CRUD operations.
func TestProjectCRUD(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project
	project := Project{
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
	nonExistentProject := Project{
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
	project2 := Project{
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

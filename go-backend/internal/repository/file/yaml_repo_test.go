package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hpe-recipe/internal/model"
)

func findWorkspaceRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "catalogs", "recipe-detection")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find workspace root containing catalogs/recipe-detection")
		}
		dir = parent
	}
}

func TestYAMLRepo_ReadExistingWorkspace(t *testing.T) {
	workspaceRoot := findWorkspaceRoot(t)

	repo := NewYAMLCatalogRepository(workspaceRoot)
	ctx := context.Background()

	// 1. Check listing versions
	versions, err := repo.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions error: %v", err)
	}
	if len(versions) == 0 {
		t.Fatalf("expected versions in workspace, got 0")
	}

	// 2. Read specific version (e.g. 2.1.3)
	rel, err := repo.ReadVersion(ctx, "2.1.3")
	if err != nil {
		t.Fatalf("ReadVersion(2.1.3) error: %v", err)
	}
	if rel == nil {
		t.Fatalf("expected version 2.1.3 to be found")
	}
	if len(rel.Recipes) == 0 {
		t.Fatalf("expected recipes in 2.1.3, got 0")
	}

	// 3. Read environment version
	devVer, err := repo.ReadEnvironmentVersion(ctx, "dev")
	if err != nil {
		t.Fatalf("ReadEnvironmentVersion(dev) error: %v", err)
	}
	if devVer == "" {
		t.Fatalf("expected dev environment to have active version")
	}

	// 4. Read environment history
	hist, err := repo.ReadEnvironmentHistory(ctx, "dev")
	if err != nil {
		t.Fatalf("ReadEnvironmentHistory(dev) error: %v", err)
	}
	if len(hist) == 0 {
		t.Fatalf("expected dev environment history entries")
	}

	// 5. Read audit history
	audit, err := repo.ReadAuditHistory(ctx)
	if err != nil {
		t.Fatalf("ReadAuditHistory error: %v", err)
	}
	if len(audit) == 0 {
		t.Fatalf("expected audit history entries")
	}
}

func TestYAMLRepo_WriteAndReadVersion(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewYAMLCatalogRepository(tempDir)
	ctx := context.Background()

	release := &model.HelmRelease{
		Version:     "3.0.0",
		CatalogName: "Test Catalog",
		Recipes: []model.Recipe{
			{
				Version:     "1.0.0",
				Description: "Sample recipe",
				Components: map[string]model.ComponentSpec{
					"redis": {
						Version: "6.0",
					},
				},
			},
		},
	}

	err := repo.WriteVersion(ctx, release)
	if err != nil {
		t.Fatalf("WriteVersion error: %v", err)
	}

	readRel, err := repo.ReadVersion(ctx, "3.0.0")
	if err != nil {
		t.Fatalf("ReadVersion error: %v", err)
	}
	if readRel == nil || readRel.CatalogName != "Test Catalog" {
		t.Fatalf("expected CatalogName 'Test Catalog', got %+v", readRel)
	}
}

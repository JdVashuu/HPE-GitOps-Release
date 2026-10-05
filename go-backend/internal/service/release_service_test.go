package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hpe-recipe/internal/model"
	"hpe-recipe/internal/repository/file"
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

func TestReleaseService_CompareAndPreview(t *testing.T) {
	workspaceRoot := findWorkspaceRoot(t)

	repo := file.NewYAMLCatalogRepository(workspaceRoot)
	svc := NewReleaseService(repo, nil)
	ctx := context.Background()

	// Compare 2.1.1 to 2.1.2
	diff, err := svc.CompareVersions(ctx, "dev", "2.1.1", "2.1.2")
	if err != nil {
		t.Fatalf("CompareVersions failed: %v", err)
	}

	if diff.From != "2.1.1" || diff.To != "2.1.2" {
		t.Fatalf("unexpected diff metadata: %+v", diff)
	}

	// Deploy preview for 2.1.3
	preview, err := svc.GetDeployPreview(ctx, "dev", "2.1.3", "auto")
	if err != nil {
		t.Fatalf("GetDeployPreview failed: %v", err)
	}
	if preview.TargetVersion != "2.1.3" {
		t.Fatalf("expected targetVersion 2.1.3, got %s", preview.TargetVersion)
	}
}

func TestReleaseService_ValidateComponentCompatibility(t *testing.T) {
	svc := &releaseService{}

	validRelease := &model.HelmRelease{
		Recipes: []model.Recipe{
			{
				Version: "1.0.0",
				Components: map[string]model.ComponentSpec{
					"db": {
						Version:   "1.0",
						UpgradeTo: []string{"2.0"},
					},
				},
				UpgradeTo: []string{"2.0.0"},
			},
			{
				Version: "2.0.0",
				Components: map[string]model.ComponentSpec{
					"db": {
						Version:     "2.0",
						UpgradeFrom: []string{"1.0"},
					},
				},
				UpgradeFrom: []string{"1.0.0"},
			},
		},
	}

	if err := svc.ValidateComponentCompatibility(validRelease); err != nil {
		t.Fatalf("expected validRelease to pass compatibility check, got: %v", err)
	}

	invalidRelease := &model.HelmRelease{
		Recipes: []model.Recipe{
			{
				Version: "1.0.0",
				Components: map[string]model.ComponentSpec{
					"db": {
						Version:   "1.0",
						UpgradeTo: []string{"2.0"},
					},
				},
				UpgradeTo: []string{"2.0.0"},
			},
			{
				Version: "2.0.0",
				Components: map[string]model.ComponentSpec{
					"db": {
						Version:     "3.0", // db skipped from 1.0 to 3.0 which is not allowed by 1.0 UpgradeTo
						UpgradeFrom: []string{"2.0"},
					},
				},
				UpgradeFrom: []string{"1.0.0"},
			},
		},
	}

	if err := svc.ValidateComponentCompatibility(invalidRelease); err == nil {
		t.Fatalf("expected invalidRelease to fail compatibility check, but passed")
	}
}

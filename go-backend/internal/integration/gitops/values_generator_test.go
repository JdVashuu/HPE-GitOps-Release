package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hpe-recipe/internal/model"
)

func TestGenerateValuesYAML(t *testing.T) {
	rel := &model.HelmRelease{
		Version:            "2.1.4",
		Cluster:            "dev",
		CatalogName:        "Test Analytics",
		CatalogDescription: "Test Description",
		CatalogStatus:      "GA",
		Maintainer:         "HPE Team",
		Recipes: []model.Recipe{
			{
				Version:     "1.0.0",
				Description: "Initial test recipe",
				Components: map[string]model.ComponentSpec{
					"db": {
						Version:     "14.0",
						UpgradeFrom: []string{"13.0"},
						UpgradeTo:   []string{"15.0"},
					},
				},
				UpgradeTo: []string{"1.0.1"},
			},
		},
	}

	valuesFileName := ResolveValuesFileName(rel)
	if valuesFileName != "values-v2.1.4.yaml" {
		t.Fatalf("expected values-v2.1.4.yaml, got %s", valuesFileName)
	}

	yamlContent, err := GenerateValuesYAML(rel, valuesFileName)
	if err != nil {
		t.Fatalf("GenerateValuesYAML error: %v", err)
	}

	if !strings.Contains(yamlContent, "chartVersion: 2.1.4") {
		t.Errorf("expected chartVersion: 2.1.4 in yaml")
	}
	if !strings.Contains(yamlContent, "target_cluster: dev") {
		t.Errorf("expected target_cluster: dev in yaml")
	}
	if !strings.Contains(yamlContent, "values_file: values-v2.1.4.yaml") {
		t.Errorf("expected values_file: values-v2.1.4.yaml in yaml")
	}
}

func TestUpdateChartMetadata(t *testing.T) {
	tempDir := t.TempDir()
	chartPath := filepath.Join(tempDir, "Chart.yaml")

	initial := `apiVersion: v2
name: recipe-detection-chart
description: Helm chart
type: application
version: 2.1.3
appVersion: "2.1.3"
annotations:
  recipe-detection/values-file: values-v2.1.3.yaml
`
	if err := os.WriteFile(chartPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial Chart.yaml failed: %v", err)
	}

	err := UpdateChartMetadata(chartPath, "2.1.4", "values-v2.1.4.yaml")
	if err != nil {
		t.Fatalf("UpdateChartMetadata failed: %v", err)
	}

	updated, err := os.ReadFile(chartPath)
	if err != nil {
		t.Fatalf("read updated Chart.yaml failed: %v", err)
	}

	s := string(updated)
	if !strings.Contains(s, "version: 2.1.4") {
		t.Errorf("expected version: 2.1.4 in %s", s)
	}
	if !strings.Contains(s, `appVersion: "2.1.4"`) {
		t.Errorf(`expected appVersion: "2.1.4" in %s`, s)
	}
	if !strings.Contains(s, "recipe-detection/values-file: values-v2.1.4.yaml") {
		t.Errorf("expected values-file annotation updated in %s", s)
	}
}

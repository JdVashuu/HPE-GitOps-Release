package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"hpe-recipe/internal/config"
	"hpe-recipe/internal/integration/websocket"
	"hpe-recipe/internal/model"
	"hpe-recipe/internal/repository/file"
	"hpe-recipe/internal/service"
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

func setupTestServer(t *testing.T) http.Handler {
	workspaceRoot := findWorkspaceRoot(t)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8081,
			ContextPath: "/api",
		},
		Promotion: config.PromotionConfig{
			Pipeline: []string{"dev", "qa", "integration", "prod"},
		},
	}

	repo := file.NewYAMLCatalogRepository(workspaceRoot)
	hub := websocket.NewHub()
	events := service.NewEventService(hub)
	platformSvc := service.NewPlatformService(cfg, repo, nil, nil, events)
	releaseSvc := service.NewReleaseService(repo, nil)

	catalogCtrl := NewCatalogController(platformSvc, events)
	releaseCtrl := NewReleaseController(releaseSvc, platformSvc, events)
	recipeCtrl := NewRecipeController(platformSvc, releaseSvc)
	healthCtrl := NewHealthController()
	wsCtrl := NewWSController(hub)

	return NewRouter(RouterParams{
		Catalog: catalogCtrl,
		Release: releaseCtrl,
		Recipe:  recipeCtrl,
		Health:  healthCtrl,
		WS:      wsCtrl,
	})
}

func TestController_Pipeline(t *testing.T) {
	handler := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pipeline", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var pipeline []string
	if err := json.Unmarshal(rec.Body.Bytes(), &pipeline); err != nil {
		t.Fatalf("decode pipeline error: %v", err)
	}

	if len(pipeline) != 4 || pipeline[0] != "dev" || pipeline[3] != "prod" {
		t.Fatalf("unexpected pipeline: %v", pipeline)
	}
}

func TestController_Health(t *testing.T) {
	handler := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health error: %v", err)
	}
	if body["status"] != "UP" {
		t.Fatalf("expected UP, got %v", body["status"])
	}
}

func TestController_EnvironmentsAndVersions(t *testing.T) {
	handler := setupTestServer(t)

	// Environments
	reqEnv := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	recEnv := httptest.NewRecorder()
	handler.ServeHTTP(recEnv, reqEnv)
	if recEnv.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recEnv.Code)
	}

	// Versions
	reqVer := httptest.NewRequest(http.MethodGet, "/api/versions", nil)
	recVer := httptest.NewRecorder()
	handler.ServeHTTP(recVer, reqVer)
	if recVer.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recVer.Code)
	}

	// Specific Version
	reqV := httptest.NewRequest(http.MethodGet, "/api/versions/2.1.3", nil)
	recV := httptest.NewRecorder()
	handler.ServeHTTP(recV, reqV)
	if recV.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recV.Code)
	}

	var rel model.HelmRelease
	if err := json.Unmarshal(recV.Body.Bytes(), &rel); err != nil {
		t.Fatalf("decode version error: %v", err)
	}
	if rel.Version != "2.1.3" {
		t.Fatalf("expected version 2.1.3, got %s", rel.Version)
	}

	// Promotion Options
	reqPromo := httptest.NewRequest(http.MethodGet, "/api/versions/2.1.3/promotion-options", nil)
	recPromo := httptest.NewRecorder()
	handler.ServeHTTP(recPromo, reqPromo)
	if recPromo.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recPromo.Code)
	}
}

func TestController_HelmReleases(t *testing.T) {
	handler := setupTestServer(t)

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/helm-releases?cluster=dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Specific release
	reqRel := httptest.NewRequest(http.MethodGet, "/api/helm-releases/2.1.3?cluster=dev", nil)
	recRel := httptest.NewRecorder()
	handler.ServeHTTP(recRel, reqRel)
	if recRel.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recRel.Code)
	}

	// Deploy Preview
	reqPrev := httptest.NewRequest(http.MethodGet, "/api/helm-releases/2.1.3/deploy-preview?cluster=dev&baseline=auto", nil)
	recPrev := httptest.NewRecorder()
	handler.ServeHTTP(recPrev, reqPrev)
	if recPrev.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recPrev.Code)
	}
}

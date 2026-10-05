package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hpe-recipe/internal/config"
	"hpe-recipe/internal/controller"
	"hpe-recipe/internal/integration/gitops"
	"hpe-recipe/internal/integration/jenkins"
	"hpe-recipe/internal/integration/websocket"
	"hpe-recipe/internal/repository/cache"
	"hpe-recipe/internal/repository/file"
	"hpe-recipe/internal/service"
)

// App manages the lifecycle of the HPE Recipe Detection Go backend.
type App struct {
	cfg    *config.Config
	server *http.Server
	hub    *websocket.Hub
}

// New initializes the application and wires all layers together.
func New(cfg *config.Config) (*App, error) {
	// 1. In-memory cache & Workspace Lock Manager
	memCache := cache.NewMemoryCache()

	// 2. GitOps Integration Client
	gitOps := gitops.NewClient(
		cfg.GitOps.RepoURL,
		cfg.GitOps.LocalPath,
		cfg.GitOps.Branch,
		cfg.GitOps.Username,
		cfg.GitOps.Token,
		cfg.GitOps.ValuesDir,
		cfg.GitOps.StateCacheTTL,
		memCache,
	)

	// 3. File Repository (uses GitOps local workspace directory)
	repo := file.NewYAMLCatalogRepository(cfg.GitOps.LocalPath)

	// 4. Jenkins Integration Client
	jenkinsClient := jenkins.NewClient(
		cfg.Jenkins.URL,
		cfg.Jenkins.Job,
		cfg.Jenkins.Username,
		cfg.Jenkins.Token,
	)

	// 5. WebSocket Hub & Event Service
	hub := websocket.NewHub()
	events := service.NewEventService(hub)

	// 6. Business Services
	platformSvc := service.NewPlatformService(cfg, repo, gitOps, jenkinsClient, events)
	releaseSvc := service.NewReleaseService(repo, gitOps)

	// 7. Controllers
	catalogCtrl := controller.NewCatalogController(platformSvc, events)
	releaseCtrl := controller.NewReleaseController(releaseSvc, platformSvc, events)
	recipeCtrl := controller.NewRecipeController(platformSvc, releaseSvc)
	healthCtrl := controller.NewHealthController()
	wsCtrl := controller.NewWSController(hub)

	// 8. Router with net/http ServeMux
	handler := controller.NewRouter(controller.RouterParams{
		Catalog: catalogCtrl,
		Release: releaseCtrl,
		Recipe:  recipeCtrl,
		Health:  healthCtrl,
		WS:      wsCtrl,
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &App{
		cfg:    cfg,
		server: server,
		hub:    hub,
	}, nil
}

// Run starts background workers and the HTTP listener, listening for termination signals.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start WebSocket Hub loop
	go a.hub.Run(ctx)

	// Start HTTP server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		slog.Info("Starting HPE Recipe Detection server", "addr", a.server.Addr, "port", a.cfg.Server.Port)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Listen for shutdown signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-stopChan:
		slog.Info("Received shutdown signal", "signal", sig.String())
	case <-ctx.Done():
		slog.Info("Context cancelled, shutting down")
	}

	// Graceful shutdown with 10s timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Graceful shutdown failed", "err", err)
		return a.server.Close()
	}

	slog.Info("Server stopped cleanly")
	return nil
}

package controller

import (
	"net/http"
	"strings"

	"hpe-recipe/internal/middleware"
)

// RouterParams contains all controllers needed to assemble HTTP routes.
type RouterParams struct {
	Catalog *CatalogController
	Release *ReleaseController
	Recipe  *RecipeController
	Health  *HealthController
	WS      *WSController
}

// NewRouter registers all application routes on a standard library net/http ServeMux with middleware.
func NewRouter(params RouterParams) http.Handler {
	mux := http.NewServeMux()

	registerRoutes := func(prefix string) {
		p := strings.TrimRight(prefix, "/")

		// Platform / Catalog
		mux.HandleFunc("GET "+p+"/pipeline", params.Catalog.Pipeline)
		mux.HandleFunc("GET "+p+"/environments", params.Catalog.Environments)
		mux.HandleFunc("GET "+p+"/versions", params.Catalog.Versions)
		mux.HandleFunc("GET "+p+"/versions/{version}", params.Catalog.Version)
		mux.HandleFunc("GET "+p+"/versions/{version}/promotion-options", params.Catalog.PromotionOptions)
		mux.HandleFunc("POST "+p+"/versions", params.Catalog.Create)
		mux.HandleFunc("POST "+p+"/versions/{version}/deploy", params.Catalog.DeployToDev)
		mux.HandleFunc("POST "+p+"/versions/{version}/promote", params.Catalog.Promote)
		mux.HandleFunc("POST "+p+"/environments/{env}/rollback", params.Catalog.Rollback)
		mux.HandleFunc("DELETE "+p+"/versions/{version}", params.Catalog.DeleteVersion)
		mux.HandleFunc("POST "+p+"/catalog/edit", params.Catalog.EditDev)
		mux.HandleFunc("GET "+p+"/history", params.Catalog.History)
		mux.HandleFunc("DELETE "+p+"/history", params.Catalog.ClearHistory)

		// Helm Releases
		mux.HandleFunc("GET "+p+"/helm-releases", params.Release.GetAllHelmReleases)
		mux.HandleFunc("GET "+p+"/helm-releases/compare", params.Release.CompareHelmVersions)
		mux.HandleFunc("GET "+p+"/helm-releases/{version}", params.Release.GetHelmRelease)
		mux.HandleFunc("POST "+p+"/helm-releases", params.Release.CreateHelmRelease)
		mux.HandleFunc("PUT "+p+"/helm-releases/{version}", params.Release.UpdateHelmRelease)
		mux.HandleFunc("PUT "+p+"/helm-releases/{version}/status", params.Release.UpdateStatus)
		mux.HandleFunc("POST "+p+"/helm-releases/{version}/deploy", params.Release.DeployRelease)
		mux.HandleFunc("DELETE "+p+"/helm-releases/{version}", params.Release.DeleteHelmRelease)
		mux.HandleFunc("GET "+p+"/helm-releases/{version}/recipes", params.Release.GetRecipes)
		mux.HandleFunc("POST "+p+"/helm-releases/{version}/recipes", params.Release.AddRecipe)
		mux.HandleFunc("PUT "+p+"/helm-releases/{version}/recipes/{recipeVersion}", params.Release.UpdateRecipe)
		mux.HandleFunc("DELETE "+p+"/helm-releases/{version}/recipes/{recipeVersion}", params.Release.DeleteRecipe)
		mux.HandleFunc("GET "+p+"/helm-releases/{version}/recipes/{recipeVersion}/components", params.Release.GetComponents)
		mux.HandleFunc("GET "+p+"/helm-releases/{version}/recipes/{recipeVersion}/upgradePaths", params.Release.GetUpgradePaths)
		mux.HandleFunc("GET "+p+"/helm-releases/{version}/deploy-preview", params.Release.DeployPreview)
		mux.HandleFunc("GET "+p+"/helm-releases/{version}/promotion-options", params.Release.PromotionOptions)

		// Recipe sub-resources
		mux.HandleFunc("GET "+p+"/recipes/{recipeVersion}/components", params.Recipe.GetComponents)
		mux.HandleFunc("GET "+p+"/recipes/{recipeVersion}/upgradePaths", params.Recipe.GetUpgradePaths)

		// Health & Actuator
		mux.HandleFunc("GET "+p+"/health", params.Health.Health)
		mux.HandleFunc("GET "+p+"/actuator/health", params.Health.ActuatorHealth)

		// WebSocket
		mux.HandleFunc("GET "+p+"/ws/releases", params.WS.ServeWS)
	}

	// Register with /api prefix (matching application.yml context-path) and without prefix for direct routing
	registerRoutes("/api")
	registerRoutes("")

	// Apply middleware stack: Recovery -> Logging -> CORS
	handler := middleware.Recovery(mux)
	handler = middleware.Logger(handler)
	handler = middleware.CORS(handler)

	return handler
}

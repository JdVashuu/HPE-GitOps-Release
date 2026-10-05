package controller

import (
	"net/http"

	"hpe-recipe/internal/model"
	"hpe-recipe/internal/service"
)

type RecipeController struct {
	platform service.PlatformService
	releases service.ReleaseService
}

func NewRecipeController(platform service.PlatformService, releases service.ReleaseService) *RecipeController {
	return &RecipeController{
		platform: platform,
		releases: releases,
	}
}

func (c *RecipeController) GetComponents(w http.ResponseWriter, r *http.Request) {
	recipeVersion := pathParam(r, "recipeVersion")
	dev := c.platform.GetPipeline()[0]

	activeDev, err := c.platform.GetEnvironments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	devVersion := activeDev[dev]
	if devVersion == "" {
		writeJSON(w, http.StatusOK, map[string]model.ComponentSpec{})
		return
	}

	comps, err := c.releases.GetComponents(r.Context(), dev, devVersion, recipeVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comps)
}

func (c *RecipeController) GetUpgradePaths(w http.ResponseWriter, r *http.Request) {
	recipeVersion := pathParam(r, "recipeVersion")
	dev := c.platform.GetPipeline()[0]

	activeDev, err := c.platform.GetEnvironments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	devVersion := activeDev[dev]
	if devVersion == "" {
		writeJSON(w, http.StatusOK, []string{})
		return
	}

	paths, err := c.releases.GetUpgradePaths(r.Context(), dev, devVersion, recipeVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, paths)
}

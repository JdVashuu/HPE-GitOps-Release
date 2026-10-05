package controller

import (
	"net/http"
	"strings"

	"hpe-recipe/internal/model"
	"hpe-recipe/internal/service"
)

type ReleaseController struct {
	releases service.ReleaseService
	platform service.PlatformService
	events   service.EventService
}

func NewReleaseController(
	releases service.ReleaseService,
	platform service.PlatformService,
	events service.EventService,
) *ReleaseController {
	return &ReleaseController{
		releases: releases,
		platform: platform,
		events:   events,
	}
}

func (c *ReleaseController) GetAllHelmReleases(w http.ResponseWriter, r *http.Request) {
	cluster := queryParam(r, "cluster", "")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'cluster' is required")
		return
	}

	summaries, err := c.releases.GetAllReleases(r.Context(), cluster)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (c *ReleaseController) GetHelmRelease(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'cluster' is required")
		return
	}

	rel, err := c.releases.GetRelease(r.Context(), cluster, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rel == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

func (c *ReleaseController) CreateHelmRelease(w http.ResponseWriter, r *http.Request) {
	cluster := queryParam(r, "cluster", "")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'cluster' is required")
		return
	}

	var release model.HelmRelease
	if err := readJSON(r, &release); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	created, err := c.releases.CreateRelease(r.Context(), cluster, &release)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastReleaseCreated(created)
	writeJSON(w, http.StatusCreated, created)
}

func (c *ReleaseController) UpdateHelmRelease(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'cluster' is required")
		return
	}

	var release model.HelmRelease
	if err := readJSON(r, &release); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	updated, err := c.releases.UpdateRelease(r.Context(), cluster, version, &release)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastReleaseUpdated(updated)
	writeJSON(w, http.StatusOK, updated)
}

func (c *ReleaseController) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'cluster' is required")
		return
	}

	var body map[string]string
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	status := body["status"]
	if status == "" {
		writeError(w, http.StatusBadRequest, "'status' field is required")
		return
	}

	if strings.EqualFold(status, "deployed") {
		eventAction := body["eventAction"]
		if eventAction == "" {
			eventAction = body["action"]
		}
		if err := c.platform.CompleteDeployment(r.Context(), version, cluster, eventAction, body["fromVersion"]); err != nil {
			writeError(w, http.StatusInternalServerError, "Complete deployment failed: "+err.Error())
			return
		}
	}

	c.events.BroadcastStatusChange(version, status, cluster)
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version,
		"status":  status,
		"cluster": cluster,
	})
}

func (c *ReleaseController) DeployRelease(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'cluster' is required")
		return
	}

	pipeline := c.platform.GetPipeline()
	var err error
	if len(pipeline) > 0 && cluster == pipeline[0] {
		err = c.platform.DeployToDev(r.Context(), version)
	} else {
		err = c.platform.Promote(r.Context(), version, cluster)
	}

	if err != nil {
		c.events.BroadcastStatusChange(version, "push_failed", cluster)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastStatusChange(version, "deploying", cluster)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Deployment triggered. Git environment state will update after Jenkins succeeds.",
		"version": version,
		"cluster": cluster,
	})
}

func (c *ReleaseController) DeleteHelmRelease(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	if err := c.platform.DeleteVersion(r.Context(), version); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c.events.BroadcastReleaseDeleted(version)
	w.WriteHeader(http.StatusNoContent)
}

func (c *ReleaseController) GetRecipes(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	recipes, err := c.releases.GetRecipes(r.Context(), cluster, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recipes)
}

func (c *ReleaseController) AddRecipe(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	var recipe model.Recipe
	if err := readJSON(r, &recipe); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	added, err := c.releases.AddRecipe(r.Context(), cluster, version, &recipe)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastRecipeAdded(version, cluster, added)
	writeJSON(w, http.StatusCreated, added)
}

func (c *ReleaseController) UpdateRecipe(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	recipeVersion := pathParam(r, "recipeVersion")
	cluster := queryParam(r, "cluster", "")
	var recipe model.Recipe
	if err := readJSON(r, &recipe); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	updated, err := c.releases.UpdateRecipe(r.Context(), cluster, version, recipeVersion, &recipe)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastRecipeUpdated(version, cluster, updated)
	writeJSON(w, http.StatusOK, updated)
}

func (c *ReleaseController) DeleteRecipe(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	recipeVersion := pathParam(r, "recipeVersion")
	cluster := queryParam(r, "cluster", "")

	deleted, err := c.releases.DeleteRecipe(r.Context(), cluster, version, recipeVersion)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !deleted {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	c.events.BroadcastRecipeDeleted(version, cluster, recipeVersion)
	w.WriteHeader(http.StatusNoContent)
}

func (c *ReleaseController) GetComponents(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	recipeVersion := pathParam(r, "recipeVersion")
	cluster := queryParam(r, "cluster", "")

	comps, err := c.releases.GetComponents(r.Context(), cluster, version, recipeVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comps)
}

func (c *ReleaseController) GetUpgradePaths(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	recipeVersion := pathParam(r, "recipeVersion")
	cluster := queryParam(r, "cluster", "")

	paths, err := c.releases.GetUpgradePaths(r.Context(), cluster, version, recipeVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, paths)
}

func (c *ReleaseController) CompareHelmVersions(w http.ResponseWriter, r *http.Request) {
	cluster := queryParam(r, "cluster", "")
	from := queryParam(r, "from", "")
	to := queryParam(r, "to", "")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "Query parameters 'from' and 'to' are required")
		return
	}

	diff, err := c.releases.CompareVersions(r.Context(), cluster, from, to)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (c *ReleaseController) DeployPreview(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	cluster := queryParam(r, "cluster", "")
	baseline := queryParam(r, "baseline", "auto")

	preview, err := c.releases.GetDeployPreview(r.Context(), cluster, version, baseline)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (c *ReleaseController) PromotionOptions(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	opts, err := c.platform.GetPromotionOptions(r.Context(), version)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, opts)
}

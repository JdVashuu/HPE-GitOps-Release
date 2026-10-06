package controller

import (
	"net/http"
	"strings"

	"hpe-recipe/internal/model"
	"hpe-recipe/internal/service"
)

type CatalogController struct {
	platform service.PlatformService
	events   service.EventService
}

func NewCatalogController(platform service.PlatformService, events service.EventService) *CatalogController {
	return &CatalogController{
		platform: platform,
		events:   events,
	}
}

func (c *CatalogController) Pipeline(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, c.platform.GetPipeline())
}

func (c *CatalogController) Environments(w http.ResponseWriter, r *http.Request) {
	envs, err := c.platform.GetEnvironments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, envs)
}

func (c *CatalogController) Versions(w http.ResponseWriter, r *http.Request) {
	versions, err := c.platform.GetVersions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func (c *CatalogController) Version(w http.ResponseWriter, r *http.Request) {
	v := pathParam(r, "version")
	release, err := c.platform.GetVersion(r.Context(), v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if release == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, release)
}

func (c *CatalogController) PromotionOptions(w http.ResponseWriter, r *http.Request) {
	v := pathParam(r, "version")
	opts, err := c.platform.GetPromotionOptions(r.Context(), v)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, opts)
}

func (c *CatalogController) History(w http.ResponseWriter, r *http.Request) {
	history, err := c.platform.GetHistory(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (c *CatalogController) ClearHistory(w http.ResponseWriter, r *http.Request) {
	if err := c.platform.ClearHistory(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Deployment history cleared",
	})
}

func (c *CatalogController) Create(w http.ResponseWriter, r *http.Request) {
	deployToDev := strings.EqualFold(queryParam(r, "deployToDev", "false"), "true")

	var release model.HelmRelease
	if err := readJSON(r, &release); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	var created *model.HelmRelease
	var err error

	if deployToDev {
		created, err = c.platform.CreateAndDeployToDev(r.Context(), &release)
	} else {
		created, err = c.platform.CreateVersion(r.Context(), &release)
	}

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastVersionCreated(created.Version)
	if deployToDev {
		dev := c.platform.GetPipeline()[0]
		c.events.BroadcastStatusChange(created.Version, "deploying", dev)
	}

	writeJSON(w, http.StatusCreated, created)
}

func (c *CatalogController) DeployToDev(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	dev := c.platform.GetPipeline()[0]

	if err := c.platform.DeployToDev(r.Context(), version); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastStatusChange(version, "deploying", dev)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Deploying " + version + " to " + dev,
		"version": version,
		"env":     dev,
	})
}

func (c *CatalogController) Promote(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	toEnv := queryParam(r, "to", "")
	if toEnv == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'to' is required")
		return
	}

	if err := c.platform.Promote(r.Context(), version, toEnv); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastStatusChange(version, "deploying", toEnv)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Promoting " + version + " to " + toEnv,
		"version": version,
		"env":     toEnv,
	})
}

func (c *CatalogController) Rollback(w http.ResponseWriter, r *http.Request) {
	env := pathParam(r, "env")
	targetVersion, err := c.platform.Rollback(r.Context(), env)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastStatusChange(targetVersion, "deploying", env)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Rolling back " + env,
		"version": targetVersion,
		"env":     env,
	})
}

func (c *CatalogController) DeleteVersion(w http.ResponseWriter, r *http.Request) {
	version := pathParam(r, "version")
	if err := c.platform.DeleteVersion(r.Context(), version); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c.events.BroadcastReleaseDeleted(version)
	w.WriteHeader(http.StatusNoContent)
}

func (c *CatalogController) EditDev(w http.ResponseWriter, r *http.Request) {
	var edited model.HelmRelease
	if err := readJSON(r, &edited); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	forked, err := c.platform.EditDev(r.Context(), &edited)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dev := c.platform.GetPipeline()[0]
	c.events.BroadcastVersionCreated(forked.Version)
	c.events.BroadcastStatusChange(forked.Version, "deploying", dev)

	writeJSON(w, http.StatusCreated, forked)
}

func (c *CatalogController) GetAllCatalogs(w http.ResponseWriter, r *http.Request) {
	cluster := queryParam(r, "cluster", "dev")

	envs, err := c.platform.GetEnvironments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	version := envs[cluster]
	if version == "" {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	rel, err := c.platform.GetVersion(r.Context(), version)
	if err != nil || rel == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	rel.Cluster = cluster
	writeJSON(w, http.StatusOK, []*model.HelmRelease{rel})
}

func (c *CatalogController) GetCatalogRecipes(w http.ResponseWriter, r *http.Request) {
	catalogVersion := pathParam(r, "catalogVersion")
	rel, err := c.platform.GetVersion(r.Context(), catalogVersion)
	if err != nil || rel == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, rel.Recipes)
}

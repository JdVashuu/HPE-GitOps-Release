package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"hpe-recipe/internal/integration/gitops"
	"hpe-recipe/internal/model"
	"hpe-recipe/internal/repository"
)

// ReleaseService defines inspection, diffing, and recipe management operations.
type ReleaseService interface {
	GetAllReleases(ctx context.Context, cluster string) ([]model.ReleaseSummary, error)
	GetRelease(ctx context.Context, cluster, version string) (*model.HelmRelease, error)
	CreateRelease(ctx context.Context, cluster string, release *model.HelmRelease) (*model.HelmRelease, error)
	UpdateRelease(ctx context.Context, cluster, version string, release *model.HelmRelease) (*model.HelmRelease, error)
	GetRecipes(ctx context.Context, cluster, version string) ([]model.Recipe, error)
	AddRecipe(ctx context.Context, cluster, version string, recipe *model.Recipe) (*model.Recipe, error)
	UpdateRecipe(ctx context.Context, cluster, version, recipeVersion string, recipe *model.Recipe) (*model.Recipe, error)
	DeleteRecipe(ctx context.Context, cluster, version, recipeVersion string) (bool, error)
	GetComponents(ctx context.Context, cluster, version, recipeVersion string) (map[string]model.ComponentSpec, error)
	GetUpgradePaths(ctx context.Context, cluster, version, recipeVersion string) ([]string, error)
	CompareVersions(ctx context.Context, cluster, from, to string) (*model.VersionComparison, error)
	GetDeployPreview(ctx context.Context, cluster, version, baseline string) (*model.DeployPreview, error)
	ValidateComponentCompatibility(release *model.HelmRelease) error
}

type releaseService struct {
	repo   repository.CatalogRepository
	gitOps gitops.Client
}

// NewReleaseService initializes a ReleaseService.
func NewReleaseService(repo repository.CatalogRepository, gitOps gitops.Client) ReleaseService {
	return &releaseService{
		repo:   repo,
		gitOps: gitOps,
	}
}

func (s *releaseService) GetAllReleases(ctx context.Context, cluster string) ([]model.ReleaseSummary, error) {
	if s.gitOps != nil {
		_ = s.gitOps.SyncIfStale(ctx)
	}
	active, _ := s.repo.ReadEnvironmentVersion(ctx, cluster)

	versions, err := s.repo.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	var summaries []model.ReleaseSummary
	for _, v := range versions {
		rel, err := s.repo.ReadVersion(ctx, v)
		if err != nil || rel == nil {
			continue
		}
		status := "available"
		if v == active {
			status = "deployed"
		}
		summaries = append(summaries, model.ReleaseSummary{
			Version:       rel.Version,
			ReleaseName:   "recipe-" + cluster,
			Status:        status,
			Cluster:       cluster,
			CatalogName:   rel.CatalogName,
			CatalogStatus: rel.CatalogStatus,
		})
	}
	return summaries, nil
}

func (s *releaseService) GetRelease(ctx context.Context, cluster, version string) (*model.HelmRelease, error) {
	if s.gitOps != nil {
		_ = s.gitOps.SyncIfStale(ctx)
	}
	v := NormalizeVersion(version)
	rel, err := s.repo.ReadVersion(ctx, v)
	if err != nil {
		return nil, err
	}
	if rel == nil {
		return nil, nil
	}
	active, _ := s.repo.ReadEnvironmentVersion(ctx, cluster)
	rel.Cluster = cluster
	rel.ReleaseName = "recipe-" + cluster
	if rel.Version == active {
		rel.Status = "deployed"
	} else {
		rel.Status = "available"
	}
	return rel, nil
}

func (s *releaseService) CreateRelease(ctx context.Context, cluster string, release *model.HelmRelease) (*model.HelmRelease, error) {
	v := NormalizeVersion(release.Version)
	if v == "" {
		return nil, errors.New("version is required")
	}
	exists, err := s.repo.VersionExists(ctx, v)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("version %s already exists", v)
	}

	release.Version = v
	err = s.gitOps.Mutate(ctx, "catalog: create release "+v, func() error {
		return s.repo.WriteVersion(ctx, release)
	})
	if err != nil {
		return nil, err
	}
	return s.GetRelease(ctx, cluster, v)
}

func (s *releaseService) UpdateRelease(ctx context.Context, cluster, version string, release *model.HelmRelease) (*model.HelmRelease, error) {
	v := NormalizeVersion(version)
	exists, err := s.repo.VersionExists(ctx, v)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("version %s does not exist", v)
	}

	release.Version = v
	err = s.gitOps.Mutate(ctx, "catalog: update release "+v, func() error {
		return s.repo.WriteVersion(ctx, release)
	})
	if err != nil {
		return nil, err
	}
	return s.GetRelease(ctx, cluster, v)
}

func (s *releaseService) GetRecipes(ctx context.Context, cluster, version string) ([]model.Recipe, error) {
	rel, err := s.GetRelease(ctx, cluster, version)
	if err != nil {
		return nil, err
	}
	if rel == nil {
		return []model.Recipe{}, nil
	}
	return rel.Recipes, nil
}

func (s *releaseService) AddRecipe(ctx context.Context, cluster, version string, recipe *model.Recipe) (*model.Recipe, error) {
	v := NormalizeVersion(version)
	rel, err := s.repo.ReadVersion(ctx, v)
	if err != nil {
		return nil, err
	}
	if rel == nil {
		return nil, fmt.Errorf("release %s not found", v)
	}

	rel.Recipes = append(rel.Recipes, *recipe)
	err = s.gitOps.Mutate(ctx, fmt.Sprintf("catalog: add recipe %s to %s", recipe.Version, v), func() error {
		return s.repo.WriteVersion(ctx, rel)
	})
	if err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *releaseService) UpdateRecipe(ctx context.Context, cluster, version, recipeVersion string, recipe *model.Recipe) (*model.Recipe, error) {
	v := NormalizeVersion(version)
	rel, err := s.repo.ReadVersion(ctx, v)
	if err != nil {
		return nil, err
	}
	if rel == nil {
		return nil, fmt.Errorf("release %s not found", v)
	}

	found := false
	for i, r := range rel.Recipes {
		if r.Version == recipeVersion {
			if recipe.Version == "" {
				recipe.Version = recipeVersion
			}
			rel.Recipes[i] = *recipe
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("recipe %s not found in release %s", recipeVersion, v)
	}

	err = s.gitOps.Mutate(ctx, fmt.Sprintf("catalog: update recipe %s in %s", recipeVersion, v), func() error {
		return s.repo.WriteVersion(ctx, rel)
	})
	if err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *releaseService) DeleteRecipe(ctx context.Context, cluster, version, recipeVersion string) (bool, error) {
	v := NormalizeVersion(version)
	rel, err := s.repo.ReadVersion(ctx, v)
	if err != nil {
		return false, err
	}
	if rel == nil {
		return false, fmt.Errorf("release %s not found", v)
	}

	newRecipes := make([]model.Recipe, 0, len(rel.Recipes))
	found := false
	for _, r := range rel.Recipes {
		if r.Version == recipeVersion {
			found = true
			continue
		}
		newRecipes = append(newRecipes, r)
	}
	if !found {
		return false, nil
	}

	rel.Recipes = newRecipes
	err = s.gitOps.Mutate(ctx, fmt.Sprintf("catalog: delete recipe %s from %s", recipeVersion, v), func() error {
		return s.repo.WriteVersion(ctx, rel)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *releaseService) GetComponents(ctx context.Context, cluster, version, recipeVersion string) (map[string]model.ComponentSpec, error) {
	recipes, err := s.GetRecipes(ctx, cluster, version)
	if err != nil {
		return nil, err
	}
	for _, r := range recipes {
		if r.Version == recipeVersion {
			if r.Components == nil {
				return map[string]model.ComponentSpec{}, nil
			}
			return r.Components, nil
		}
	}
	return map[string]model.ComponentSpec{}, nil
}

func (s *releaseService) GetUpgradePaths(ctx context.Context, cluster, version, recipeVersion string) ([]string, error) {
	recipes, err := s.GetRecipes(ctx, cluster, version)
	if err != nil {
		return nil, err
	}
	for _, r := range recipes {
		if r.Version == recipeVersion {
			if r.UpgradeTo == nil {
				return []string{}, nil
			}
			return r.UpgradeTo, nil
		}
	}
	return []string{}, nil
}

func (s *releaseService) CompareVersions(ctx context.Context, cluster, from, to string) (*model.VersionComparison, error) {
	r1, err := s.repo.ReadVersion(ctx, NormalizeVersion(from))
	if err != nil {
		return nil, err
	}
	r2, err := s.repo.ReadVersion(ctx, NormalizeVersion(to))
	if err != nil {
		return nil, err
	}
	if r1 == nil || r2 == nil {
		return nil, errors.New("invalid versions: one or both releases do not exist")
	}

	fromByVersion := make(map[string]model.Recipe)
	for _, r := range r1.Recipes {
		fromByVersion[r.Version] = r
	}
	toByVersion := make(map[string]model.Recipe)
	for _, r := range r2.Recipes {
		toByVersion[r.Version] = r
	}

	var added []model.RecipeDiffItem
	var removed []model.RecipeDiffItem
	var changed []model.RecipeChangedItem

	for v, r := range toByVersion {
		if _, exists := fromByVersion[v]; !exists {
			added = append(added, model.RecipeDiffItem{
				Version:     v,
				Description: r.Description,
			})
		}
	}

	for v, r := range fromByVersion {
		if _, exists := toByVersion[v]; !exists {
			removed = append(removed, model.RecipeDiffItem{
				Version:     v,
				Description: r.Description,
			})
		}
	}

	for v, a := range fromByVersion {
		b, exists := toByVersion[v]
		if !exists {
			continue
		}

		changeEntry := model.RecipeChangedItem{
			Version: v,
		}

		// Diff components
		compsAdded := make(map[string]string)
		compsRemoved := make(map[string]string)
		compsChanged := make(map[string]map[string]string)

		for compName, bSpec := range b.Components {
			if aSpec, ok := a.Components[compName]; !ok {
				compsAdded[compName] = bSpec.Version
			} else if aSpec.Version != bSpec.Version {
				compsChanged[compName] = map[string]string{
					"from": aSpec.Version,
					"to":   bSpec.Version,
				}
			}
		}

		for compName, aSpec := range a.Components {
			if _, ok := b.Components[compName]; !ok {
				compsRemoved[compName] = aSpec.Version
			}
		}

		compChanges := make(map[string]interface{})
		if len(compsAdded) > 0 {
			compChanges["added"] = compsAdded
		}
		if len(compsRemoved) > 0 {
			compChanges["removed"] = compsRemoved
		}
		if len(compsChanged) > 0 {
			compChanges["changed"] = compsChanged
		}
		if len(compChanges) > 0 {
			changeEntry.Components = compChanges
		}

		// Diff paths
		fromSet := toSet(a.UpgradeTo)
		toSetMap := toSet(b.UpgradeTo)
		var pathsAdded []string
		var pathsRemoved []string

		for p := range toSetMap {
			if _, ok := fromSet[p]; !ok {
				pathsAdded = append(pathsAdded, p)
			}
		}
		for p := range fromSet {
			if _, ok := toSetMap[p]; !ok {
				pathsRemoved = append(pathsRemoved, p)
			}
		}
		sort.Strings(pathsAdded)
		sort.Strings(pathsRemoved)

		if len(pathsAdded) > 0 || len(pathsRemoved) > 0 {
			pathChanges := make(map[string]interface{})
			if len(pathsAdded) > 0 {
				pathChanges["added"] = pathsAdded
			}
			if len(pathsRemoved) > 0 {
				pathChanges["removed"] = pathsRemoved
			}
			changeEntry.UpgradeTo = pathChanges
		}

		if len(changeEntry.Components) > 0 || len(changeEntry.UpgradeTo) > 0 {
			changed = append(changed, changeEntry)
		}
	}

	sort.Slice(added, func(i, j int) bool { return added[i].Version < added[j].Version })
	sort.Slice(removed, func(i, j int) bool { return removed[i].Version < removed[j].Version })
	sort.Slice(changed, func(i, j int) bool { return changed[i].Version < changed[j].Version })

	return &model.VersionComparison{
		From:           from,
		To:             to,
		RecipesAdded:   added,
		RecipesRemoved: removed,
		RecipesChanged: changed,
	}, nil
}

func (s *releaseService) GetDeployPreview(ctx context.Context, cluster, version, baseline string) (*model.DeployPreview, error) {
	v := NormalizeVersion(version)
	proposed, err := s.repo.ReadVersion(ctx, v)
	if err != nil {
		return nil, err
	}
	if proposed == nil {
		return nil, fmt.Errorf("release %s not found", v)
	}

	activeVersion, _ := s.repo.ReadEnvironmentVersion(ctx, cluster)
	baselineVersion := baseline
	if baseline == "" || strings.EqualFold(baseline, "auto") || strings.EqualFold(baseline, "latest") {
		baselineVersion = activeVersion
	} else if strings.EqualFold(baseline, "none") || strings.EqualFold(baseline, "new") {
		baselineVersion = ""
	}

	if baselineVersion == "" {
		return &model.DeployPreview{
			Cluster:         cluster,
			TargetVersion:   v,
			BaselineVersion: "",
			IsNewDeploy:     true,
			HasChanges:      len(proposed.Recipes) > 0,
			Summary:         "First deploy to this cluster — no previous Helm release to compare against.",
			RecipesAdded:    []model.RecipeDiffItem{},
			RecipesRemoved:  []model.RecipeDiffItem{},
			RecipesChanged:  []model.RecipeChangedItem{},
		}, nil
	}

	diff, err := s.CompareVersions(ctx, cluster, baselineVersion, v)
	if err != nil {
		return nil, err
	}

	hasChanges := len(diff.RecipesAdded) > 0 || len(diff.RecipesRemoved) > 0 || len(diff.RecipesChanged) > 0
	summary := fmt.Sprintf("No recipe differences versus currently deployed v%s.", baselineVersion)
	if hasChanges {
		summary = fmt.Sprintf("Changes detected versus currently deployed v%s.", baselineVersion)
	}

	return &model.DeployPreview{
		Cluster:         cluster,
		TargetVersion:   v,
		BaselineVersion: baselineVersion,
		IsNewDeploy:     false,
		HasChanges:      hasChanges,
		Summary:         summary,
		RecipesAdded:    diff.RecipesAdded,
		RecipesRemoved:  diff.RecipesRemoved,
		RecipesChanged:  diff.RecipesChanged,
	}, nil
}

func (s *releaseService) ValidateComponentCompatibility(release *model.HelmRelease) error {
	if release == nil || len(release.Recipes) == 0 {
		return nil
	}

	recipesByVersion := make(map[string]model.Recipe)
	for _, r := range release.Recipes {
		recipesByVersion[r.Version] = r
	}

	for _, target := range release.Recipes {
		fromVersions := target.UpgradeFrom
		if len(fromVersions) == 0 {
			// Infer from other recipes whose upgrade_to contains target.Version
			for _, r := range release.Recipes {
				for _, upTo := range r.UpgradeTo {
					if upTo == target.Version {
						fromVersions = append(fromVersions, r.Version)
					}
				}
			}
		}

		for _, fromV := range fromVersions {
			source, ok := recipesByVersion[fromV]
			if !ok {
				return fmt.Errorf("upgrade path references missing recipe version: %s", fromV)
			}

			for compName, targetSpec := range target.Components {
				sourceSpec, ok := source.Components[compName]
				if !ok {
					continue
				}

				if len(targetSpec.UpgradeFrom) > 0 && !contains(targetSpec.UpgradeFrom, sourceSpec.Version) {
					return fmt.Errorf("component %s version %s cannot upgrade from %s", compName, targetSpec.Version, sourceSpec.Version)
				}
				if len(sourceSpec.UpgradeTo) > 0 && !contains(sourceSpec.UpgradeTo, targetSpec.Version) {
					return fmt.Errorf("component %s version %s cannot upgrade to %s", compName, sourceSpec.Version, targetSpec.Version)
				}
			}
		}
	}
	return nil
}

func toSet(items []string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, it := range items {
		m[it] = struct{}{}
	}
	return m
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

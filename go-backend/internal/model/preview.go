package model

// DeployPreview represents the diff report between a target version and the current cluster baseline.
type DeployPreview struct {
	Cluster         string              `json:"cluster"`
	TargetVersion   string              `json:"targetVersion"`
	BaselineVersion string              `json:"baselineVersion"`
	IsNewDeploy     bool                `json:"isNewDeploy"`
	HasChanges      bool                `json:"hasChanges"`
	Summary         string              `json:"summary"`
	RecipesAdded    []RecipeDiffItem    `json:"recipesAdded"`
	RecipesRemoved  []RecipeDiffItem    `json:"recipesRemoved"`
	RecipesChanged  []RecipeChangedItem `json:"recipesChanged"`
}

// RecipeDiffItem represents an added or removed recipe.
type RecipeDiffItem struct {
	Version     string `json:"version"`
	Description string `json:"description"`
}

// RecipeChangedItem details changes within a specific recipe (component version bumps, upgrade paths).
type RecipeChangedItem struct {
	Version    string                 `json:"version"`
	Components map[string]interface{} `json:"components,omitempty"`
	UpgradeTo  map[string]interface{} `json:"upgrade_to,omitempty"`
}

// VersionComparison is the payload returned by /api/helm-releases/compare.
type VersionComparison struct {
	From           string              `json:"from"`
	To             string              `json:"to"`
	RecipesAdded   []RecipeDiffItem    `json:"recipesAdded"`
	RecipesRemoved []RecipeDiffItem    `json:"recipesRemoved"`
	RecipesChanged []RecipeChangedItem `json:"recipesChanged"`
}

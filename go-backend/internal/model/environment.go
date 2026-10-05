package model

// EnvironmentFile models the content of catalogs/recipe-detection/environments/<env>.yaml
type EnvironmentFile struct {
	CatalogVersion string `json:"catalogVersion" yaml:"catalogVersion"`
}

// PromotionOptions contains the pipeline status, active placements, and valid promotion/rollback targets.
type PromotionOptions struct {
	Pipeline               []string          `json:"pipeline"`
	DeployedOn             map[string]bool   `json:"deployedOn"`
	ActiveVersionOnCluster map[string]string `json:"activeVersionOnCluster"`
	AllowedTargets         []string          `json:"allowedTargets"`
	NextTarget             string            `json:"nextTarget,omitempty"`
	CanRollback            map[string]bool   `json:"canRollback"`
}

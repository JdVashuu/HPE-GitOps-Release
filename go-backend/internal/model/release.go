package model

// HelmRelease represents the domain catalog definition and its deployment attributes.
type HelmRelease struct {
	Version            string            `json:"version" yaml:"chartVersion"`
	ReleaseName        string            `json:"releaseName,omitempty" yaml:"-"`
	Status             string            `json:"status,omitempty" yaml:"-"`
	Cluster            string            `json:"cluster,omitempty" yaml:"target_cluster,omitempty"`
	CatalogName        string            `json:"catalog_name,omitempty" yaml:"catalog_name,omitempty"`
	CatalogDescription string            `json:"catalog_description,omitempty" yaml:"catalog_description,omitempty"`
	CatalogReleaseDate string            `json:"release_date,omitempty" yaml:"release_date,omitempty"`
	CatalogStatus      string            `json:"catalog_status,omitempty" yaml:"catalog_status,omitempty"`
	Maintainer         string            `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	ValuesFileName     string            `json:"valuesFileName,omitempty" yaml:"values_file,omitempty"`
	Recipes            []Recipe          `json:"recipes" yaml:"recipes"`
}

// HelmReleaseWrapper wraps HelmRelease under the top-level recipeData key in YAML files.
type HelmReleaseWrapper struct {
	RecipeData HelmRelease `yaml:"recipeData"`
}

// ReleaseSummary provides a lightweight summary of a release in an environment.
type ReleaseSummary struct {
	Version       string `json:"version"`
	ReleaseName   string `json:"releaseName"`
	Status        string `json:"status"` // "deployed" or "available"
	Cluster       string `json:"cluster"`
	CatalogName   string `json:"catalog_name,omitempty"`
	CatalogStatus string `json:"catalog_status,omitempty"`
}

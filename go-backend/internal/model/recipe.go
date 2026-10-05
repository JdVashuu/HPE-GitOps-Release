package model

// Recipe represents an application/service within a catalog version.
type Recipe struct {
	Version      string                   `json:"version" yaml:"version"`
	Description  string                   `json:"description" yaml:"description"`
	ReleaseDate  string                   `json:"release_date,omitempty" yaml:"release_date,omitempty"`
	Status       string                   `json:"status,omitempty" yaml:"status,omitempty"`
	ReleaseNotes string                   `json:"release_notes,omitempty" yaml:"release_notes,omitempty"`
	Components   map[string]ComponentSpec `json:"components" yaml:"components"`
	UpgradeTo    []string                 `json:"upgrade_to,omitempty" yaml:"upgrade_to,omitempty"`
	UpgradeFrom  []string                 `json:"upgrade_from,omitempty" yaml:"upgrade_from,omitempty"`
}

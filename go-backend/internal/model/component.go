package model

// ComponentSpec defines version, release date, and upgrade constraints for a sub-component.
type ComponentSpec struct {
	Version     string   `json:"version" yaml:"version"`
	ReleaseDate string   `json:"release_date,omitempty" yaml:"release_date,omitempty"`
	UpgradeFrom []string `json:"upgrade_from" yaml:"upgrade_from"`
	UpgradeTo   []string `json:"upgrade_to" yaml:"upgrade_to"`
}

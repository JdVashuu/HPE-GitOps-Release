package model

// AuditLogEntry represents an event record in catalogs/recipe-detection/history.yaml
type AuditLogEntry struct {
	Timestamp   string `json:"timestamp" yaml:"timestamp"`
	Action      string `json:"action" yaml:"action"` // create, deploy, promote, edit, rollback, uninstall, delete
	Version     string `json:"version" yaml:"version"`
	Env         string `json:"env,omitempty" yaml:"env,omitempty"`
	FromVersion string `json:"fromVersion,omitempty" yaml:"fromVersion,omitempty"`
}

package repository

import (
	"context"
	"hpe-recipe/internal/model"
)

// CatalogRepository defines operations for persisting and retrieving catalog metadata from disk.
type CatalogRepository interface {
	// Versions
	ListVersions(ctx context.Context) ([]string, error)
	VersionExists(ctx context.Context, version string) (bool, error)
	ReadVersion(ctx context.Context, version string) (*model.HelmRelease, error)
	WriteVersion(ctx context.Context, release *model.HelmRelease) error
	DeleteVersion(ctx context.Context, version string) error

	// Environments
	ReadEnvironmentVersion(ctx context.Context, env string) (string, error)
	ReadAllEnvironments(ctx context.Context) (map[string]string, error)
	SetEnvironmentVersion(ctx context.Context, env, version string) error
	DeleteEnvironment(ctx context.Context, env string) error

	// Environment History (Rollback Timeline)
	ReadEnvironmentHistory(ctx context.Context, env string) ([]string, error)
	ReadEnvironmentHistories(ctx context.Context, envs []string) (map[string][]string, error)
	AppendEnvironmentHistory(ctx context.Context, env, version string) error

	// Audit History (UI Event Log)
	ReadAuditHistory(ctx context.Context) ([]model.AuditLogEntry, error)
	AppendAuditHistory(ctx context.Context, entry model.AuditLogEntry) error
	ClearAuditHistory(ctx context.Context) error
}

// WorkspaceLockManager synchronizes concurrent read/write operations against the local Git clone.
type WorkspaceLockManager interface {
	RLock()
	RUnlock()
	Lock()
	Unlock()
}

package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"hpe-recipe/internal/model"
	"hpe-recipe/internal/repository"
)

const (
	catalogSubPath = "catalogs/recipe-detection"
)

type yamlCatalogRepository struct {
	repoRoot string
}

// NewYAMLCatalogRepository initializes a file-based catalog repository rooted at repoRoot.
func NewYAMLCatalogRepository(repoRoot string) repository.CatalogRepository {
	return &yamlCatalogRepository{
		repoRoot: repoRoot,
	}
}

func (r *yamlCatalogRepository) versionsDir() string {
	return filepath.Join(r.repoRoot, catalogSubPath, "versions")
}

func (r *yamlCatalogRepository) environmentsDir() string {
	return filepath.Join(r.repoRoot, catalogSubPath, "environments")
}

func (r *yamlCatalogRepository) envHistoryDir() string {
	return filepath.Join(r.repoRoot, catalogSubPath, "environment-history")
}

func (r *yamlCatalogRepository) historyFile() string {
	return filepath.Join(r.repoRoot, catalogSubPath, "history.yaml")
}

// ======================== VERSIONS ========================

func (r *yamlCatalogRepository) ListVersions(ctx context.Context) ([]string, error) {
	dir := r.versionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read versions dir failed: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			v := strings.TrimSuffix(entry.Name(), ".yaml")
			versions = append(versions, v)
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func (r *yamlCatalogRepository) VersionExists(ctx context.Context, version string) (bool, error) {
	v, err := validateID("version", version)
	if err != nil {
		return false, err
	}
	targetPath := filepath.Join(r.versionsDir(), v+".yaml")
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

func (r *yamlCatalogRepository) ReadVersion(ctx context.Context, version string) (*model.HelmRelease, error) {
	v, err := validateID("version", version)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(r.versionsDir(), v+".yaml")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read version %s failed: %w", v, err)
	}

	var wrapper model.HelmReleaseWrapper
	if err := yaml.Unmarshal(data, &wrapper); err == nil && wrapper.RecipeData.Version != "" {
		res := wrapper.RecipeData
		if res.Version == "" {
			res.Version = v
		}
		return &res, nil
	}

	// Fallback to unwrapped HelmRelease
	var direct model.HelmRelease
	if err := yaml.Unmarshal(data, &direct); err != nil {
		return nil, fmt.Errorf("parse version %s YAML failed: %w", v, err)
	}
	if direct.Version == "" {
		direct.Version = v
	}
	return &direct, nil
}

func (r *yamlCatalogRepository) WriteVersion(ctx context.Context, release *model.HelmRelease) error {
	if release == nil {
		return errors.New("release must not be nil")
	}
	v, err := validateID("version", release.Version)
	if err != nil {
		return err
	}
	release.Version = v

	if err := os.MkdirAll(r.versionsDir(), 0755); err != nil {
		return fmt.Errorf("mkdir versions dir failed: %w", err)
	}

	wrapper := model.HelmReleaseWrapper{
		RecipeData: *release,
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal release %s failed: %w", v, err)
	}

	targetPath := filepath.Join(r.versionsDir(), v+".yaml")
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("write version file %s failed: %w", targetPath, err)
	}
	return nil
}

func (r *yamlCatalogRepository) DeleteVersion(ctx context.Context, version string) error {
	v, err := validateID("version", version)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(r.versionsDir(), v+".yaml")
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete version %s failed: %w", v, err)
	}
	return nil
}

// ======================== ENVIRONMENTS ========================

func (r *yamlCatalogRepository) ReadEnvironmentVersion(ctx context.Context, env string) (string, error) {
	e, err := validateID("environment", env)
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(r.environmentsDir(), e+".yaml")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read environment %s failed: %w", e, err)
	}

	var envFile model.EnvironmentFile
	if err := yaml.Unmarshal(data, &envFile); err != nil {
		return "", fmt.Errorf("parse environment %s failed: %w", e, err)
	}
	return strings.TrimSpace(envFile.CatalogVersion), nil
}

func (r *yamlCatalogRepository) ReadAllEnvironments(ctx context.Context) (map[string]string, error) {
	dir := r.environmentsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read environments dir failed: %w", err)
	}

	result := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			envName := strings.TrimSuffix(entry.Name(), ".yaml")
			v, err := r.ReadEnvironmentVersion(ctx, envName)
			if err != nil {
				return nil, err
			}
			if v != "" {
				result[envName] = v
			}
		}
	}
	return result, nil
}

func (r *yamlCatalogRepository) SetEnvironmentVersion(ctx context.Context, env, version string) error {
	e, err := validateID("environment", env)
	if err != nil {
		return err
	}
	v, err := validateID("version", version)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(r.environmentsDir(), 0755); err != nil {
		return fmt.Errorf("mkdir environments dir failed: %w", err)
	}

	envFile := model.EnvironmentFile{
		CatalogVersion: v,
	}
	data, err := yaml.Marshal(envFile)
	if err != nil {
		return fmt.Errorf("marshal environment %s failed: %w", e, err)
	}

	targetPath := filepath.Join(r.environmentsDir(), e+".yaml")
	return os.WriteFile(targetPath, data, 0644)
}

func (r *yamlCatalogRepository) DeleteEnvironment(ctx context.Context, env string) error {
	e, err := validateID("environment", env)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(r.environmentsDir(), e+".yaml")
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete environment %s failed: %w", e, err)
	}
	return nil
}

// ======================== ENVIRONMENT HISTORY ========================

func (r *yamlCatalogRepository) ReadEnvironmentHistory(ctx context.Context, env string) ([]string, error) {
	e, err := validateID("environment", env)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(r.envHistoryDir(), e+".yaml")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read environment history %s failed: %w", e, err)
	}

	var history []string
	if err := yaml.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("parse environment history %s failed: %w", e, err)
	}
	if history == nil {
		return []string{}, nil
	}
	return history, nil
}

func (r *yamlCatalogRepository) ReadEnvironmentHistories(ctx context.Context, envs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, env := range envs {
		hist, err := r.ReadEnvironmentHistory(ctx, env)
		if err != nil {
			return nil, err
		}
		result[env] = hist
	}
	return result, nil
}

func (r *yamlCatalogRepository) AppendEnvironmentHistory(ctx context.Context, env, version string) error {
	e, err := validateID("environment", env)
	if err != nil {
		return err
	}
	v, err := validateID("version", version)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(r.envHistoryDir(), 0755); err != nil {
		return fmt.Errorf("mkdir environment-history dir failed: %w", err)
	}

	history, err := r.ReadEnvironmentHistory(ctx, e)
	if err != nil {
		return err
	}
	history = append(history, v)

	data, err := yaml.Marshal(history)
	if err != nil {
		return fmt.Errorf("marshal environment history %s failed: %w", e, err)
	}

	targetPath := filepath.Join(r.envHistoryDir(), e+".yaml")
	return os.WriteFile(targetPath, data, 0644)
}

// ======================== AUDIT HISTORY ========================

func (r *yamlCatalogRepository) ReadAuditHistory(ctx context.Context) ([]model.AuditLogEntry, error) {
	targetPath := r.historyFile()
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.AuditLogEntry{}, nil
		}
		return nil, fmt.Errorf("read history file failed: %w", err)
	}

	var logs []model.AuditLogEntry
	if err := yaml.Unmarshal(data, &logs); err != nil {
		return nil, fmt.Errorf("parse history file failed: %w", err)
	}
	if logs == nil {
		return []model.AuditLogEntry{}, nil
	}
	return logs, nil
}

func (r *yamlCatalogRepository) AppendAuditHistory(ctx context.Context, entry model.AuditLogEntry) error {
	if err := os.MkdirAll(filepath.Dir(r.historyFile()), 0755); err != nil {
		return fmt.Errorf("mkdir history file dir failed: %w", err)
	}

	logs, err := r.ReadAuditHistory(ctx)
	if err != nil {
		logs = []model.AuditLogEntry{}
	}
	logs = append(logs, entry)

	data, err := yaml.Marshal(logs)
	if err != nil {
		return fmt.Errorf("marshal history logs failed: %w", err)
	}

	return os.WriteFile(r.historyFile(), data, 0644)
}

func (r *yamlCatalogRepository) ClearAuditHistory(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(r.historyFile()), 0755); err != nil {
		return fmt.Errorf("mkdir history file dir failed: %w", err)
	}

	empty := []model.AuditLogEntry{}
	data, err := yaml.Marshal(empty)
	if err != nil {
		return fmt.Errorf("marshal empty history failed: %w", err)
	}

	return os.WriteFile(r.historyFile(), data, 0644)
}

// validateID ensures IDs cannot traverse directories or contain invalid path characters.
func validateID(kind, id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	if trimmed == "" {
		return "", fmt.Errorf("%s must not be blank", kind)
	}
	if strings.Contains(trimmed, "..") || strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("invalid %s '%s' (must not contain '..', '/', or '\\')", kind, id)
	}
	return trimmed, nil
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"hpe-recipe/internal/config"
	"hpe-recipe/internal/integration/gitops"
	"hpe-recipe/internal/integration/jenkins"
	"hpe-recipe/internal/model"
	"hpe-recipe/internal/repository"
)

// PlatformService defines operations for the catalog promotion lifecycle.
type PlatformService interface {
	GetPipeline() []string
	GetEnvironments(ctx context.Context) (map[string]string, error)
	GetVersions(ctx context.Context) ([]string, error)
	GetVersion(ctx context.Context, version string) (*model.HelmRelease, error)
	GetPromotionOptions(ctx context.Context, version string) (*model.PromotionOptions, error)
	GetHistory(ctx context.Context) ([]model.AuditLogEntry, error)
	ClearHistory(ctx context.Context) error

	CreateVersion(ctx context.Context, release *model.HelmRelease) (*model.HelmRelease, error)
	CreateAndDeployToDev(ctx context.Context, release *model.HelmRelease) (*model.HelmRelease, error)
	DeployToDev(ctx context.Context, version string) error
	Promote(ctx context.Context, version, toEnv string) error
	Rollback(ctx context.Context, env string) (string, error)
	EditDev(ctx context.Context, edited *model.HelmRelease) (*model.HelmRelease, error)
	CompleteDeployment(ctx context.Context, version, env, action, fromVersion string) error
	DeleteVersion(ctx context.Context, version string) error
}

type platformService struct {
	cfg      *config.Config
	repo     repository.CatalogRepository
	gitOps   gitops.Client
	jenkins  jenkins.Client
	events   EventService
}

// NewPlatformService creates an instance of PlatformService.
func NewPlatformService(
	cfg *config.Config,
	repo repository.CatalogRepository,
	gitOps gitops.Client,
	jenkins jenkins.Client,
	events EventService,
) PlatformService {
	return &platformService{
		cfg:     cfg,
		repo:    repo,
		gitOps:  gitOps,
		jenkins: jenkins,
		events:  events,
	}
}

func (s *platformService) GetPipeline() []string {
	return s.cfg.Promotion.Pipeline
}

func (s *platformService) GetEnvironments(ctx context.Context) (map[string]string, error) {
	if s.gitOps != nil {
		if err := s.gitOps.SyncIfStale(ctx); err != nil {
			slog.Warn("Sync failed during GetEnvironments", "err", err)
		}
	}
	return s.repo.ReadAllEnvironments(ctx)
}

func (s *platformService) GetVersions(ctx context.Context) ([]string, error) {
	if s.gitOps != nil {
		if err := s.gitOps.SyncIfStale(ctx); err != nil {
			slog.Warn("Sync failed during GetVersions", "err", err)
		}
	}
	return s.repo.ListVersions(ctx)
}

func (s *platformService) GetVersion(ctx context.Context, version string) (*model.HelmRelease, error) {
	if s.gitOps != nil {
		if err := s.gitOps.SyncIfStale(ctx); err != nil {
			slog.Warn("Sync failed during GetVersion", "err", err)
		}
	}
	return s.repo.ReadVersion(ctx, NormalizeVersion(version))
}

func (s *platformService) GetHistory(ctx context.Context) ([]model.AuditLogEntry, error) {
	if s.gitOps != nil {
		if err := s.gitOps.SyncIfStale(ctx); err != nil {
			slog.Warn("Sync failed during GetHistory", "err", err)
		}
	}
	return s.repo.ReadAuditHistory(ctx)
}

func (s *platformService) ClearHistory(ctx context.Context) error {
	return s.gitOps.Mutate(ctx, "catalog: clear deployment history", func() error {
		return s.repo.ClearAuditHistory(ctx)
	})
}

func (s *platformService) GetPromotionOptions(ctx context.Context, version string) (*model.PromotionOptions, error) {
	v := NormalizeVersion(version)
	pipeline := s.GetPipeline()

	envs, err := s.GetEnvironments(ctx)
	if err != nil {
		return nil, err
	}

	histories, err := s.repo.ReadEnvironmentHistories(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	deployedOn := make(map[string]bool)
	activeVersions := make(map[string]string)
	canRollback := make(map[string]bool)

	for _, env := range pipeline {
		active := envs[env]
		activeVersions[env] = active
		deployedOn[env] = (v == active)
		hist := histories[env]
		canRollback[env] = (len(hist) >= 2)
	}

	furthest := -1
	for i, env := range pipeline {
		if v == envs[env] {
			furthest = i
		}
	}

	var allowedTargets []string
	var nextTarget string
	if furthest >= 0 && furthest < len(pipeline)-1 {
		nextTarget = pipeline[furthest+1]
		allowedTargets = append(allowedTargets, nextTarget)
	}

	return &model.PromotionOptions{
		Pipeline:               pipeline,
		DeployedOn:             deployedOn,
		ActiveVersionOnCluster: activeVersions,
		AllowedTargets:         allowedTargets,
		NextTarget:             nextTarget,
		CanRollback:            canRollback,
	}, nil
}

func (s *platformService) CreateVersion(ctx context.Context, release *model.HelmRelease) (*model.HelmRelease, error) {
	if release == nil {
		return nil, errors.New("release body is required")
	}

	versions, err := s.GetVersions(ctx)
	if err != nil {
		return nil, err
	}
	if len(versions) > 0 {
		return nil, errors.New("create is only available on an empty system; a catalog version already exists (use Edit on dev to fork a new version)")
	}

	version := NormalizeVersion(release.Version)
	release.Version = version

	err = s.gitOps.Mutate(ctx, "catalog: write version "+version, func() error {
		if err := s.repo.WriteVersion(ctx, release); err != nil {
			return err
		}
		return s.repo.AppendAuditHistory(ctx, model.AuditLogEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Action:    "create",
			Version:   version,
		})
	})
	if err != nil {
		return nil, err
	}

	return s.repo.ReadVersion(ctx, version)
}

func (s *platformService) CreateAndDeployToDev(ctx context.Context, release *model.HelmRelease) (*model.HelmRelease, error) {
	if release == nil {
		return nil, errors.New("release body is required")
	}

	pipeline := s.GetPipeline()
	if len(pipeline) == 0 {
		return nil, errors.New("pipeline is empty")
	}
	dev := pipeline[0]

	currentDev, err := s.repo.ReadEnvironmentVersion(ctx, dev)
	if err != nil {
		return nil, err
	}
	if currentDev != "" {
		return nil, fmt.Errorf("create is only available when %s has no deployed catalog (use Edit on dev to fork a new version)", dev)
	}

	version := NormalizeVersion(release.Version)
	exists, err := s.repo.VersionExists(ctx, version)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("version already exists: %s", version)
	}

	release.Version = version

	err = s.gitOps.Mutate(ctx, "catalog: write version "+version, func() error {
		return s.repo.WriteVersion(ctx, release)
	})
	if err != nil {
		return nil, err
	}

	if err := s.renderAndTrigger(ctx, version, dev, "create_deploy", ""); err != nil {
		return nil, err
	}

	return s.repo.ReadVersion(ctx, version)
}

func (s *platformService) DeployToDev(ctx context.Context, version string) error {
	v := NormalizeVersion(version)
	exists, err := s.repo.VersionExists(ctx, v)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("version does not exist: %s", v)
	}

	dev := s.GetPipeline()[0]
	return s.renderAndTrigger(ctx, v, dev, "deploy", "")
}

func (s *platformService) Promote(ctx context.Context, version, toEnv string) error {
	v := NormalizeVersion(version)
	pipeline := s.GetPipeline()

	idx := -1
	for i, env := range pipeline {
		if env == toEnv {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("unknown environment: %s", toEnv)
	}
	if idx == 0 {
		return fmt.Errorf("cannot promote to %s (it is the first stage; deploy to it directly)", toEnv)
	}

	prev := pipeline[idx-1]
	prevVersion, err := s.repo.ReadEnvironmentVersion(ctx, prev)
	if err != nil {
		return err
	}
	if v != prevVersion {
		return fmt.Errorf("version %s must be active on %s before promoting to %s", v, prev, toEnv)
	}

	return s.renderAndTrigger(ctx, v, toEnv, "promote", prev)
}

func (s *platformService) Rollback(ctx context.Context, env string) (string, error) {
	pipeline := s.GetPipeline()
	found := false
	for _, p := range pipeline {
		if p == env {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("unknown environment: %s", env)
	}

	history, err := s.repo.ReadEnvironmentHistory(ctx, env)
	if err != nil {
		return "", err
	}
	if len(history) < 2 {
		return "", fmt.Errorf("no previous version to roll back to on %s", env)
	}

	current := history[len(history)-1]
	previous := history[len(history)-2]

	if err := s.renderAndTrigger(ctx, previous, env, "rollback", current); err != nil {
		return "", err
	}
	return previous, nil
}

func (s *platformService) EditDev(ctx context.Context, edited *model.HelmRelease) (*model.HelmRelease, error) {
	if edited == nil {
		return nil, errors.New("edited catalog body is required")
	}

	dev := s.GetPipeline()[0]
	currentDev, err := s.repo.ReadEnvironmentVersion(ctx, dev)
	if err != nil {
		return nil, err
	}
	if currentDev == "" {
		return nil, errors.New("nothing to edit: dev has no deployed catalog version yet")
	}

	versions, err := s.repo.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	existingMap := make(map[string]struct{})
	for _, v := range versions {
		existingMap[v] = struct{}{}
	}

	newVersion := NextPatchVersion(currentDev, existingMap)
	edited.Version = newVersion

	err = s.gitOps.Mutate(ctx, "catalog: edit dev -> "+newVersion, func() error {
		return s.repo.WriteVersion(ctx, edited)
	})
	if err != nil {
		return nil, err
	}

	if err := s.renderAndTrigger(ctx, newVersion, dev, "edit", currentDev); err != nil {
		return nil, err
	}

	return s.repo.ReadVersion(ctx, newVersion)
}

func (s *platformService) CompleteDeployment(ctx context.Context, version, env, action, fromVersion string) error {
	v := NormalizeVersion(version)
	exists, err := s.repo.VersionExists(ctx, v)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("version does not exist: %s", v)
	}

	eventAction := action
	if eventAction == "" {
		eventAction = "deploy"
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	return s.gitOps.Mutate(ctx, fmt.Sprintf("catalog: complete %s %s -> %s", eventAction, env, v), func() error {
		if err := s.repo.SetEnvironmentVersion(ctx, env, v); err != nil {
			return err
		}
		if err := s.repo.AppendEnvironmentHistory(ctx, env, v); err != nil {
			return err
		}

		switch eventAction {
		case "create_deploy":
			_ = s.repo.AppendAuditHistory(ctx, model.AuditLogEntry{
				Timestamp: now,
				Action:    "create",
				Version:   v,
			})
			return s.repo.AppendAuditHistory(ctx, model.AuditLogEntry{
				Timestamp: now,
				Action:    "deploy",
				Version:   v,
				Env:       env,
			})
		case "deploy", "promote", "edit", "rollback":
			return s.repo.AppendAuditHistory(ctx, model.AuditLogEntry{
				Timestamp:   now,
				Action:      eventAction,
				Version:     v,
				Env:         env,
				FromVersion: fromVersion,
			})
		default:
			return fmt.Errorf("unknown deployment completion action: %s", eventAction)
		}
	})
}

func (s *platformService) DeleteVersion(ctx context.Context, version string) error {
	v := NormalizeVersion(version)
	envs, err := s.repo.ReadAllEnvironments(ctx)
	if err != nil {
		return err
	}

	var affected []string
	for env, curV := range envs {
		if curV == v {
			affected = append(affected, env)
		}
	}

	versionExists, err := s.repo.VersionExists(ctx, v)
	if err != nil {
		return err
	}

	if len(affected) == 0 && !versionExists {
		return nil
	}

	// 1. Uninstall on all affected environments
	for _, env := range affected {
		_ = s.jenkins.Trigger(ctx, jenkins.BuildParams{
			Cluster: env,
			Action:  "uninstall",
		})

		_ = s.gitOps.Mutate(ctx, "catalog: clear environment "+env, func() error {
			_ = s.repo.DeleteEnvironment(ctx, env)
			return s.repo.AppendAuditHistory(ctx, model.AuditLogEntry{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Action:    "uninstall",
				Version:   v,
				Env:       env,
			})
		})
	}

	// 2. Delete version file
	if versionExists {
		_ = s.gitOps.Mutate(ctx, "catalog: delete version "+v, func() error {
			_ = s.repo.DeleteVersion(ctx, v)
			return s.repo.AppendAuditHistory(ctx, model.AuditLogEntry{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Action:    "delete",
				Version:   v,
			})
		})
	}

	return nil
}

func (s *platformService) renderAndTrigger(ctx context.Context, version, env, action, fromVersion string) error {
	release, err := s.repo.ReadVersion(ctx, version)
	if err != nil {
		return fmt.Errorf("read version %s failed: %w", version, err)
	}
	if release == nil {
		return fmt.Errorf("version not found: %s", version)
	}

	release.Cluster = env
	release.ReleaseName = "recipe-" + env
	release.Status = "deploying"

	valuesFile, err := s.gitOps.GenerateAndPushRelease(ctx, release)
	if err != nil {
		return fmt.Errorf("generate and push release failed: %w", err)
	}

	return s.jenkins.Trigger(ctx, jenkins.BuildParams{
		Cluster:           env,
		Action:            "deploy",
		ChartVersion:      release.Version,
		ValuesFile:        valuesFile,
		DeployEventAction: action,
		FromVersion:       fromVersion,
	})
}

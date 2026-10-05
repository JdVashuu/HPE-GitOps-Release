package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"hpe-recipe/internal/model"
	"hpe-recipe/internal/repository"
)

const maxPushAttempts = 3

// Client coordinates Git operations (pull, stage, commit, push, reset).
type Client interface {
	OpenOrClone(ctx context.Context) error
	SyncIfStale(ctx context.Context) error
	SyncToRemote(ctx context.Context) error
	Mutate(ctx context.Context, message string, mutator func() error) error
	GenerateAndPushRelease(ctx context.Context, release *model.HelmRelease) (string, error)
	GetRepoPath() string
}

type gitOpsClient struct {
	repoURL             string
	localPath           string
	branch              string
	username            string
	token               string
	valuesDir           string
	cacheTTL            time.Duration
	lockManager         repository.WorkspaceLockManager
	repo                *git.Repository
	lastSyncedAtMillis  atomic.Int64
	remoteSyncCount     atomic.Int64
}

// NewClient creates a new unified GitOps client.
func NewClient(
	repoURL, localPath, branch, username, token, valuesDir string,
	cacheTTL time.Duration,
	lockManager repository.WorkspaceLockManager,
) Client {
	return &gitOpsClient{
		repoURL:     repoURL,
		localPath:   localPath,
		branch:      branch,
		username:    username,
		token:       token,
		valuesDir:   valuesDir,
		cacheTTL:    cacheTTL,
		lockManager: lockManager,
	}
}

func (g *gitOpsClient) GetRepoPath() string {
	return g.localPath
}

func (g *gitOpsClient) auth() transport.AuthMethod {
	if g.token != "" {
		username := g.username
		if username == "" {
			username = "git"
		}
		return &githttp.BasicAuth{
			Username: username,
			Password: g.token,
		}
	}
	return nil
}

// OpenOrClone opens an existing repository or clones from remote.
func (g *gitOpsClient) OpenOrClone(ctx context.Context) error {
	g.lockManager.Lock()
	defer g.lockManager.Unlock()
	return g.openOrCloneUnlocked(ctx)
}

func (g *gitOpsClient) openOrCloneUnlocked(ctx context.Context) error {
	gitDir := filepath.Join(g.localPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		r, err := git.PlainOpen(g.localPath)
		if err == nil {
			g.repo = r
			slog.Info("Opened existing Git repository", "path", g.localPath)
			return nil
		}
		slog.Warn("Failed to open existing repo, re-cloning", "err", err)
		os.RemoveAll(g.localPath)
	}

	slog.Info("Cloning repository", "url", g.repoURL, "target", g.localPath, "branch", g.branch)
	r, err := git.PlainCloneContext(ctx, g.localPath, false, &git.CloneOptions{
		URL:           g.repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(g.branch),
		SingleBranch:  true,
		Auth:          g.auth(),
	})
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	g.repo = r
	g.markSynced()
	return nil
}

// SyncIfStale checks cache TTL and triggers an asynchronous refresh if elapsed.
func (g *gitOpsClient) SyncIfStale(ctx context.Context) error {
	now := time.Now().UnixMilli()
	if now-g.lastSyncedAtMillis.Load() > g.cacheTTL.Milliseconds() {
		g.lastSyncedAtMillis.Store(now)
		if g.token != "" {
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = g.SyncToRemote(bgCtx)
			}()
		}
	}
	return nil
}

// SyncToRemote executes fetch against origin/<branch>.
func (g *gitOpsClient) SyncToRemote(ctx context.Context) error {
	if g.repo == nil {
		if err := g.openOrCloneUnlocked(ctx); err != nil {
			return err
		}
	}

	remote, err := g.repo.Remote("origin")
	if err != nil {
		// If remote doesn't exist, skip remote sync (e.g. in offline unit tests)
		return nil
	}

	if g.token == "" {
		// Remote credentials not configured; skip remote fetch and use local repo state
		g.markSynced()
		return nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err = remote.FetchContext(fetchCtx, &git.FetchOptions{
		Auth: g.auth(),
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		slog.Warn("Git fetch warning (continuing with local state)", "err", err)
	}

	g.remoteSyncCount.Add(1)
	g.markSynced()
	return nil
}

func (g *gitOpsClient) markSynced() {
	g.lastSyncedAtMillis.Store(time.Now().UnixMilli())
}

// Mutate executes a file mutation under a write lock, committing and pushing to remote with retries.
func (g *gitOpsClient) Mutate(ctx context.Context, message string, mutator func() error) error {
	g.lockManager.Lock()
	defer g.lockManager.Unlock()

	for attempt := 1; attempt <= maxPushAttempts; attempt++ {
		// 1. Sync to remote
		if err := g.SyncToRemote(ctx); err != nil {
			slog.Warn("Pre-mutation sync warning", "attempt", attempt, "err", err)
		}

		// 2. Perform file edits
		if err := mutator(); err != nil {
			return fmt.Errorf("mutator function failed: %w", err)
		}

		// 3. Stage changes
		worktree, err := g.repo.Worktree()
		if err != nil {
			return fmt.Errorf("get worktree failed: %w", err)
		}

		if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
			return fmt.Errorf("git add failed: %w", err)
		}

		status, err := worktree.Status()
		if err != nil {
			return fmt.Errorf("git status failed: %w", err)
		}

		if status.IsClean() {
			g.markSynced()
			return nil
		}

		// 4. Commit
		_, err = worktree.Commit(message, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Recipe Detection",
				Email: "recipe-detection@hpe.com",
				When:  time.Now(),
			},
		})
		if err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}

		// 5. Push to remote
		if g.token == "" {
			slog.Info("GIT_TOKEN not configured. Changes committed to local git repository; skipping remote push.", "message", message)
			g.markSynced()
			return nil
		}

		err = g.repo.PushContext(ctx, &git.PushOptions{
			Auth: g.auth(),
		})
		if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
			g.markSynced()
			return nil
		}

		if attempt >= maxPushAttempts {
			return fmt.Errorf("git push rejected after %d attempts: %w", attempt, err)
		}
		slog.Warn("Git push rejected, retrying mutation", "attempt", attempt, "err", err)
	}
	return nil
}

// GenerateAndPushRelease writes the Helm values file and updates Chart.yaml, then commits and pushes.
func (g *gitOpsClient) GenerateAndPushRelease(ctx context.Context, release *model.HelmRelease) (string, error) {
	valuesFileName := ResolveValuesFileName(release)

	err := g.Mutate(ctx, "Release v"+release.Version, func() error {
		// 1. Generate values YAML
		valuesYAML, err := GenerateValuesYAML(release, valuesFileName)
		if err != nil {
			return fmt.Errorf("generate values YAML failed: %w", err)
		}

		targetDir := filepath.Join(g.localPath, g.valuesDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("mkdir values dir failed: %w", err)
		}

		valuesFilePath := filepath.Join(targetDir, valuesFileName)
		if err := os.WriteFile(valuesFilePath, []byte(valuesYAML), 0644); err != nil {
			return fmt.Errorf("write values file failed: %w", err)
		}

		// 2. Update Chart.yaml metadata
		chartFilePath := filepath.Join(targetDir, "Chart.yaml")
		if _, err := os.Stat(chartFilePath); err == nil {
			if err := UpdateChartMetadata(chartFilePath, release.Version, valuesFileName); err != nil {
				return fmt.Errorf("update Chart.yaml failed: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}
	return valuesFileName, nil
}

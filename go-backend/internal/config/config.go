package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config represents all application configuration options.
type Config struct {
	Server     ServerConfig
	GitOps     GitOpsConfig
	Promotion  PromotionConfig
	Jenkins    JenkinsConfig
	Kubernetes KubernetesConfig
}

type ServerConfig struct {
	Port        int
	ContextPath string
}

type GitOpsConfig struct {
	RepoURL             string
	LocalPath           string
	Branch              string
	Username            string
	Token               string
	ValuesDir           string
	StateCacheTTL       time.Duration
	StateCacheTTLSecond int
}

type PromotionConfig struct {
	Pipeline []string
}

type JenkinsConfig struct {
	URL      string
	Job      string
	Username string
	Token    string
}

type KubernetesConfig struct {
	Clusters map[string]string // cluster -> context
}

// Load loads configuration from environment variables with sensible defaults matching application.yml.
func Load() *Config {
	loadDotEnv()
	port := getEnvInt("SERVER_PORT", getEnvInt("PORT", 8081))
	contextPath := getEnv("SERVER_CONTEXT_PATH", "/api")

	defaultLocalPath := detectWorkspaceRoot()

	cacheTTLSec := getEnvInt("GITOPS_STATE_CACHE_TTL_SECONDS", 8)

	pipelineRaw := getEnv("PROMOTION_PIPELINE", "dev,qa,integration,prod")
	pipeline := parseCommaList(pipelineRaw)

	return &Config{
		Server: ServerConfig{
			Port:        port,
			ContextPath: contextPath,
		},
		GitOps: GitOpsConfig{
			RepoURL:             getEnv("GITOPS_REPO_URL", "https://github.com/JdVashuu/HPE-GitOps-Release.git"),
			LocalPath:           getEnv("GITOPS_LOCAL_PATH", defaultLocalPath),
			Branch:              getEnv("GITOPS_BRANCH", "main"),
			Username:            getEnv("GIT_USERNAME", "TasteTheThunder"),
			Token:               getEnv("GIT_TOKEN", ""),
			ValuesDir:           getEnv("GITOPS_VALUES_DIR", "helm/recipe-detection-chart"),
			StateCacheTTL:       time.Duration(cacheTTLSec) * time.Second,
			StateCacheTTLSecond: cacheTTLSec,
		},
		Promotion: PromotionConfig{
			Pipeline: pipeline,
		},
		Jenkins: JenkinsConfig{
			URL:      getEnv("JENKINS_URL", "http://localhost:8080"),
			Job:      getEnv("JENKINS_JOB", "hpe-recipe-final"),
			Username: getEnv("JENKINS_USER", "thatoneuke"),
			Token:    getEnv("JENKINS_TOKEN", ""),
		},
		Kubernetes: KubernetesConfig{
			Clusters: map[string]string{
				"dev":         "dev",
				"qa":          "qa",
				"integration": "integration",
				"prod":        "prod",
			},
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return i
		}
	}
	return defaultVal
}

func parseCommaList(val string) []string {
	parts := strings.Split(val, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func detectWorkspaceRoot() string {
	dir, err := os.Getwd()
	if err == nil {
		for {
			if _, err := os.Stat(filepath.Join(dir, "catalogs", "recipe-detection")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return filepath.Join(os.TempDir(), "hpe-recipe-workspace")
}

func loadDotEnv() {
	roots := []string{".env", "../.env"}
	ws := detectWorkspaceRoot()
	if ws != "" {
		roots = append(roots, filepath.Join(ws, ".env"), filepath.Join(ws, "go-backend", ".env"))
	}
	for _, p := range roots {
		data, err := os.ReadFile(p)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.TrimSpace(parts[1])
					v = strings.Trim(v, `"'`)
					if _, exists := os.LookupEnv(k); !exists {
						_ = os.Setenv(k, v)
					}
				}
			}
			return
		}
	}
}

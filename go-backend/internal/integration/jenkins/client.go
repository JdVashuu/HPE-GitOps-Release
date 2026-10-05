package jenkins

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BuildParams contains the parameters passed to the parameterized Jenkins job.
type BuildParams struct {
	Cluster           string
	Action            string // "deploy" or "uninstall"
	ChartVersion      string
	ValuesFile        string
	DeployEventAction string // "create_deploy", "deploy", "promote", "edit", "rollback"
	FromVersion       string
}

// Client defines the interface for interacting with Jenkins.
type Client interface {
	Trigger(ctx context.Context, params BuildParams) error
}

type jenkinsClient struct {
	url        string
	job        string
	username   string
	token      string
	httpClient *http.Client
}

// NewClient initializes a Jenkins client.
func NewClient(jenkinsURL, job, username, token string) Client {
	return &jenkinsClient{
		url:      strings.TrimRight(jenkinsURL, "/"),
		job:      job,
		username: username,
		token:    token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type crumbResponse struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

func (j *jenkinsClient) getCrumb(ctx context.Context, authHeader string) (field, crumb string, err error) {
	crumbURL := fmt.Sprintf("%s/crumbIssuer/api/json", j.url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crumbURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create crumb request failed: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch crumb failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("crumb request returned %d: %s", resp.StatusCode, string(body))
	}

	var cr crumbResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", "", fmt.Errorf("decode crumb response failed: %w", err)
	}
	return cr.CrumbRequestField, cr.Crumb, nil
}

func (j *jenkinsClient) Trigger(ctx context.Context, params BuildParams) error {
	if j.username == "" || j.token == "" {
		slog.Warn("Jenkins credentials not configured (JENKINS_USER/JENKINS_TOKEN). Skipping build trigger (standalone mode).",
			"cluster", params.Cluster, "action", params.Action, "valuesFile", params.ValuesFile)
		return nil
	}

	auth := base64.StdEncoding.EncodeToString([]byte(j.username + ":" + j.token))
	authHeader := "Basic " + auth

	crumbField, crumb, err := j.getCrumb(ctx, authHeader)
	if err != nil {
		slog.Warn("Could not fetch Jenkins crumb (CSRF might be disabled)", "err", err)
	}

	endpoint := fmt.Sprintf("%s/job/%s/buildWithParameters", j.url, url.PathEscape(j.job))
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid Jenkins URL: %w", err)
	}

	q := u.Query()
	q.Set("CLUSTER", params.Cluster)
	q.Set("ACTION", params.Action)
	q.Set("ALLOW_DEPLOY", "yes")

	if params.ChartVersion != "" {
		q.Set("CHART_VERSION", params.ChartVersion)
	}
	if params.ValuesFile != "" {
		q.Set("VALUES_FILE", params.ValuesFile)
	}
	if params.DeployEventAction != "" {
		q.Set("DEPLOY_EVENT_ACTION", params.DeployEventAction)
	}
	if params.FromVersion != "" {
		q.Set("FROM_VERSION", params.FromVersion)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create Jenkins trigger request failed: %w", err)
	}

	req.Header.Set("Authorization", authHeader)
	if crumbField != "" && crumb != "" {
		req.Header.Set(crumbField, crumb)
	}

	slog.Info("Triggering Jenkins build", "url", u.String(), "cluster", params.Cluster, "action", params.Action)

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("trigger Jenkins job failed: %w", err)
	}
	defer resp.Body.Close()

	if (resp.StatusCode < 200 || resp.StatusCode >= 400) && resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jenkins trigger failed with status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("Jenkins build triggered successfully", "cluster", params.Cluster, "status", resp.StatusCode)
	return nil
}

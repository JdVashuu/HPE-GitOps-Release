# HPE Recipe Detection - Go Backend

An idiomatic **Go 1.24+** backend implementing the HPE Recipe Detection GitOps catalog and release management architecture using the standard library `net/http` router, pure GitOps file-based persistence (no external database or Redis), and Jenkins deployment orchestration.

## Architecture Highlights

1. **Pure GitOps Persistence**:
   - Platform state, environment placements, and deployment histories are persisted as declarative YAML files in Git under `catalogs/recipe-detection/`.
   - Read requests are accelerated by a thread-safe in-memory cache with configurable TTL (`GITOPS_STATE_CACHE_TTL_SECONDS`).
2. **Unified Git Workspace**:
   - Consolidates the separate clones from the Java version into a single managed workspace with optimistic locking and push retry loops via `go-git/v5`.
3. **Standard Library `net/http` Router**:
   - Uses Go's enhanced `http.ServeMux` pattern matching (`r.PathValue(...)`) with panic recovery, CORS, and `slog` structured request logging.
4. **Real-time WebSockets**:
   - Full-duplex WebSocket multicasting at `/api/ws/releases` for real-time status updates (`deploying`, `deployed`, `failed`) and catalog events.
5. **Exact REST & Model Parity**:
   - 100% wire-compatible with the Java Spring Boot API, matching all endpoints, query parameters, and JSON/YAML serialization schemas.

## Project Structure

```text
go-backend/
├── cmd/
│   └── recipe-api/
│       └── main.go                         # Service entrypoint
├── internal/
│   ├── app/
│   │   └── app.go                          # Server lifecycle & dependency injection
│   ├── config/
│   │   └── config.go                       # Environment variable configuration loader
│   ├── controller/
│   │   ├── catalog_controller.go           # /pipeline, /environments, /versions, /history
│   │   ├── release_controller.go           # /helm-releases, /compare, /deploy-preview
│   │   ├── recipe_controller.go            # /recipes sub-resources
│   │   ├── health_controller.go            # /health and /actuator/health
│   │   ├── ws_controller.go                # /ws/releases WebSocket handler
│   │   ├── router.go                       # http.ServeMux route registration
│   │   └── helper.go                       # JSON & param helpers
│   ├── middleware/
│   │   ├── cors.go                         # Cross-Origin Resource Sharing
│   │   ├── logger.go                       # slog request logging
│   │   └── recovery.go                     # Panic recovery
│   ├── model/
│   │   ├── component.go                    # ComponentSpec domain model
│   │   ├── recipe.go                       # Recipe domain model
│   │   ├── release.go                      # HelmRelease & ReleaseSummary models
│   │   ├── environment.go                  # EnvironmentFile & PromotionOptions models
│   │   ├── audit.go                        # AuditLogEntry model
│   │   ├── preview.go                      # DeployPreview & VersionComparison models
│   │   └── websocket.go                    # WebSocket event envelope
│   ├── repository/
│   │   ├── catalog_repository.go           # Repository interfaces
│   │   ├── file/
│   │   │   └── yaml_repo.go                # Disk YAML reader/writer
│   │   └── cache/
│   │       └── memory_cache.go             # Thread-safe in-memory TTL cache & lock
│   ├── service/
│   │   ├── platform_service.go             # Catalog promotion, rollback, edit, lifecycle
│   │   ├── release_service.go              # Diff comparison, deploy preview, compatibility
│   │   ├── event_service.go                # WebSocket broadcast coordination
│   │   └── semver.go                       # Version normalization and patch incrementing
│   └── integration/
│       ├── gitops/
│       │   ├── client.go                   # go-git client with push-retry loop
│       │   ├── values_generator.go         # values-v<version>.yaml generator
│       │   └── chart_mutator.go            # Chart.yaml version/annotation mutator
│       ├── jenkins/
│       │   └── client.go                   # Jenkins CSRF crumb & build trigger client
│       └── websocket/
│           └── hub.go                      # Gorilla WebSocket connection hub
├── go.mod
└── go.sum
```

## Configuration

The Go service reads the following environment variables (matching `application.yml`):

| Variable | Description | Default |
| :--- | :--- | :--- |
| `SERVER_PORT` / `PORT` | HTTP server port | `8081` |
| `SERVER_CONTEXT_PATH` | Base context path | `/api` |
| `GIT_USERNAME` | GitHub username | `TasteTheThunder` |
| `GIT_TOKEN` | GitHub personal access token | `""` |
| `GITOPS_REPO_URL` | Remote repository clone URL | `https://github.com/TasteTheThunder/hpe-recipe-final.git` |
| `GITOPS_LOCAL_PATH` | Local Git clone workspace path | `/tmp/hpe-recipe-workspace` |
| `GITOPS_BRANCH` | Target Git branch | `main` |
| `GITOPS_VALUES_DIR` | Relative path to Helm chart | `helm/recipe-detection-chart` |
| `GITOPS_STATE_CACHE_TTL_SECONDS` | In-memory read cache TTL | `8` |
| `PROMOTION_PIPELINE` | Comma-separated promotion pipeline | `dev,qa,integration,prod` |
| `JENKINS_URL` | Jenkins instance base URL | `http://localhost:8080` |
| `JENKINS_JOB` | Parameterized Jenkins job name | `hpe-recipe-final` |
| `JENKINS_USER` | Jenkins username | `thatoneuke` |
| `JENKINS_TOKEN` | Jenkins API token | `""` |

## Run Locally

### 1. Build and Run Directly

```bash
cd go-backend
go run ./cmd/recipe-api
```

Or compile a binary:

```bash
cd go-backend
go build -o bin/recipe-api ./cmd/recipe-api
./bin/recipe-api
```

### 2. Run with Local Workspace Override

If you are developing locally and want the Go backend to inspect the current Git repository directly without cloning into `/tmp`:

```bash
export GITOPS_LOCAL_PATH="$(pwd)/.."
cd go-backend
go run ./cmd/recipe-api
```

### 3. Run Unit & Integration Tests

```bash
cd go-backend
go test -v ./...
```

## API Endpoints

### Platform Management
- `GET /api/pipeline`: Pipeline stages (`["dev", "qa", "integration", "prod"]`)
- `GET /api/environments`: Active version per environment (`{"dev": "2.1.3", "qa": "2.1.2"}`)
- `GET /api/versions`: All catalog versions (`["2.1.1", "2.1.2", "2.1.3"]`)
- `GET /api/versions/{version}`: Details of a catalog version
- `GET /api/versions/{version}/promotion-options`: Targets and rollback availability
- `POST /api/versions?deployToDev=true`: Create first catalog version
- `POST /api/versions/{version}/deploy`: Deploy version to DEV
- `POST /api/versions/{version}/promote?to={env}`: Promote version to next stage
- `POST /api/environments/{env}/rollback`: Roll back environment to last known good state
- `POST /api/catalog/edit`: Fork new patch version from active DEV catalog
- `DELETE /api/versions/{version}`: Uninstall and delete a version
- `GET /api/history`: Deployment event audit log
- `DELETE /api/history`: Clear event audit log

### Helm Releases
- `GET /api/helm-releases?cluster={env}`: List releases for cluster
- `GET /api/helm-releases/{version}?cluster={env}`: Release details
- `PUT /api/helm-releases/{version}/status?cluster={env}`: Jenkins deployment callback
- `POST /api/helm-releases/{version}/deploy?cluster={env}`: Deploy or promote release
- `GET /api/helm-releases/{version}/deploy-preview?cluster={env}&baseline=auto`: Release diff preview
- `GET /api/helm-releases/compare?cluster={env}&from={v1}&to={v2}`: Version comparison diff

### Recipes & WebSockets
- `GET /api/recipes/{recipeVersion}/components`: Component specifications
- `GET /api/recipes/{recipeVersion}/upgradePaths`: Upgrade pathways
- `GET /api/health`: Health probe (`{"status":"UP","service":"recipe-detection-api"}`)
- `GET /api/actuator/health`: Actuator health probe (`{"status":"UP"}`)
- `GET /api/ws/releases`: WebSocket connection for real-time status broadcasts

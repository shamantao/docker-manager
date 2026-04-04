# Docker Manager Architecture (Dev Guide)

This document explains the code structure so contributors can extend the app.
It is the technical companion to [README.md](../README.md), which stays focused on user-facing behavior, installation, and CLI/TUI usage.

Scope of this document:
- internal module layout
- execution flow
- technical decisions
- extension points for contributors

To avoid duplication, user workflows and end-user examples should stay in [README.md](../README.md).

## Project Layout

```
docker-manager/
├── main.go                 # CLI entrypoint
├── go.mod                  # Go dependencies
├── Makefile                # Build/install
├── README.md               # User guide
└── pkg/
    ├── discovery/          # Project discovery
    ├── docker/             # Docker/Compose wrapper
    ├── config/             # Optional YAML config
    ├── project/            # Data structures
    └── tui/                # Bubble Tea dashboard
```

## Execution Flow

```
main.go
  └─ parse arguments
  └─ handle*() function
      ├─ discovery.DiscoverInDefaultPath()
      ├─ locate target project
      └─ docker.Manager executes the action
```

Example:

```go
// main.go - handleStart()
1. docker.EnsureDockerRunning()
2. projects := discovery.DiscoverInDefaultPath()
3. find project by name
4. mgr := docker.NewManager(project.Path)
5. mgr.StartProjectStream(&project, outputChan)
```

## Modules

### 1) pkg/project

```go
type Project struct {
    Name         string
    Path         string
    ComposePath  string
    Services     []Service
    Running      bool
    ServiceCount int
    Orphan       bool
}
```

### 2) pkg/discovery

```go
func DiscoverInDefaultPath() ([]project.Project, error)
// Combines:
// 1) auto-discovery from configured roots
// 2) explicit projects from config
```

**Configuration priority:**

1. `DOCKER_MANAGER_ROOT` environment variable (highest priority)
2. `roots` field in `~/.docker-manager/projects.yml`
3. `root` field in `~/.docker-manager/projects.yml` (backward compatibility)

Auto-discovered entries are deduplicated by project name.
Explicit config entries override auto-discovered path when names collide.

### 3) pkg/docker

Key APIs:

```go
func (m *Manager) StartProjectStream(p *project.Project, output chan<- string) error
func (m *Manager) StopProjectStream(p *project.Project, output chan<- string) error
func (m *Manager) RestartServiceStream(p *project.Project, output chan<- string, service string) error
func (m *Manager) StopOrphanProjectStream(p *project.Project, output chan<- string) error
func (m *Manager) DiscoverOrphanProjects(knownNames map[string]bool) ([]project.Project, error)
func (m *Manager) GetStatus(p *project.Project) (bool, int, error)
func (m *Manager) GetStatusDetailed(p *project.Project) (bool, int, string)
func (m *Manager) GetServiceURLs(p *project.Project) (map[string][]string, error)
```

Implementation notes:
- Compose is executed through CLI for compatibility.
- If `.env` exists, commands are run through `bash` with `source .env`.
- `--env-file /dev/null` prevents Compose from reparsing `.env` with limited interpolation support.
- Streaming actions combine stdout/stderr and forward output lines to TUI/CLI.

### 4) pkg/config

YAML config file at `~/.docker-manager/projects.yml`.

**Auto-initialization**: On first launch, `config.EnsureDefaultConfig()` creates:

```yaml
projects: {}
```

Supported fields:

```yaml
root: /legacy/single/root            # optional (backward compat)
roots:                               # preferred
    - /path/one
    - /path/two
projects:
  example:
        path: /abs/path/to/docker-example
```

### 5) pkg/tui

Bubble Tea model for dashboard interactions.

Current behavior:
- async start/stop/restart operations
- spinner while operation is running
- live output tail (`maxLogLines`) from Docker commands
- key locking during running operations (except quit)
- orphan project handling (shown with `Orphan=true`)

## Notes

- Project names are normalized to lowercase for Docker Compose compatibility.
- `status <project>` uses Docker labels to extract exposed ports and prints `http://localhost:<port>`.
- `status` (global) appends an orphan section when running containers are outside configured projects.

---

### 6. `main.go`
Entrypoint and command routing:

```go
func main()
    // Parse os.Args
    // Switch sur la commande
    // Appelle le bon function handle*()

func handleStart(projectName string)
    // 1. check Docker daemon
    // 2. discover projects
    // 3. locate target project
    // 4. run streamed action through docker.Manager

// same orchestration pattern for stop/restart/status/logs/dashboard
```

**À modifier si :**
- Tu veux ajouter une commande (ex: `export`, `import`, `backup`)
- Tu veux un vrai CLI parser (urfave/cli, cobra, etc.)
- Tu veux des configurations globales (`--debug`, `--config`)

---

## External dependencies

```go
import (
    "flag"
    "os/exec"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/log"

    "gopkg.in/yaml.v3"
)
```

These dependencies are compile-time/runtime inside the binary; deployment remains a single executable.

---

## How to add a command

Minimal checklist:
- add a `case` in `main()` command switch
- add `handleXxx(...)` orchestration in `main.go`
- add manager method(s) in `pkg/docker` if Docker actions are needed
- update help output in `printHelp()`
- update [README.md](../README.md) for functional behavior
- update this file only for technical changes

## Logs and debugging

Logging backend:
```go
logger := log.New(os.Stderr)  // charmbracelet/log
logger.Fatal(err)
```

Quick debug loop:
```bash
# add debug points
logger.Debug("Ma variable:", myVar)

# compile and run
make build
./docker-manager start pbwww
```

---

## Test and validation

```bash
# rebuild
make build

# local check
./docker-manager status

# install global binary
make install
docker-manager status
```

---

## Architecture decisions

### Pourquoi pas d'API Docker SDK ?
- ✅ Plus simple d'utiliser le CLI docker-compose
- ✅ Compatible avec tous les docker-compose versions
- ✅ Moins de dépendances
- ❌ Moins de contrôle fine-grained

### Pourquoi Go ?
- ✅ Executable unique, zéro dépendances runtime
- ✅ Ultra-rapide sur M1
- ✅ Perfect pour les CLI tools
- ✅ Cross-plateforme facile

### Pourquoi Bubble Tea pour TUI ?
- ✅ simple async event model for terminal UI
- ✅ mature ecosystem (spinner, styling)
- ✅ easy integration with streamed command output

---

## Improvement ideas

1. **Health Checks**
   ```bash
   docker-manager health pbwww
   ```

2. **Config Profiles**
   ```bash
   docker-manager start pbwww --profile prod
   ```

3. **Multi-Docker Support**
   ```bash
   docker-manager list-all-hosts
   docker-manager start pbwww --host remote-server
   ```

4. **Persistent Metrics**
   - Tracker le uptime de chaque service
   - Exporter en JSON pour monitoring

5. **Web UI (Go http server)**
   - Si TUI n'est pas assez pour toi

6. **Webhook Notifications**
   - Slack/Discord quand un service crash

---

## Maintainer note

When behavior is user-visible, document it in [README.md](../README.md).
When behavior is implementation-specific, document it here.

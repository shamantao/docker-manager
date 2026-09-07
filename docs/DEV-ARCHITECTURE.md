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

    // Docker inventory (filled from container labels)
    Source       string   // config | scan | docker
    Standalone   bool     // plain "docker run" container
    Containers   []string
    RunningCount int      // containers currently up
    TotalCount   int      // containers that exist (running + stopped)
    Images       []string
}
```

`StatusString()` derives four states from these counters: running, partial
(`RunningCount < TotalCount`), installed-but-stopped (`TotalCount > 0`,
`RunningCount == 0`), and stopped (no container at all).
`Manageable()` is true when a compose file exists **or** containers exist.

### 2) pkg/discovery

```go
func DiscoverAll(mgr *docker.Manager) ([]project.Project, error)
// Filesystem projects merged with Docker's own inventory. This is the entry
// point used by every command.

func DiscoverInDefaultPath() ([]project.Project, error)
// Filesystem only:
// 1) scan of the roots (2 levels deep, any compose file name)
// 2) explicit projects from config

func EffectiveRoots() []string
// Roots actually scanned on this machine (existing ones only)
```

**Root priority:**

1. `DOCKER_MANAGER_ROOT` environment variable (highest priority)
2. `roots` field in `~/.docker-manager/projects.yml`
3. `root` field in `~/.docker-manager/projects.yml` (backward compatibility)
4. `defaultRoots()` — built-in common locations, used when none of the
   configured roots exists on this machine. This is what makes a config file
   copied from another machine harmless.

`matchProject()` merges an inventory entry into the filesystem list by cleaned
path first, then by exact name, then by name without the `docker-` prefix
(compose names a project after its directory, the tool strips the prefix).
When the recorded compose file does not exist here but Docker knows a valid one,
Docker's path wins — the same config then works on a different machine.

### 3) pkg/docker

Key APIs:

```go
func (m *Manager) StartProjectStream(p *project.Project, output chan<- string) error
func (m *Manager) StopProjectStream(p *project.Project, output chan<- string) error
func (m *Manager) RestartServiceStream(p *project.Project, output chan<- string, service string) error
func (m *Manager) UpdateProjectStream(p *project.Project, output chan<- string) error

// Projects without a compose file (docker run, or compose file absent here)
func (m *Manager) StartContainersStream(p *project.Project, output chan<- string) error
func (m *Manager) StopContainersStream(p *project.Project, output chan<- string) error
func (m *Manager) RestartContainersStream(p *project.Project, output chan<- string) error
func (m *Manager) GetContainerLogs(p *project.Project, follow bool) error

// Inventory (inventory.go)
func (m *Manager) Inventory() ([]project.Project, error)
func ComposeCommand() []string           // ["docker","compose"] or ["docker-compose"]
func FindComposeFile(dir string) string  // docker-compose.yml|yaml, compose.yml|yaml

func (m *Manager) GetStatus(p *project.Project) (bool, int, error)
func (m *Manager) GetStatusDetailed(p *project.Project) (bool, int, string)
func (m *Manager) GetServiceURLs(p *project.Project) (map[string][]string, error)
```

Implementation notes:
- The compose binary is detected once at runtime (`ComposeCommand()`): Compose v2
  plugin is preferred, v1 `docker-compose` is the fallback. Nothing is hardcoded,
  which is what allows the same binary to work on a Mac and on a Raspberry Pi.
- `composeExec()` centralises command construction (compose file name, `-p`, `.env`).
- `Inventory()` parses `docker ps -a` including the compose labels; the parsing is
  split into pure functions (`parseContainerRows`, `buildInventory`) so it is unit
  tested without a running daemon (`pkg/docker/inventory_test.go`).
- Non-compose actions use `docker start|stop|restart` and never `docker rm`: a
  stopped container stays visible and restartable.
- `UpdateProjectStream` is deliberately limited to `pull` on images the project
  already references — no new image is ever fetched.
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
- async start/stop/restart/update operations
- action dispatch depends on `DockerComposeExists()`: compose stream or plain
  container stream
- spinner while operation is running
- live output tail (`maxLogLines`) from Docker commands
- key locking during running operations (except quit)
- `SetRefresh()` injects an inventory reload closure (used by `R` and after every
  completed operation), which keeps `pkg/tui` free of a dependency on `pkg/discovery`

## Notes

- Project names are normalized to lowercase for Docker Compose compatibility.
- `status <project>` uses Docker labels to extract exposed ports and prints `http://localhost:<port>`.
- `status` (global) lists everything discovered on the machine, including stopped
  containers, with a legend for the state markers.
- Tests: `go test ./...` covers row parsing, inventory grouping, compose-path
  resolution, the filesystem scan, and project matching — all without Docker.

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

### Compatibilite Docker Compose plugin

Contexte:
- Sur certaines distributions recentes, seule la commande `docker compose` est disponible.
- Le binaire `docker-compose` peut etre absent du `PATH`.

Symptome observe:
- Echec de `start`/`dashboard` avec:
    - `exec: "docker-compose": executable file not found in $PATH`

Workaround operationnel (en attendant un correctif dans les sources):

```bash
cat > /tmp/docker-compose <<'EOF'
#!/usr/bin/env sh
exec docker compose "$@"
EOF
chmod 755 /tmp/docker-compose
sudo install -m 0755 /tmp/docker-compose /usr/local/bin/docker-compose
rm -f /tmp/docker-compose
docker-compose version
```

Validation:

```bash
docker-manager daemon status
docker-manager start <project>
docker-manager status <project>
```

Note TTY:
- Des artefacts de rendu terminal (ex: `;1R`) peuvent apparaitre selon l'emulateur.
- Cela vient des sequences de controle TTY et n'indique pas necessairement une erreur metier.
- Les commandes non interactives (`status`, `start`, `logs`) restent la reference de verification.

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

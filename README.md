# Docker Manager

A fast, practical Docker project manager with a CLI and a small TUI dashboard.

This README is the functional user guide.
For internal code structure and implementation details, see [docs/DEV-ARCHITECTURE.md](docs/DEV-ARCHITECTURE.md).

## Features

- **Zero-config discovery**: finds projects on any machine, from Docker's own labels
- Shows **running**, **partially running**, and **installed-but-stopped** containers
- Manages both `docker compose` projects and plain `docker run` containers
- Fast CLI: add, remove, list, start, stop, restart, update, status, logs
- Detailed status for a single project (services + URLs)
- Interactive TUI dashboard with spinner + live operation logs
- Docker daemon management (start, stop, status)
- Runs on macOS, Linux, and Raspberry Pi (arm64 / armv7)

## Requirements

- Docker Engine or Docker Desktop
- Docker Compose v2 (`docker compose`) or v1 (`docker-compose`) — auto-detected

## Install

On the machine itself (compiles if Go is available, otherwise uses a prebuilt binary):

```bash
./install.sh
```

Deploying to a Raspberry Pi from your dev machine (no Go needed on the Pi):

```bash
make release                                   # builds every target
scp docker-manager-linux-arm64 install.sh pi@raspberry:~/
ssh pi@raspberry "./install.sh"                # picks the matching binary
```

`install.sh` detects OS/architecture (`darwin`/`linux`, `arm64`/`amd64`/`arm`),
installs into `/usr/local/bin` (override with `PREFIX=`), and warns if the Docker
daemon is unreachable.

## Usage

### CLI

```bash
# Global status
docker-manager status

# Registered projects from config
docker-manager list

# Add/remove explicit project entries
docker-manager add ~/docker/docker-pbwww
docker-manager remove pbwww

# Detailed status for one project
docker-manager status pbwww

# Start (build + up)
docker-manager start pbwww

# Stop (down + remove containers)
docker-manager stop pbwww

# Fast restart (no rebuild)
docker-manager restart pbwww nginx

# Update images already used by the project, then recreate
docker-manager update pbwww

# Logs (use -f for follow)
docker-manager logs pbwww
docker-manager logs pbwww nginx -f

# Docker daemon management
docker-manager daemon status         # Check daemon status
docker-manager daemon start          # Start Docker daemon
docker-manager daemon stop           # Stop Docker daemon
```

### Dashboard (TUI)

```bash
docker-manager dashboard
```

Keys:
- `↑/↓` or `k/j`: navigate
- `s`: start
- `d`: stop (compose `down`, or `docker stop` for standalone containers)
- `r`: restart
- `u`: update images (pull + recreate)
- `R`: reload the inventory
- `q`: quit

During long operations (image pull/build/up/down), the dashboard shows:
- an animated spinner
- live output lines from Docker Compose
- final success/error status

Status markers:

| Marker | Meaning |
| --- | --- |
| `▶ Running` | all containers up |
| `◐ Partiel` | some containers up, some stopped |
| `⏸ Installé, arrêté` | containers exist on the machine but are stopped |
| `⏹ Stopped` | project found on disk, no container created |
| `👻` | not in config — located through Docker |
| `⬦` | plain container, not managed by docker-compose |

## Project discovery

Discovery combines two independent sources and merges them, so moving the binary
to another machine requires no configuration at all.

**1. Docker's own inventory** (`docker ps -a`). Compose stamps every container it
creates with `com.docker.compose.project`, `...project.working_dir` and
`...project.config_files`. Reading those labels tells Docker Manager which
projects exist, where their compose file lives, and how many containers are
running versus stopped. Stopped containers are included — that is how
"installed but not started" shows up. Containers started with `docker run` have
no such labels and are listed individually.

**2. A filesystem scan** of root directories (2 levels deep), which also catches
projects whose containers have never been created:

1. `DOCKER_MANAGER_ROOT` environment variable
2. `roots` list in `~/.docker-manager/projects.yml`
3. `root` in `~/.docker-manager/projects.yml` (backward compatibility)
4. built-in defaults when nothing above exists on this machine:
   `~/docker`, `~/dockers`, `~/stacks`, `~/containers`, `~/compose`, `~`,
   `/opt/docker`, `/opt/stacks`, `/opt/containers`, `/srv/docker`, `/srv`,
   `/volume1/docker`, `/home/pi/docker`

A folder is a project if it contains `docker-compose.yml`, `docker-compose.yaml`,
`compose.yml` or `compose.yaml`. The `docker-` prefix is no longer required.
Hidden folders, `node_modules` and similar are skipped.

Entries are merged by path first, then by name (tolerating the `docker-` prefix).
Roots that do not exist on the current machine are silently ignored, and if a
copied config points only at missing paths, the built-in defaults take over.

## Updating images

`docker-manager update <project>` runs `compose pull` followed by `compose up -d`.
It only refreshes images the project already references — it never downloads an
image that is not already part of the project. For a standalone container, the
image is pulled and you are told to recreate the container to apply it.

## Detailed status URLs

`docker-manager status <project>` prints local URLs derived from published ports.

Example output:

```
URLs:
  - nginx => http://localhost:80
  - open-webui => http://localhost:3000
```

## Configuration (optional)

At first launch, Docker Manager creates a default config file at:

```
~/.docker-manager/projects.yml
```

This file contains:

```yaml
roots:
  - /home/you/docker
projects: {}
```

### Discovery roots

Edit `~/.docker-manager/projects.yml`:

```yaml
roots:
  - /path/to/docker/projects
  - /another/path
```

Temporary override from shell:

```bash
export DOCKER_MANAGER_ROOT=/path/to/your/docker/projects
docker-manager status
```

`DOCKER_MANAGER_ROOT` overrides config roots for that command session.

### Explicit project registration

Register a specific compose directory directly:

```bash
docker-manager add /path/to/docker-myproject
docker-manager remove myproject
```

This writes entries under `projects:` in `~/.docker-manager/projects.yml`.

### About `.env` interpolation

If a project has a `.env` file with nested variable interpolation (for example `${BASE}/docker`), Docker Manager resolves it through bash before running Docker Compose.
This supports setups where `.env` is a symlink to a shared secrets file.

## Local development

Technical architecture and contributor notes live in [docs/DEV-ARCHITECTURE.md](docs/DEV-ARCHITECTURE.md).

```bash
make build
make run
make test
make clean
```

## Version

Version is defined in a single source of truth in [main.go](main.go):

```go
const Version = "1.4.0"
```

Use `docker-manager --version` to display it.

## License

MIT

# Docker Manager

A fast, practical Docker project manager with a CLI and a small TUI dashboard.

This README is the functional user guide.
For internal code structure and implementation details, see [docs/DEV-ARCHITECTURE.md](docs/DEV-ARCHITECTURE.md).

## Features

- Auto-discovery of `docker-*` projects from configured roots
- Fast CLI: add, remove, list, start, stop, restart, status, logs
- Detailed status for a single project (services + URLs)
- Interactive TUI dashboard with spinner + live operation logs
- Detection of running containers outside config (shown as orphan entries)
- Docker daemon management (start, stop, status)

## Requirements

- Docker Desktop (or Docker Engine)
- docker-compose v1 or Docker Compose v2

## Install (from source)

```bash
make install
```

This builds and installs `docker-manager` into `/usr/local/bin`.

## Usage

### CLI

```bash
# Global status
docker-manager status

# Registered projects from config
docker-manager list

# Add/remove explicit project entries
docker-manager add ~/kDrive/docker/docker-pbwww
docker-manager remove pbwww

# Detailed status for one project
docker-manager status pbwww

# Start (build + up)
docker-manager start pbwww

# Stop (down + remove containers)
docker-manager stop pbwww

# Fast restart (no rebuild)
docker-manager restart pbwww nginx

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
- `S`: start
- `D`: stop (down)
- `R`: restart
- `Q`: quit

During long operations (image pull/build/up/down), the dashboard shows:
- an animated spinner
- live output lines from Docker Compose
- final success/error status

Orphan running containers (not managed by configured projects) are shown with a `👻` marker and can be stopped from the dashboard.

## Project discovery

Docker Manager can discover projects from roots and explicit config entries.

Discovery order:
1. `DOCKER_MANAGER_ROOT` environment variable (single root override)
2. `roots` list in `~/.docker-manager/projects.yml`
3. `root` in `~/.docker-manager/projects.yml` (backward compatibility)
4. explicit `projects` entries added with `docker-manager add`

Auto-discovered folders must match:

- name starts with `docker-`
- contains a `docker-compose.yml`

Project names are normalized to lowercase for Docker Compose compatibility.

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
  - /Users/you/kDrive/docker
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
const Version = "1.3.0"
```

Use `docker-manager --version` to display it.

## License

MIT

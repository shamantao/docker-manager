# Docker Manager

A fast, practical Docker project manager with a CLI and a small TUI dashboard.

## Features

- Auto-discovery of `docker-*` projects
- Fast CLI: start, stop, restart, status, logs
- Detailed status for a single project (services + URLs)
- Interactive TUI dashboard
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

## Project discovery

Docker Manager scans a single root directory and picks any folder that matches:

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
root: /home/yourname/docker
projects: {}
```

### Changing the root directory

**Option 1: Edit the config file** (recommended)

```yaml
root: /path/to/your/docker/projects
```

**Option 2: Environment variable** (temporary override)

```bash
export DOCKER_MANAGER_ROOT=/path/to/your/docker/projects
docker-manager status
```

The environment variable takes precedence over the file.

### Project-specific settings

You can add health checks or custom settings per project:

```yaml
root: /home/yourname/docker
projects:
  example:
    path: ./docker-example
    services:
      - name: web
        health_check: "curl -f http://localhost"
```

## Local development

```bash
make build
make run
make test
make clean
```

## License

MIT

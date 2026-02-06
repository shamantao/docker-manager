# Docker Manager

A fast, practical Docker project manager with a CLI and a small TUI dashboard.

## Features

- Auto-discovery of `docker-*` projects
- Fast CLI: start, stop, restart, status, logs
- Detailed status for a single project (services + URLs)
- Interactive TUI dashboard

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

## Local development

```bash
make build
make run
make test
make clean
```

## License

MIT
- [ ] Export des logs en JSON
- [ ] Intégration avec les registries privées
- [ ] Profils docker-compose
- [ ] Multi-user support

---

**Happy Docker managing! 🚀**

# Docker Manager Architecture (Dev Guide)

This document explains the code structure so contributors can extend the app.

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
5. mgr.StartProject(&project)
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
}
```

### 2) pkg/discovery

```go
func DiscoverInDefaultPath() ([]project.Project, error)
// Scans a root folder and returns projects
// A project is a folder that starts with "docker-" and contains docker-compose.yml
```

**Configuration priority:**

1. `DOCKER_MANAGER_ROOT` environment variable (highest priority)
2. `root` field in `~/.docker-manager/projects.yml`
3. Default: `$HOME/docker`

At first launch, Docker Manager creates a default config file with `root: $HOME/docker`.

### 3) pkg/docker

```go
func (m *Manager) StartProject(p *project.Project) error
func (m *Manager) StopProject(p *project.Project) error
func (m *Manager) RestartService(p *project.Project, serviceName string) error
func (m *Manager) GetStatus(p *project.Project) (bool, int, error)
func (m *Manager) GetStatusDetailed(p *project.Project) (bool, int, string)
func (m *Manager) GetServiceURLs(p *project.Project) (map[string][]string, error)
```

All actions are delegated to Docker CLI / Docker Compose for compatibility.

### 4) pkg/config

YAML config file at `~/.docker-manager/projects.yml`.

**Auto-initialization**: On first launch, `config.EnsureDefaultConfig()` creates:

```yaml
root: /home/user/docker
projects: {}
```

Users can then customize:

```yaml
root: /custom/path
projects:
  example:
    path: ./docker-example
    services:
      - name: web
        health_check: "curl -f http://localhost"
```

The `root` field is used by discovery if `DOCKER_MANAGER_ROOT` is not set.

### 5) pkg/tui

Bubble Tea TUI model with a simple list + hotkeys.

## Notes

- Project names are normalized to lowercase for Docker Compose compatibility.
- `status <project>` uses Docker labels to extract exposed ports and prints `http://localhost:<port>`.

---

### 6. `main.go`
Point d'entrée et routage :

```go
func main()
    // Parse os.Args
    // Switch sur la commande
    // Appelle le bon function handle*()

func handleStart(projectName string)
    // Orchestre la séquence
    // 1. Check Docker
    // 2. Découvre projets
    // 3. Trouve le projet
    // 4. Exécute l'action

// Même pattern pour handleStop, handleRestart, etc.
```

**À modifier si :**
- Tu veux ajouter une commande (ex: `export`, `import`, `backup`)
- Tu veux un vrai CLI parser (urfave/cli, cobra, etc.)
- Tu veux des configurations globales (`--debug`, `--config`)

---

## 🔌 Dépendances externes

```go
import (
    "flag"                            // CLI basique
    "os/exec"                         // Exécute docker-compose
    
    tea "github.com/charmbracelet/bubbletea"    // TUI
    "github.com/charmbracelet/lipgloss"         // Formatting
    "github.com/charmbracelet/log"              // Logging
    
    "gopkg.in/yaml.v3"                          // Config YAML
)
```

Toutes ces librairies sont lightweight et n'ont pas de dépendances runtime. L'exécutable est standalone ! 🎯

---

## 🚀 Comment ajouter une fonctionnalité

### Exemple: Ajouter un commande `export` (backup docker-compose)

**1. Ajouter dans main.go :**
```go
case "export":
    if len(os.Args) < 3 {
        fmt.Println("usage: docker-manager export <project> <output-file>")
        os.Exit(1)
    }
    if err := handleExport(os.Args[2], os.Args[3]); err != nil {
        logger.Fatal(err)
    }
```

**2. Implémenter en bas de main.go :**
```go
func handleExport(projectName string, outputFile string) error {
    projects, err := discovery.DiscoverInDefaultPath()
    // ... trouver le projet ...
    
    // Lire le docker-compose.yml
    data, err := os.ReadFile(filepath.Join(p.Path, "docker-compose.yml"))
    
    // Écrire dans le fichier cible
    return os.WriteFile(outputFile, data, 0644)
}
```

**3. Ajouter à l'aide (dans printHelp()) :**
```go
export <project> <file>  Exporte la config docker-compose
```

C'est simple ! La structure est prête pour ça. 💪

---

## 📈 Logs & Debugging

Le code utilise :
```go
logger := log.New(os.Stderr)  // charmbracelet/log
logger.Fatal(err)   // Arrête avec une erreur
```

Pour déboguer :
```bash
# Ajoute du debugging dans le code
logger.Debug("Ma variable:", myVar)

# Compile et exécute
make build
./docker-manager start pbwww
```

---

## 🧪 Tester les changements

```bash
# 1. Modifie le code
vim pkg/docker/docker.go

# 2. Recompile
make build

# 3. Test local
./docker-manager status

# 4. Test depuis partout (après make install)
make install
docker-manager status
```

---

## 🎯 Architecture Decisions

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
- ✅ Beautiful, modern interface
- ✅ Bien maintenu par Charm
- ✅ Pas trop complexe
- ❌ Pas de widgets complexes (mais on n'en a pas besoin)

---

## 🔮 Idées d'amélioration

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

## 📞 Support pour modifications

Si tu veux ajouter quelque chose :
1. Identifie le module (discovery, docker, tui, etc.)
2. Regarde l'interface/signature existante
3. Ajoute ta fonction
4. Test avec `make build`

La structure est prête pour ça ! 🚀

---

**Bon dev ! 💻**

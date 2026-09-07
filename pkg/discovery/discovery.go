package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phil/docker-manager/pkg/config"
	"github.com/phil/docker-manager/pkg/docker"
	"github.com/phil/docker-manager/pkg/project"
)

// maxScanDepth limite la profondeur d'exploration des racines
const maxScanDepth = 2

// skippedDirs ne sont jamais explorées lors du scan
var skippedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "venv": true, "__pycache__": true,
	"Library": true, "Applications": true, "Pictures": true, "Movies": true,
	"Music": true, "proc": true, "sys": true, "dev": true,
}

// Discoverer détecte automatiquement les projets Docker dans un dossier
type Discoverer struct {
	SearchPath string
}

// NewDiscoverer crée un nouveau découvreur
func NewDiscoverer(searchPath string) *Discoverer {
	return &Discoverer{SearchPath: searchPath}
}

// Discover trouve tous les projets Docker sous le répertoire spécifié.
// Un projet = un dossier contenant un fichier compose (docker-compose.yml,
// compose.yaml, ...). Le préfixe "docker-" n'est plus obligatoire.
func (d *Discoverer) Discover() ([]project.Project, error) {
	var projects []project.Project
	if err := scanDir(d.SearchPath, 0, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func scanDir(dir string, depth int, out *[]project.Project) error {
	if composePath := docker.FindComposeFile(dir); composePath != "" {
		*out = append(*out, project.Project{
			Name:        projectNameFromDir(dir),
			Path:        dir,
			ComposePath: composePath,
			Source:      project.SourceScan,
			Services:    []project.Service{},
		})
		// Un dossier projet n'est pas exploré plus loin
		return nil
	}

	if depth >= maxScanDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if depth == 0 {
			return fmt.Errorf("erreur lors de la lecture du répertoire: %w", err)
		}
		return nil // sous-dossier illisible → on ignore
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || skippedDirs[entry.Name()] {
			continue
		}
		scanDir(filepath.Join(dir, entry.Name()), depth+1, out)
	}
	return nil
}

// projectNameFromDir dérive le nom d'un projet depuis son dossier
func projectNameFromDir(dir string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Base(dir), "docker-"))
}

// DiscoverInDefaultPath découvre les projets sur disque :
// racines configurées ou racines par défaut + projets enregistrés via "add".
func DiscoverInDefaultPath() ([]project.Project, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = &config.Config{Projects: make(map[string]config.ProjectConfig)}
	}

	var projects []project.Project
	seen := make(map[string]bool)

	// 1. Auto-découverte depuis les racines (env > roots[] > root > défauts machine)
	for _, rootDir := range resolveRoots(cfg) {
		if info, err := os.Stat(rootDir); err != nil || !info.IsDir() {
			continue // racine absente sur cette machine → on ignore sans erreur
		}
		found, _ := NewDiscoverer(rootDir).Discover()
		for _, p := range found {
			if !seen[p.Name] {
				projects = append(projects, p)
				seen[p.Name] = true
			}
		}
	}

	// 2. Projets enregistrés explicitement via "docker-manager add".
	// Un projet explicite remplace l'entrée auto-découverte si même nom.
	for name, pcfg := range cfg.Projects {
		composePath := docker.FindComposeFile(pcfg.Path)
		if composePath == "" {
			composePath = filepath.Join(pcfg.Path, "docker-compose.yml")
		}
		if seen[name] {
			for i := range projects {
				if projects[i].Name == name {
					projects[i].Path = pcfg.Path
					projects[i].ComposePath = composePath
					projects[i].Source = project.SourceConfig
					break
				}
			}
			continue
		}
		projects = append(projects, project.Project{
			Name:        name,
			Path:        pcfg.Path,
			ComposePath: composePath,
			Source:      project.SourceConfig,
			Services:    []project.Service{},
		})
		seen[name] = true
	}

	return projects, nil
}

// DiscoverAll retourne l'inventaire complet de la machine : projets trouvés sur
// disque ET projets/containers connus de Docker (arrêtés inclus), fusionnés.
//
// C'est le point d'entrée portable : sans aucune configuration, l'outil retrouve
// où sont les projets grâce aux labels que docker compose pose sur les containers.
func DiscoverAll(mgr *docker.Manager) ([]project.Project, error) {
	projects, _ := DiscoverInDefaultPath()

	inventory, err := mgr.Inventory()
	if err != nil {
		// Docker injoignable : on retourne au moins ce qu'on a trouvé sur disque
		return projects, err
	}

	for _, inv := range inventory {
		if idx := matchProject(projects, inv); idx >= 0 {
			// Le projet est déjà connu sur disque : on lui greffe l'état Docker
			projects[idx].RunningCount = inv.RunningCount
			projects[idx].TotalCount = inv.TotalCount
			projects[idx].Containers = inv.Containers
			projects[idx].Images = inv.Images
			projects[idx].Standalone = inv.Standalone
			projects[idx].Running = inv.RunningCount > 0
			projects[idx].ServiceCount = inv.RunningCount
			// Le chemin enregistré peut venir d'une autre machine : si le fichier
			// compose n'existe pas ici mais que Docker en connaît un valide, il prime.
			if projects[idx].Path == "" || (!projects[idx].DockerComposeExists() && inv.DockerComposeExists()) {
				projects[idx].Path = inv.Path
				projects[idx].ComposePath = inv.ComposePath
			} else if projects[idx].ComposePath == "" {
				projects[idx].ComposePath = inv.ComposePath
			}
			continue
		}

		// Projet inconnu du disque : Docker sait où il est (ou c'est un standalone)
		inv.Orphan = !inv.DockerComposeExists()
		projects = append(projects, inv)
	}

	return projects, nil
}

// matchProject retrouve l'index d'un projet déjà listé correspondant à l'entrée
// d'inventaire : par chemin d'abord (le plus fiable), puis par nom.
func matchProject(projects []project.Project, inv project.Project) int {
	if inv.Path != "" {
		invPath := filepath.Clean(inv.Path)
		for i := range projects {
			if projects[i].Path != "" && filepath.Clean(projects[i].Path) == invPath {
				return i
			}
		}
	}
	for i := range projects {
		if projects[i].Name == inv.Name {
			return i
		}
	}
	// Le projet compose peut porter le nom du dossier avec son préfixe "docker-"
	normalized := strings.ToLower(strings.TrimPrefix(inv.Name, "docker-"))
	for i := range projects {
		if projects[i].Name == normalized {
			return i
		}
	}
	return -1
}

// EffectiveRoots retourne les racines réellement scannées sur cette machine
// (racines configurées et/ou racines par défaut, filtrées sur celles qui existent).
func EffectiveRoots() []string {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = &config.Config{Projects: make(map[string]config.ProjectConfig)}
	}
	var roots []string
	for _, r := range resolveRoots(cfg) {
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			roots = append(roots, r)
		}
	}
	return roots
}

// resolveRoots retourne la liste ordonnée des dossiers racines à scanner
func resolveRoots(cfg *config.Config) []string {
	if envRoot := os.Getenv("DOCKER_MANAGER_ROOT"); envRoot != "" {
		return filepath.SplitList(envRoot)
	}
	var configured []string
	if len(cfg.Roots) > 0 {
		configured = cfg.Roots
	} else if cfg.Root != "" {
		configured = []string{cfg.Root}
	}

	// Une config copiée depuis une autre machine peut ne désigner que des chemins
	// absents ici : dans ce cas on repasse sur les racines par défaut plutôt que
	// de ne rien trouver.
	for _, r := range configured {
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			return configured
		}
	}
	return append(configured, defaultRoots()...)
}

// defaultRoots retourne les emplacements habituels des projets Docker.
// Ils sont testés sur toutes les machines : ceux qui n'existent pas sont ignorés,
// ce qui permet de déplacer le binaire sans reconfigurer quoi que ce soit.
func defaultRoots() []string {
	home := os.Getenv("HOME")
	roots := []string{}
	if home != "" {
		roots = append(roots,
			filepath.Join(home, "docker"),
			filepath.Join(home, "dockers"),
			filepath.Join(home, "stacks"),
			filepath.Join(home, "containers"),
			filepath.Join(home, "compose"),
			home,
		)
	}
	roots = append(roots,
		"/opt/docker", "/opt/stacks", "/opt/containers",
		"/srv/docker", "/srv",
		"/volume1/docker", // NAS Synology
		"/home/pi/docker",
	)
	return roots
}

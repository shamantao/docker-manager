package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phil/docker-manager/pkg/config"
	"github.com/phil/docker-manager/pkg/project"
)

// Discoverer détecte automatiquement les projets Docker
type Discoverer struct {
	SearchPath string
}

// NewDiscoverer crée un nouveau découvreur
func NewDiscoverer(searchPath string) *Discoverer {
	return &Discoverer{
		SearchPath: searchPath,
	}
}

// Discover trouve tous les projets Docker dans le répertoire spécifié
func (d *Discoverer) Discover() ([]project.Project, error) {
	var projects []project.Project

	// Lister les dossiers docker-*
	entries, err := os.ReadDir(d.SearchPath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture du répertoire: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Chercher les dossiers docker-*
		if !strings.HasPrefix(entry.Name(), "docker-") {
			continue
		}

		projectPath := filepath.Join(d.SearchPath, entry.Name())
		composePath := filepath.Join(projectPath, "docker-compose.yml")

		// Vérifier que docker-compose.yml existe
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			continue
		}

		// Extraire le nom du projet (sans le préfixe "docker-")
		// Convertir en minuscules pour compatibilité docker-compose
		projectName := strings.ToLower(strings.TrimPrefix(entry.Name(), "docker-"))

		p := project.Project{
			Name:        projectName,
			Path:        projectPath,
			ComposePath: composePath,
			Services:    []project.Service{},
			Running:     false,
		}

		projects = append(projects, p)
	}

	return projects, nil
}

// DiscoverInDefaultPath découvre tous les projets : auto-scan des racines + projets explicites
func DiscoverInDefaultPath() ([]project.Project, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = &config.Config{Projects: make(map[string]config.ProjectConfig)}
	}

	var projects []project.Project
	seen := make(map[string]bool)

	// 1. Auto-découverte depuis les racines configurées (env > roots[] > root)
	for _, rootDir := range resolveRoots(cfg) {
		if _, err := os.Stat(rootDir); os.IsNotExist(err) {
			continue // dossier configuré mais absent → on ignore sans erreur
		}
		d := NewDiscoverer(rootDir)
		found, _ := d.Discover()
		for _, p := range found {
			if !seen[p.Name] {
				projects = append(projects, p)
				seen[p.Name] = true
			}
		}
	}

	// 2. Projets enregistrés explicitement via "docker-manager add"
	// Un projet explicite remplace l'entrée auto-découverte si même nom
	for name, pcfg := range cfg.Projects {
		composePath := filepath.Join(pcfg.Path, "docker-compose.yml")
		if seen[name] {
			for i := range projects {
				if projects[i].Name == name {
					projects[i].Path = pcfg.Path
					projects[i].ComposePath = composePath
					break
				}
			}
		} else {
			projects = append(projects, project.Project{
				Name:        name,
				Path:        pcfg.Path,
				ComposePath: composePath,
				Services:    []project.Service{},
			})
			seen[name] = true
		}
	}

	return projects, nil
}

// resolveRoots retourne la liste ordonnée des dossiers racines à scanner
func resolveRoots(cfg *config.Config) []string {
	if envRoot := os.Getenv("DOCKER_MANAGER_ROOT"); envRoot != "" {
		return []string{envRoot}
	}
	if len(cfg.Roots) > 0 {
		return cfg.Roots
	}
	if cfg.Root != "" {
		return []string{cfg.Root}
	}
	return nil
}

// defaultRootDir est conservé pour compatibilité éventuelle
func defaultRootDir() string {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, "docker")
}

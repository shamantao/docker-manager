package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// DiscoverInDefaultPath découvre les projets dans le chemin Docker par défaut
func DiscoverInDefaultPath() ([]project.Project, error) {
	dockerDir := filepath.Join(os.Getenv("HOME"), "devwww", "docker")
	if _, err := os.Stat(dockerDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("répertoire docker non trouvé: %s", dockerDir)
	}

	discoverer := NewDiscoverer(dockerDir)
	return discoverer.Discover()
}

package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// Sources possibles d'un projet
const (
	SourceConfig = "config" // enregistré explicitement via "add"
	SourceScan   = "scan"   // trouvé en scannant les dossiers racines
	SourceDocker = "docker" // reconstruit depuis les containers connus de Docker
)

// Service représente un service docker d'un projet
type Service struct {
	Name      string
	Status    string // running, exited, created
	Container string
	Ports     string
}

// Project représente un projet Docker complet
type Project struct {
	Name         string
	Path         string
	ComposePath  string
	Services     []Service
	Running      bool
	ServiceCount int
	Orphan       bool // true si le container tourne sans être dans la config

	// Inventaire Docker (rempli par la découverte via labels)
	Source       string   // config | scan | docker
	Standalone   bool     // container lancé via "docker run", hors docker-compose
	Containers   []string // noms des containers rattachés
	RunningCount int      // containers actuellement en cours
	TotalCount   int      // containers existants (running + arrêtés)
	Images       []string // images utilisées par les containers
}

// GetAbsolutePath retourne le chemin absolu du projet
func (p *Project) GetAbsolutePath() (string, error) {
	absPath, err := filepath.Abs(p.Path)
	if err != nil {
		return "", fmt.Errorf("erreur lors du calcul du chemin absolu: %w", err)
	}
	return absPath, nil
}

// Exists vérifie si le projet existe
func (p *Project) Exists() bool {
	if p.Path == "" {
		return false
	}
	info, err := os.Stat(p.Path)
	return err == nil && info.IsDir()
}

// DockerComposeExists vérifie si le fichier compose existe
func (p *Project) DockerComposeExists() bool {
	if p.ComposePath == "" {
		return false
	}
	info, err := os.Stat(p.ComposePath)
	return err == nil && !info.IsDir()
}

// Manageable indique si l'outil peut piloter ce projet (compose utilisable
// ou containers connus de Docker).
func (p *Project) Manageable() bool {
	return p.DockerComposeExists() || p.TotalCount > 0
}

// StoppedCount retourne le nombre de containers existants mais arrêtés
func (p *Project) StoppedCount() int {
	n := p.TotalCount - p.RunningCount
	if n < 0 {
		return 0
	}
	return n
}

// StatusString retourne un string formaté du statut.
// Trois états distincts : en cours, partiellement en cours, installé mais arrêté.
func (p *Project) StatusString() string {
	suffix := ""
	if p.Standalone {
		suffix = " ⬦" // container hors compose
	} else if p.Orphan {
		suffix = " 👻" // hors config, retrouvé via Docker
	}

	switch {
	case p.RunningCount > 0 && p.StoppedCount() > 0:
		return fmt.Sprintf("◐ Partiel (%d/%d)%s", p.RunningCount, p.TotalCount, suffix)
	case p.Running || p.RunningCount > 0:
		count := p.RunningCount
		if count == 0 {
			count = p.ServiceCount
		}
		return fmt.Sprintf("▶ Running (%d services)%s", count, suffix)
	case p.TotalCount > 0:
		return fmt.Sprintf("⏸ Installé, arrêté (%d)%s", p.TotalCount, suffix)
	default:
		return "⏹ Stopped" + suffix
	}
}

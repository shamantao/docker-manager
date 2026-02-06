package project

import (
	"fmt"
	"os"
	"path/filepath"
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
	info, err := os.Stat(p.Path)
	return err == nil && info.IsDir()
}

// DockerComposeExists vérifie si docker-compose.yml existe
func (p *Project) DockerComposeExists() bool {
	info, err := os.Stat(p.ComposePath)
	return err == nil && !info.IsDir()
}

// StatusString retourne un string formaté du statut
func (p *Project) StatusString() string {
	if p.Running {
		return fmt.Sprintf("▶ Running (%d services)", p.ServiceCount)
	}
	return "⏹ Stopped"
}

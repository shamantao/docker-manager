package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ServiceConfig contient la config d'un service
type ServiceConfig struct {
	Name        string `yaml:"name"`
	HealthCheck string `yaml:"health_check,omitempty"`
}

// ProjectConfig contient la config d'un projet
type ProjectConfig struct {
	Path     string           `yaml:"path"`
	Services []ServiceConfig  `yaml:"services,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
}

// Config contient la configuration globale
type Config struct {
	Projects map[string]ProjectConfig `yaml:"projects"`
}

// LoadConfig charge la configuration depuis le fichier YAML
func LoadConfig() (*Config, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".docker-manager", "projects.yml")

	// Si le fichier n'existe pas, retourner une config vide
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{
			Projects: make(map[string]ProjectConfig),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture du fichier config: %w", err)
	}

	cfg := &Config{
		Projects: make(map[string]ProjectConfig),
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du parsing du YAML: %w", err)
	}

	return cfg, nil
}

// SaveConfig sauvegarde la configuration dans le fichier YAML
func SaveConfig(cfg *Config) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".docker-manager")

	// Créer le répertoire s'il n'existe pas
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("erreur lors de la création du répertoire: %w", err)
	}

	configPath := filepath.Join(configDir, "projects.yml")

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation YAML: %w", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier: %w", err)
	}

	return nil
}

// GetProjectConfig retourne la configuration d'un projet spécifique
func (c *Config) GetProjectConfig(projectName string) ProjectConfig {
	if cfg, exists := c.Projects[projectName]; exists {
		return cfg
	}
	return ProjectConfig{}
}

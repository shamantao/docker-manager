package docker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/phil/docker-manager/pkg/project"
)

// Manager gère les opérations Docker
type Manager struct {
	WorkDir string
}

// NewManager crée un nouveau gestionnaire Docker
func NewManager(workDir string) *Manager {
	return &Manager{
		WorkDir: workDir,
	}
}

// StartProject démarre un projet avec build
func (m *Manager) StartProject(p *project.Project) error {
	fmt.Printf("🔨 Construction de l'image %s...\n", p.Name)
	cmd := exec.Command("docker-compose", "-f", "docker-compose.yml", "-p", p.Name, "build")
	cmd.Dir = p.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erreur lors de la construction: %w", err)
	}

	fmt.Printf("🚀 Démarrage du projet %s...\n", p.Name)
	cmd = exec.Command("docker-compose", "-f", "docker-compose.yml", "-p", p.Name, "up", "-d")
	cmd.Dir = p.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erreur lors du démarrage: %w", err)
	}

	fmt.Printf("✅ Projet %s démarré avec succès\n", p.Name)
	return nil
}

// StopProject arrête et supprime les containers
func (m *Manager) StopProject(p *project.Project) error {
	fmt.Printf("🛑 Arrêt du projet %s...\n", p.Name)
	cmd := exec.Command("docker-compose", "-f", "docker-compose.yml", "-p", p.Name, "down")
	cmd.Dir = p.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erreur lors de l'arrêt: %w", err)
	}

	fmt.Printf("✅ Projet %s arrêté et conteneurs supprimés\n", p.Name)
	return nil
}

// RestartService redémarre un service (rapide, sans rebuild)
func (m *Manager) RestartService(p *project.Project, serviceName string) error {
	fmt.Printf("🔄 Redémarrage du service %s du projet %s...\n", serviceName, p.Name)
	cmd := exec.Command("docker-compose", "-f", "docker-compose.yml", "-p", p.Name, "restart", serviceName)
	cmd.Dir = p.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erreur lors du redémarrage: %w", err)
	}

	fmt.Printf("✅ Service %s redémarré avec succès\n", serviceName)
	return nil
}

// GetStatus récupère le statut d'un projet
// Retourne: (running, containerCount, detailedError)
func (m *Manager) GetStatus(p *project.Project) (bool, int, error) {
	cmd := exec.Command("docker-compose", "-f", "docker-compose.yml", "-p", p.Name, "ps", "-q")
	cmd.Dir = p.Path

	output, err := cmd.Output()
	if err != nil {
		// Ne pas retourner d'erreur - juste indiquer "not ready"
		// Cela signifie que docker-compose.yml manque ou la config est cassée
		return false, 0, nil
	}

	containers := strings.Count(strings.TrimSpace(string(output)), "\n") + 1
	if strings.TrimSpace(string(output)) == "" {
		containers = 0
	}

	running := containers > 0 && strings.TrimSpace(string(output)) != ""

	return running, containers, nil
}

// GetStatusDetailed récupère le statut détaillé avec des informations d'erreur
func (m *Manager) GetStatusDetailed(p *project.Project) (bool, int, string) {
	cmd := exec.Command("docker-compose", "-f", "docker-compose.yml", "-p", p.Name, "ps", "-q")
	cmd.Dir = p.Path

	var stderr strings.Builder
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return false, 0, errMsg
	}

	containers := strings.Count(strings.TrimSpace(string(output)), "\n") + 1
	if strings.TrimSpace(string(output)) == "" {
		containers = 0
	}

	running := containers > 0 && strings.TrimSpace(string(output)) != ""

	statusMsg := "Arrêté"
	if running {
		statusMsg = fmt.Sprintf("En cours (%d containers)", containers)
	}

	return running, containers, statusMsg
}

// GetLogs récupère les logs d'un projet
func (m *Manager) GetLogs(p *project.Project, serviceName string, follow bool) error {
	args := []string{"-f", "docker-compose.yml", "-p", p.Name, "logs"}
	if follow {
		args = append(args, "-f")
	}
	if serviceName != "" {
		args = append(args, serviceName)
	}

	cmd := exec.Command("docker-compose", args...)
	cmd.Dir = p.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// GetServices retourne la liste des services d'un projet
func (m *Manager) GetServices(p *project.Project) ([]string, error) {
	cmd := exec.Command("docker-compose", "-f", "docker-compose.yml", "config", "--services")
	cmd.Dir = p.Path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des services: %w", err)
	}

	services := strings.Fields(strings.TrimSpace(string(output)))
	return services, nil
}

// GetServiceURLs retourne une map service -> urls locales exposees
func (m *Manager) GetServiceURLs(p *project.Project) (map[string][]string, error) {
	cmd := exec.Command(
		"docker",
		"ps",
		"--filter",
		fmt.Sprintf("label=com.docker.compose.project=%s", p.Name),
		"--format",
		"{{.Label \"com.docker.compose.service\"}}\t{{.Ports}}",
	)
	cmd.Dir = p.Path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la recuperation des ports: %w", err)
	}

	urlsByService := make(map[string][]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		serviceName := strings.TrimSpace(parts[0])
		ports := ""
		if len(parts) > 1 {
			ports = parts[1]
		}

		urls := portsToURLs(ports)
		if len(urls) == 0 {
			continue
		}

		urlsByService[serviceName] = appendUnique(urlsByService[serviceName], urls)
	}

	return urlsByService, nil
}

func portsToURLs(ports string) []string {
	ports = strings.TrimSpace(ports)
	if ports == "" {
		return nil
	}

	var urls []string
	seen := make(map[string]struct{})

	for _, segment := range strings.Split(ports, ",") {
		segment = strings.TrimSpace(segment)
		if !strings.Contains(segment, "->") {
			continue
		}

		parts := strings.SplitN(segment, "->", 2)
		if len(parts) < 2 {
			continue
		}

		hostPart := strings.TrimSpace(parts[0])
		port := extractHostPort(hostPart)
		if port == "" {
			continue
		}

		url := fmt.Sprintf("http://localhost:%s", port)
		if _, exists := seen[url]; exists {
			continue
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}

	return urls
}

func extractHostPort(hostPart string) string {
	hostPart = strings.TrimSpace(hostPart)
	if hostPart == "" {
		return ""
	}

	idx := strings.LastIndex(hostPart, ":")
	if idx == -1 || idx == len(hostPart)-1 {
		return ""
	}

	port := strings.TrimSpace(hostPart[idx+1:])
	return port
}

func appendUnique(existing []string, values []string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, value := range existing {
		seen[value] = struct{}{}
	}

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		existing = append(existing, value)
	}

	return existing
}

// EnsureDockerRunning vérifie que Docker est accessible
func EnsureDockerRunning() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker n'est pas accessible. Assurez-vous que Docker Desktop est lancé")
	}
	return nil
}

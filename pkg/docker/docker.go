package docker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// runCompose exécute une commande docker-compose dans le répertoire du projet.
// Si un fichier .env existe, il est sourcé via bash pour résoudre l'interpolation de variables.
// Stdout et stderr sont capturés pour pouvoir afficher les erreurs dans le TUI.
func (m *Manager) runCompose(p *project.Project, args ...string) (string, error) {
	envFile := filepath.Join(p.Path, ".env")

	var cmd *exec.Cmd
	composeArgs := append([]string{"-f", "docker-compose.yml", "-p", p.Name}, args...)

	if _, err := os.Stat(envFile); err == nil {
		// .env existe → passer par bash pour résoudre l'interpolation
		// --env-file /dev/null empêche docker-compose de relire .env avec son
		// propre parser (qui ne supporte pas l'interpolation bash)
		composeArgs = append([]string{"--env-file", "/dev/null"}, composeArgs...)
		shellCmd := "set -a && source .env && set +a && docker-compose " + shelljoin(composeArgs)
		cmd = exec.Command("bash", "-c", shellCmd)
	} else {
		cmd = exec.Command("docker-compose", composeArgs...)
	}
	cmd.Dir = p.Path

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errDetail := strings.TrimSpace(stderr.String())
		if errDetail == "" {
			errDetail = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s", errDetail)
	}
	return stdout.String(), nil
}

// shelljoin concatène des arguments en les échappant pour le shell
func shelljoin(args []string) string {
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = "'" + strings.ReplaceAll(a, "'", "'\"'\"'") + "'"
	}
	return strings.Join(escaped, " ")
}

// runComposeStream lance docker-compose et envoie les lignes de sortie dans un channel.
// Le channel est fermé quand la commande se termine.
// Retourne une erreur si la commande échoue.
func (m *Manager) runComposeStream(p *project.Project, output chan<- string, args ...string) error {
	envFile := filepath.Join(p.Path, ".env")

	var cmd *exec.Cmd
	composeArgs := append([]string{"-f", "docker-compose.yml", "-p", p.Name}, args...)

	if _, err := os.Stat(envFile); err == nil {
		composeArgs = append([]string{"--env-file", "/dev/null"}, composeArgs...)
		shellCmd := "set -a && source .env && set +a && docker-compose " + shelljoin(composeArgs)
		cmd = exec.Command("bash", "-c", shellCmd)
	} else {
		cmd = exec.Command("docker-compose", composeArgs...)
	}
	cmd.Dir = p.Path

	// Combiner stdout+stderr dans un seul pipe
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		return err
	}

	// Lire les lignes en streaming
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			output <- scanner.Text()
		}
	}()

	err := cmd.Wait()
	pw.Close()
	return err
}

// StartProjectStream démarre un projet avec build, en streamant la sortie
func (m *Manager) StartProjectStream(p *project.Project, output chan<- string) error {
	output <- "🔨 Construction de l'image..."
	if err := m.runComposeStream(p, output, "build"); err != nil {
		return fmt.Errorf("erreur lors de la construction: %w", err)
	}

	output <- "🚀 Démarrage des containers..."
	if err := m.runComposeStream(p, output, "up", "-d"); err != nil {
		return fmt.Errorf("erreur lors du démarrage: %w", err)
	}

	return nil
}

// StopProjectStream arrête un projet en streamant la sortie
func (m *Manager) StopProjectStream(p *project.Project, output chan<- string) error {
	output <- "🛑 Arrêt des containers..."
	if err := m.runComposeStream(p, output, "down"); err != nil {
		return fmt.Errorf("erreur lors de l'arrêt: %w", err)
	}
	return nil
}

// RestartServiceStream redémarre un service en streamant la sortie
func (m *Manager) RestartServiceStream(p *project.Project, output chan<- string, serviceName string) error {
	output <- "🔄 Redémarrage..."
	args := []string{"restart"}
	if serviceName != "" {
		args = append(args, serviceName)
	}
	if err := m.runComposeStream(p, output, args...); err != nil {
		return fmt.Errorf("erreur lors du redémarrage: %w", err)
	}
	return nil
}

// StopOrphanProjectStream arrête un container orphelin en streamant la sortie
func (m *Manager) StopOrphanProjectStream(p *project.Project, output chan<- string) error {
	output <- "🛑 Arrêt du container orphelin..."
	cmd := exec.Command("docker", "ps", "-q", "--filter", fmt.Sprintf("label=com.docker.compose.project=%s", p.Name))
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		ids := strings.Fields(strings.TrimSpace(string(out)))
		cmd = exec.Command("docker", append([]string{"stop"}, ids...)...)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("erreur arrêt: %s", strings.TrimSpace(stderr.String()))
		}
		cmd = exec.Command("docker", append([]string{"rm"}, ids...)...)
		cmd.Run()
		return nil
	}
	cmd = exec.Command("docker", "stop", p.Name)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erreur arrêt: %s", strings.TrimSpace(stderr.String()))
	}
	cmd = exec.Command("docker", "rm", p.Name)
	cmd.Run()
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

// DiscoverOrphanProjects liste les containers Docker en cours d'exécution
// qui ne font partie d'aucun projet connu. Retourne des Project marqués Orphan.
func (m *Manager) DiscoverOrphanProjects(knownNames map[string]bool) ([]project.Project, error) {
	// Lister tous les containers en cours avec leur projet compose
	cmd := exec.Command("docker", "ps", "--format", "{{.Label \"com.docker.compose.project\"}}\t{{.Names}}\t{{.ID}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Grouper par projet compose
	type containerInfo struct {
		names []string
		count int
	}
	composeProjects := make(map[string]*containerInfo)

	// Containers standalone (sans projet compose)
	standaloneContainers := make(map[string]*containerInfo)

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		composeName := strings.TrimSpace(parts[0])
		containerName := ""
		if len(parts) > 1 {
			containerName = strings.TrimSpace(parts[1])
		}

		if composeName == "" {
			// Container standalone (lancé via docker run / Docker Desktop)
			if containerName != "" {
				if _, exists := standaloneContainers[containerName]; !exists {
					standaloneContainers[containerName] = &containerInfo{}
				}
				standaloneContainers[containerName].count++
				standaloneContainers[containerName].names = append(standaloneContainers[containerName].names, containerName)
			}
		} else {
			if _, exists := composeProjects[composeName]; !exists {
				composeProjects[composeName] = &containerInfo{}
			}
			composeProjects[composeName].count++
			if containerName != "" {
				composeProjects[composeName].names = append(composeProjects[composeName].names, containerName)
			}
		}
	}

	var orphans []project.Project

	// Projets compose non connus
	for name, info := range composeProjects {
		if knownNames[name] {
			continue
		}
		orphans = append(orphans, project.Project{
			Name:         name,
			Running:      true,
			ServiceCount: info.count,
			Orphan:       true,
		})
	}

	// Containers standalone
	for name, info := range standaloneContainers {
		if knownNames[name] {
			continue
		}
		orphans = append(orphans, project.Project{
			Name:         name,
			Running:      true,
			ServiceCount: info.count,
			Orphan:       true,
		})
	}

	return orphans, nil
}

// StopOrphanProject arrête un container/projet orphelin (sans docker-compose.yml)
func (m *Manager) StopOrphanProject(p *project.Project) error {
	// Essayer d'abord comme projet compose
	cmd := exec.Command("docker", "ps", "-q", "--filter", fmt.Sprintf("label=com.docker.compose.project=%s", p.Name))
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		// C'est un projet compose — arrêter via docker compose
		ids := strings.Fields(strings.TrimSpace(string(output)))
		cmd = exec.Command("docker", append([]string{"stop"}, ids...)...)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("erreur arrêt: %s", strings.TrimSpace(stderr.String()))
		}
		// Supprimer les containers
		cmd = exec.Command("docker", append([]string{"rm"}, ids...)...)
		cmd.Run() // best effort
		return nil
	}

	// Sinon container standalone — arrêter par nom
	cmd = exec.Command("docker", "stop", p.Name)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erreur arrêt: %s", strings.TrimSpace(stderr.String()))
	}
	cmd = exec.Command("docker", "rm", p.Name)
	cmd.Run() // best effort
	return nil
}

// EnsureDockerRunning vérifie que Docker est accessible
func EnsureDockerRunning() error {
	installed, _ := CheckDockerInstallation()
	if !installed {
		return fmt.Errorf("❌ Docker n'est pas installé.\n📖 Visitez: https://www.docker.com/products/docker-desktop")
	}

	running, _ := CheckDockerDaemonStatus()
	if !running {
		return fmt.Errorf("⏹️  Docker daemon est arrêté.\nUsez: docker-manager daemon start")
	}
	return nil
}

// CheckDockerInstallation vérifie si Docker est installé
func CheckDockerInstallation() (bool, error) {
	cmd := exec.Command("docker", "--version")
	err := cmd.Run()
	return err == nil, nil
}

// CheckDockerDaemonStatus vérifie si le daemon Docker est actif
func CheckDockerDaemonStatus() (bool, error) {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	return err == nil, nil
}

// StartDockerDaemon démarre Docker
func StartDockerDaemon() error {
	switch runtime.GOOS {
	case "darwin":
		// macOS: ouvrir Docker.app
		cmd := exec.Command("open", "-a", "Docker")
		return cmd.Run()
	case "linux":
		// Linux: systemctl start docker
		cmd := exec.Command("sudo", "systemctl", "start", "docker")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	case "windows":
		// Windows: PowerShell
		cmd := exec.Command("powershell", "-Command", "Start-Process Docker")
		return cmd.Run()
	default:
		return fmt.Errorf("système d'exploitation non supporté")
	}
}

// StopDockerDaemon arrête Docker
func StopDockerDaemon() error {
	switch runtime.GOOS {
	case "darwin":
		// macOS: quit application Docker Desktop (syntaxe osascript correcte)
		cmd := exec.Command("osascript", "-e", "quit application \"Docker Desktop\"")
		err := cmd.Run()
		if err != nil {
			// Fallback: utiliser killall si osascript échoue
			killCmd := exec.Command("killall", "Docker")
			return killCmd.Run()
		}
		return nil
	case "linux":
		// Linux: systemctl stop docker
		cmd := exec.Command("sudo", "systemctl", "stop", "docker")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	case "windows":
		// Windows: PowerShell
		cmd := exec.Command("powershell", "-Command", "Stop-Process -Name Docker.exe")
		return cmd.Run()
	default:
		return fmt.Errorf("système d'exploitation non supporté")
	}
}

// GetDockerInstallURL retourne l'URL d'installation de Docker selon l'OS
func GetDockerInstallURL() string {
	return "https://www.docker.com/products/docker-desktop"
}

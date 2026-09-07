package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/phil/docker-manager/pkg/project"
)

// Labels posés par docker compose sur chaque container qu'il crée.
// Ce sont eux qui permettent de retrouver l'emplacement des projets
// sur n'importe quelle machine, sans configuration préalable.
const (
	labelProject    = "com.docker.compose.project"
	labelWorkingDir = "com.docker.compose.project.working_dir"
	labelConfigFile = "com.docker.compose.project.config_files"
)

// containerRow est une ligne brute de "docker ps -a"
type containerRow struct {
	Project    string
	WorkingDir string
	ConfigFile string
	Name       string
	State      string
	Image      string
}

// listContainers retourne tous les containers de la machine, arrêtés inclus.
func listContainers() ([]containerRow, error) {
	format := fmt.Sprintf(
		"{{.Label \"%s\"}}\t{{.Label \"%s\"}}\t{{.Label \"%s\"}}\t{{.Names}}\t{{.State}}\t{{.Image}}",
		labelProject, labelWorkingDir, labelConfigFile,
	)
	cmd := exec.Command("docker", "ps", "-a", "--format", format)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("impossible de lister les containers: %w", err)
	}

	return parseContainerRows(string(out)), nil
}

// parseContainerRows convertit la sortie tabulée de "docker ps -a" en lignes typées
func parseContainerRows(out string) []containerRow {
	var rows []containerRow
	// Attention : ne trimmer que les sauts de ligne. Un TrimSpace global
	// supprimerait les tabulations de tête de la première ligne, càd les labels
	// vides d'un container hors compose, et décalerait toutes ses colonnes.
	for _, line := range strings.Split(strings.Trim(out, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		line = strings.TrimRight(line, "\r")
		f := strings.Split(line, "\t")
		for len(f) < 6 {
			f = append(f, "")
		}
		rows = append(rows, containerRow{
			Project:    strings.TrimSpace(f[0]),
			WorkingDir: strings.TrimSpace(f[1]),
			ConfigFile: strings.TrimSpace(f[2]),
			Name:       strings.TrimSpace(f[3]),
			State:      strings.TrimSpace(f[4]),
			Image:      strings.TrimSpace(f[5]),
		})
	}
	return rows
}

// resolveComposePath déduit le chemin du fichier compose depuis les labels.
// config_files peut contenir plusieurs chemins séparés par des virgules,
// absolus (compose v2) ou relatifs au working_dir (versions plus anciennes).
func resolveComposePath(workingDir, configFiles string) string {
	if configFiles != "" {
		first := strings.TrimSpace(strings.Split(configFiles, ",")[0])
		if first != "" {
			if !filepath.IsAbs(first) && workingDir != "" {
				first = filepath.Join(workingDir, first)
			}
			if info, err := os.Stat(first); err == nil && !info.IsDir() {
				return first
			}
		}
	}
	if workingDir != "" {
		return FindComposeFile(workingDir)
	}
	return ""
}

// Inventory reconstruit la liste des projets à partir de ce que Docker connaît :
// projets compose (via leurs labels, y compris leur emplacement sur disque) et
// containers standalone lancés en "docker run". Les containers arrêtés sont
// inclus — c'est ce qui permet de voir ce qui est installé mais non lancé.
//
// Aucune configuration n'est nécessaire : l'inventaire est valable sur
// n'importe quelle machine où l'outil est déposé.
func (m *Manager) Inventory() ([]project.Project, error) {
	rows, err := listContainers()
	if err != nil {
		return nil, err
	}
	return buildInventory(rows), nil
}

// buildInventory regroupe les containers par projet compose (ou par container
// pour les standalone) et calcule leur état.
func buildInventory(rows []containerRow) []project.Project {
	byName := make(map[string]*project.Project)
	order := []string{}

	get := func(name string) *project.Project {
		if p, ok := byName[name]; ok {
			return p
		}
		p := &project.Project{Name: name, Source: project.SourceDocker}
		byName[name] = p
		order = append(order, name)
		return p
	}

	for _, r := range rows {
		var p *project.Project

		if r.Project != "" {
			p = get(r.Project)
			if p.Path == "" && r.WorkingDir != "" {
				p.Path = r.WorkingDir
			}
			if p.ComposePath == "" {
				p.ComposePath = resolveComposePath(r.WorkingDir, r.ConfigFile)
			}
		} else {
			if r.Name == "" {
				continue
			}
			p = get(r.Name)
			p.Standalone = true
		}

		p.TotalCount++
		if r.State == "running" {
			p.RunningCount++
		}
		if r.Name != "" {
			p.Containers = append(p.Containers, r.Name)
		}
		if r.Image != "" {
			p.Images = appendUnique(p.Images, []string{r.Image})
		}
	}

	sort.Strings(order)
	projects := make([]project.Project, 0, len(order))
	for _, name := range order {
		p := byName[name]
		p.Running = p.RunningCount > 0
		p.ServiceCount = p.RunningCount
		projects = append(projects, *p)
	}
	return projects
}

// containerIDs retourne les IDs des containers d'un projet (arrêtés inclus).
func containerIDs(p *project.Project) []string {
	var args []string
	if p.Standalone {
		args = []string{"ps", "-a", "-q", "--filter", "name=^/" + p.Name + "$"}
	} else {
		args = []string{"ps", "-a", "-q", "--filter", fmt.Sprintf("label=%s=%s", labelProject, p.Name)}
	}
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return nil
	}
	return strings.Fields(strings.TrimSpace(string(out)))
}

// dockerActionStream applique une action docker (start/stop/restart) aux
// containers d'un projet sans fichier compose, en streamant la sortie.
// Contrairement à "compose down", les containers ne sont jamais supprimés :
// un container arrêté reste visible et relançable.
func (m *Manager) dockerActionStream(p *project.Project, output chan<- string, action string) error {
	ids := containerIDs(p)
	if len(ids) == 0 {
		return fmt.Errorf("aucun container trouvé pour '%s'", p.Name)
	}

	output <- fmt.Sprintf("🐳 docker %s (%d container(s))...", action, len(ids))

	var stderr strings.Builder
	cmd := exec.Command("docker", append([]string{action}, ids...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			output <- line
		}
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("erreur docker %s: %s", action, detail)
	}
	return nil
}

// StartContainersStream démarre les containers existants d'un projet non-compose
func (m *Manager) StartContainersStream(p *project.Project, output chan<- string) error {
	return m.dockerActionStream(p, output, "start")
}

// StopContainersStream arrête les containers d'un projet non-compose (sans les supprimer)
func (m *Manager) StopContainersStream(p *project.Project, output chan<- string) error {
	return m.dockerActionStream(p, output, "stop")
}

// RestartContainersStream redémarre les containers d'un projet non-compose
func (m *Manager) RestartContainersStream(p *project.Project, output chan<- string) error {
	return m.dockerActionStream(p, output, "restart")
}

// UpdateProjectStream met à jour les images d'un projet puis recrée les containers.
//
// Périmètre volontairement restreint : on ne fait que rafraîchir des images
// DÉJÀ référencées par le projet (docker compose pull / docker pull sur l'image
// du container existant). Aucune image nouvelle ou inconnue n'est téléchargée.
func (m *Manager) UpdateProjectStream(p *project.Project, output chan<- string) error {
	if p.DockerComposeExists() {
		output <- "⬇️  Mise à jour des images du projet..."
		if err := m.runComposeStream(p, output, "pull"); err != nil {
			return fmt.Errorf("erreur lors du pull: %w", err)
		}
		output <- "🚀 Recréation des containers..."
		if err := m.runComposeStream(p, output, "up", "-d"); err != nil {
			return fmt.Errorf("erreur lors de la recréation: %w", err)
		}
		return nil
	}

	// Pas de fichier compose : on rafraîchit l'image du/des containers existants.
	if len(p.Images) == 0 {
		return fmt.Errorf("aucune image connue pour '%s'", p.Name)
	}
	for _, img := range p.Images {
		output <- "⬇️  docker pull " + img
		var stderr strings.Builder
		cmd := exec.Command("docker", "pull", img)
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) != "" {
				output <- line
			}
		}
		if err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = err.Error()
			}
			return fmt.Errorf("erreur pull %s: %s", img, detail)
		}
	}
	output <- "ℹ️  Image à jour. Sans fichier compose, recréez le container pour l'appliquer."
	return nil
}

// GetContainerLogs affiche les logs des containers d'un projet sans fichier compose.
func (m *Manager) GetContainerLogs(p *project.Project, follow bool) error {
	ids := containerIDs(p)
	if len(ids) == 0 {
		return fmt.Errorf("aucun container trouvé pour '%s'", p.Name)
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if len(ids) > 1 && !follow {
		// docker logs ne prend qu'un container à la fois : on les enchaîne
		for _, id := range ids {
			cmd := exec.Command("docker", append(append([]string{}, args...), id)...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		return nil
	}

	cmd := exec.Command("docker", append(args, ids[0])...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

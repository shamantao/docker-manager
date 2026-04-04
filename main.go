package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/phil/docker-manager/pkg/config"
	"github.com/phil/docker-manager/pkg/discovery"
	"github.com/phil/docker-manager/pkg/docker"
	"github.com/phil/docker-manager/pkg/project"
	"github.com/phil/docker-manager/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// Version est la version unique de docker-manager
const Version = "1.3.0"

var logger = log.New(os.Stderr)

func main() {
	// Initialiser le fichier de config par défaut si nécessaire
	if err := config.EnsureDefaultConfig(); err != nil {
		logger.Warn("Impossible de créer le fichier de config par défaut", "error", err)
	}

	if len(os.Args) < 2 {
		printHelp()
		handleStatus()
		return
	}

	command := os.Args[1]

	switch command {
	case "start":
		if len(os.Args) < 3 {
			fmt.Println("usage: docker-manager start <project>")
			os.Exit(1)
		}
		if err := handleStart(os.Args[2]); err != nil {
			logger.Fatal(err)
		}

	case "stop":
		if len(os.Args) < 3 {
			fmt.Println("usage: docker-manager stop <project>")
			os.Exit(1)
		}
		if err := handleStop(os.Args[2]); err != nil {
			logger.Fatal(err)
		}

	case "restart":
		if len(os.Args) < 3 {
			fmt.Println("usage: docker-manager restart <project> [service]")
			os.Exit(1)
		}
		service := ""
		if len(os.Args) > 3 {
			service = os.Args[3]
		}
		if err := handleRestart(os.Args[2], service); err != nil {
			logger.Fatal(err)
		}

	case "status":
		// docker-manager status [project]
		if len(os.Args) > 2 {
			// Status détaillé d'un projet
			if err := handleStatusProject(os.Args[2]); err != nil {
				logger.Fatal(err)
			}
		} else {
			// Status global
			if err := handleStatus(); err != nil {
				logger.Fatal(err)
			}
		}

	case "logs":
		fs := flag.NewFlagSet("logs", flag.ExitOnError)
		follow := fs.Bool("f", false, "Suit les logs en temps réel")
		fs.Parse(os.Args[2:])

		args := fs.Args()
		if len(args) < 1 {
			fmt.Println("usage: docker-manager logs <project> [service] [-f]")
			os.Exit(1)
		}

		service := ""
		if len(args) > 1 {
			service = args[1]
		}

		if err := handleLogs(args[0], service, *follow); err != nil {
			logger.Fatal(err)
		}

	case "dashboard":
		if err := handleDashboard(); err != nil {
			logger.Fatal(err)
		}

	case "daemon":
		if len(os.Args) < 3 {
			fmt.Println("usage: docker-manager daemon <start|stop|status>")
			os.Exit(1)
		}
		if err := handleDaemon(os.Args[2]); err != nil {
			logger.Fatal(err)
		}

	case "add":
		if len(os.Args) < 3 {
			fmt.Println("usage: docker-manager add <path>")
			os.Exit(1)
		}
		if err := handleAdd(os.Args[2]); err != nil {
			logger.Fatal(err)
		}

	case "remove":
		if len(os.Args) < 3 {
			fmt.Println("usage: docker-manager remove <project>")
			os.Exit(1)
		}
		if err := handleRemove(os.Args[2]); err != nil {
			logger.Fatal(err)
		}

	case "list":
		if err := handleList(); err != nil {
			logger.Fatal(err)
		}

	case "--version", "-v":
		fmt.Printf("Docker Manager v%s\n", Version)

	case "--help", "-h", "help":
		printHelp()

	default:
		fmt.Printf("Commande inconnue: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf("Docker Manager v%s\n", Version)
	fmt.Print(`
Usage:
  docker-manager <command> [options]

Commands:
  add <path>               Enregistre un projet Docker (chemin vers le dossier)
  remove <project>         Retire un projet de la config
  list                     Affiche les projets enregistrés
  start <project>          Démarre un projet (build + container)
  stop <project>           Arrête et supprime les containers
  restart <project>        Redémarre un projet (sans rebuild)
  status [project]         Affiche le statut (global ou d'un projet)
  logs <project> [service] Affiche les logs
                           Options: -f (follow en temps réel)
  daemon <start|stop|status> Gère le daemon Docker
  dashboard                Lance le dashboard interactif

Exemples:
  docker-manager add ~/kDrive/docker/docker-pbwww
  docker-manager add /chemin/vers/mon-projet
  docker-manager remove pbwww
  docker-manager list
  docker-manager start pbwww
  docker-manager stop pbwww
  docker-manager restart pbwww nginx
  docker-manager status                    # Tous les projets
  docker-manager status pbwww              # Détail d'un projet
  docker-manager logs pbwww -f
  docker-manager daemon status             # Check Docker daemon
  docker-manager daemon start              # Démarrer Docker daemon
  docker-manager daemon stop               # Arrêter Docker daemon
  docker-manager dashboard

Options:
  -h, --help              Affiche cette aide
  -v, --version           Affiche la version
`)
}

func handleStart(projectName string) error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	var targetProject *project.Project
	for i := range projects {
		if projects[i].Name == projectName {
			targetProject = &projects[i]
			break
		}
	}

	if targetProject == nil {
		return fmt.Errorf("projet '%s' non trouvé", projectName)
	}

	mgr := docker.NewManager(targetProject.Path)
	output := make(chan string, 64)
	go func() {
		for line := range output {
			fmt.Println(line)
		}
	}()
	return mgr.StartProjectStream(targetProject, output)
}

func handleStop(projectName string) error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	var targetProject *project.Project
	for i := range projects {
		if projects[i].Name == projectName {
			targetProject = &projects[i]
			break
		}
	}

	if targetProject == nil {
		return fmt.Errorf("projet '%s' non trouvé", projectName)
	}

	mgr := docker.NewManager(targetProject.Path)
	output := make(chan string, 64)
	go func() {
		for line := range output {
			fmt.Println(line)
		}
	}()
	return mgr.StopProjectStream(targetProject, output)
}

func handleRestart(projectName string, serviceName string) error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	var targetProject *project.Project
	for i := range projects {
		if projects[i].Name == projectName {
			targetProject = &projects[i]
			break
		}
	}

	if targetProject == nil {
		return fmt.Errorf("projet '%s' non trouvé", projectName)
	}

	mgr := docker.NewManager(targetProject.Path)
	output := make(chan string, 64)
	go func() {
		for line := range output {
			fmt.Println(line)
		}
	}()
	return mgr.RestartServiceStream(targetProject, output, serviceName)
}

func handleStatus() error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	fmt.Println("\n📊 Statut des projets Docker")
	fmt.Println("─────────────────────────────────────────")

	mgr := docker.NewManager("")

	knownNames := make(map[string]bool)
	for _, p := range projects {
		running, count, _ := mgr.GetStatus(&p)
		knownNames[p.Name] = true

		if running {
			fmt.Printf("  %-20s ▶  Running (%d services)\n", p.Name, count)
		} else {
			fmt.Printf("  %-20s ⏹  Stopped\n", p.Name)
		}
	}

	// Containers orphelins
	orphans, _ := mgr.DiscoverOrphanProjects(knownNames)
	if len(orphans) > 0 {
		fmt.Println("  ── containers hors config ──")
		for _, o := range orphans {
			fmt.Printf("  %-20s ▶  Running (%d services) 👻\n", o.Name, o.ServiceCount)
		}
	}

	fmt.Println("─────────────────────────────────────────")
	fmt.Println()
	return nil
}

func handleStatusProject(projectName string) error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	var targetProject *project.Project
	for i := range projects {
		if projects[i].Name == projectName {
			targetProject = &projects[i]
			break
		}
	}

	if targetProject == nil {
		return fmt.Errorf("projet '%s' non trouvé", projectName)
	}

	mgr := docker.NewManager(targetProject.Path)

	fmt.Println()
	fmt.Printf("📊 Status détaillé : %s\n", targetProject.Name)
	fmt.Println("─────────────────────────────────────────")

	// Utiliser GetStatusDetailed pour avoir plus d'infos
	running, _, statusMsg := mgr.GetStatusDetailed(targetProject)

	if running {
		fmt.Printf("  Status   : ▶ %s\n", statusMsg)
	} else {
		fmt.Printf("  Status   : ⏹ %s\n", statusMsg)
	}

	// Essayer de récupérer les services
	services, err := mgr.GetServices(targetProject)
	if err == nil && len(services) > 0 {
		fmt.Printf("  Services : %v\n", services)
	}

	urlsByService, err := mgr.GetServiceURLs(targetProject)
	if err == nil && len(urlsByService) > 0 {
		fmt.Println("  URLs     :")
		if len(services) > 0 {
			for _, service := range services {
				urls, ok := urlsByService[service]
				if !ok {
					continue
				}
				for _, url := range urls {
					fmt.Printf("    - %s => %s\n", service, url)
				}
			}
		} else {
			for service, urls := range urlsByService {
				for _, url := range urls {
					fmt.Printf("    - %s => %s\n", service, url)
				}
			}
		}
	}

	// Afficher le chemin du projet
	fmt.Printf("  Path     : %s\n", targetProject.Path)
	fmt.Printf("  Compose  : %s\n", targetProject.ComposePath)

	// Vérifier que les fichiers existent
	if _, err := os.Stat(targetProject.ComposePath); os.IsNotExist(err) {
		fmt.Printf("  ⚠️  docker-compose.yml manquant!\n")
	}

	fmt.Println("─────────────────────────────────────────")
	fmt.Println()
	return nil
}

func handleLogs(projectName string, serviceName string, follow bool) error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	var targetProject *project.Project
	for i := range projects {
		if projects[i].Name == projectName {
			targetProject = &projects[i]
			break
		}
	}

	if targetProject == nil {
		return fmt.Errorf("projet '%s' non trouvé", projectName)
	}

	mgr := docker.NewManager(targetProject.Path)
	return mgr.GetLogs(targetProject, serviceName, follow)
}

func handleDashboard() error {
	if err := docker.EnsureDockerRunning(); err != nil {
		return err
	}

	projects, err := discovery.DiscoverInDefaultPath()
	if err != nil {
		return err
	}

	mgr := docker.NewManager("")

	// Charger les statuts
	knownNames := make(map[string]bool)
	for i := range projects {
		running, count, _ := mgr.GetStatus(&projects[i])
		projects[i].Running = running
		projects[i].ServiceCount = count
		knownNames[projects[i].Name] = true
	}

	// Ajouter les containers orphelins (non gérés par la config)
	orphans, _ := mgr.DiscoverOrphanProjects(knownNames)
	projects = append(projects, orphans...)

	model := tui.NewModel(projects, mgr)
	prog := tea.NewProgram(model)

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("erreur du dashboard: %w", err)
	}

	return nil
}

func handleDaemon(action string) error {
	installed, _ := docker.CheckDockerInstallation()
	if !installed {
		fmt.Println("❌ Docker n'est pas installé")
		fmt.Printf("📖 Téléchargez Docker: %s\n", docker.GetDockerInstallURL())
		return nil
	}

	running, _ := docker.CheckDockerDaemonStatus()

	switch action {
	case "status":
		if running {
			fmt.Println("✅ Docker daemon est actif")
		} else {
			fmt.Println("⏹️  Docker daemon est arrêté")
		}
	case "start":
		if running {
			fmt.Println("ℹ️  Docker daemon est déjà en cours d'exécution")
			return nil
		}
		fmt.Println("🚀 Démarrage de Docker daemon...")
		if err := docker.StartDockerDaemon(); err != nil {
			return fmt.Errorf("erreur au démarrage du daemon: %w", err)
		}
		fmt.Println("✅ Docker daemon a été démarré")
	case "stop":
		if !running {
			fmt.Println("ℹ️  Docker daemon est déjà arrêté")
			return nil
		}
		fmt.Println("🛑 Arrêt de Docker daemon...")
		if err := docker.StopDockerDaemon(); err != nil {
			return fmt.Errorf("erreur à l'arrêt du daemon: %w", err)
		}
		fmt.Println("✅ Docker daemon a été arrêté")
	default:
		fmt.Printf("Action inconnue: %s\n", action)
		fmt.Println("Utilisez: start, stop, ou status")
		return nil
	}
	return nil
}

func handleAdd(dirPath string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("chemin invalide: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("dossier introuvable: %s", absPath)
	}

	composePath := filepath.Join(absPath, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml non trouvé dans %s", absPath)
	}

	// Dérive le nom depuis le dossier (retire le préfixe "docker-" si présent)
	name := strings.ToLower(strings.TrimPrefix(filepath.Base(absPath), "docker-"))

	if err := config.AddProject(name, absPath); err != nil {
		return err
	}

	fmt.Printf("✅ Projet '%s' ajouté → %s\n", name, absPath)

	// Ouvrir le dashboard si Docker est disponible
	if err := docker.EnsureDockerRunning(); err != nil {
		fmt.Println("ℹ️  Docker daemon inactif — démarrez-le puis : docker-manager dashboard")
		return nil
	}
	fmt.Println("📂 Ouverture du dashboard...")
	return handleDashboard()
}

func handleRemove(name string) error {
	if err := config.RemoveProject(name); err != nil {
		return err
	}
	fmt.Printf("✅ Projet '%s' retiré de la config\n", name)
	return nil
}

func handleList() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	fmt.Println("\n📋 Projets enregistrés dans ~/.docker-manager/projects.yml")
	fmt.Println("─────────────────────────────────────────")

	if cfg.Root != "" {
		fmt.Printf("  📂 auto-discover root : %s\n", cfg.Root)
	}
	for _, r := range cfg.Roots {
		fmt.Printf("  📂 auto-discover root : %s\n", r)
	}

	if len(cfg.Projects) == 0 {
		fmt.Println("  (aucun projet enregistré)")
		fmt.Println("\n  Ajoutez un projet : docker-manager add /chemin/vers/projet")
	} else {
		for name, p := range cfg.Projects {
			warn := ""
			if _, err := os.Stat(filepath.Join(p.Path, "docker-compose.yml")); os.IsNotExist(err) {
				warn = " ⚠️  (docker-compose.yml introuvable)"
			}
			fmt.Printf("  ▶ %-20s → %s%s\n", name, p.Path, warn)
		}
	}

	fmt.Println("─────────────────────────────────────────")
	fmt.Println()
	return nil
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/phil/docker-manager/pkg/config"
	"github.com/phil/docker-manager/pkg/discovery"
	"github.com/phil/docker-manager/pkg/docker"
	"github.com/phil/docker-manager/pkg/project"
	"github.com/phil/docker-manager/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

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

	case "--version", "-v":
		fmt.Println("Docker Manager v1.0.0")

	case "--help", "-h", "help":
		printHelp()

	default:
		fmt.Printf("Commande inconnue: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Docker Manager v1.0.0

Usage:
  docker-manager <command> [options]

Commands:
  start <project>          Démarre un projet (build + container)
  stop <project>           Arrête et supprime les containers
  restart <project>        Redémarre un projet (sans rebuild)
  status [project]         Affiche le statut (global ou d'un projet)
  logs <project> [service] Affiche les logs
                           Options: -f (follow en temps réel)
  daemon <start|stop|status> Gère le daemon Docker
  dashboard                Lance le dashboard interactif

Exemples:
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
	return mgr.StartProject(targetProject)
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
	return mgr.StopProject(targetProject)
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

	// Si pas de service spécifié, on redémarre le projet entier
	if serviceName == "" {
		fmt.Printf("🔄 Redémarrage complet du projet %s...\n", projectName)
		// On peut implémenter un true restart ici
		return mgr.RestartService(targetProject, "")
	}

	return mgr.RestartService(targetProject, serviceName)
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

	for _, p := range projects {
		running, count, _ := mgr.GetStatus(&p)

		if running {
			fmt.Printf("  %-20s ▶  Running (%d services)\n", p.Name, count)
		} else {
			fmt.Printf("  %-20s ⏹  Stopped\n", p.Name)
		}
	}

	fmt.Println("─────────────────────────────────────────\n")
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

	fmt.Println("─────────────────────────────────────────\n")
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
	for i := range projects {
		running, count, _ := mgr.GetStatus(&projects[i])
		projects[i].Running = running
		projects[i].ServiceCount = count
	}

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

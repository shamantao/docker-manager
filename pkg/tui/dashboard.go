package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phil/docker-manager/pkg/docker"
	"github.com/phil/docker-manager/pkg/project"
)

const maxLogLines = 8

// Model est le modèle Bubble Tea pour le dashboard
type Model struct {
	projects  []project.Project
	selected  int
	manager   *docker.Manager
	message   string
	width     int
	height    int
	loading   bool
	lastError string
	spinner   spinner.Model
	logLines  []string // dernières lignes de sortie docker-compose
	refresh   func() []project.Project
}

// NewModel crée un nouveau modèle de dashboard
func NewModel(projects []project.Project, manager *docker.Manager) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Model{
		projects: projects,
		selected: 0,
		manager:  manager,
		message:  "Bienvenue dans Docker Manager",
		spinner:  s,
	}
}

// SetRefresh fournit au dashboard le moyen de recharger l'inventaire (touche R majuscule).
func (m *Model) SetRefresh(fn func() []project.Project) {
	m.refresh = fn
}

// Init initialise le modèle
func (m Model) Init() tea.Cmd {
	return nil
}

// Pour le streaming temps réel, on utilise un channel + listenForOutput
func (m *Model) runStreamingOperation(action string) tea.Cmd {
	p := m.projects[m.selected]
	idx := m.selected
	mgr := m.manager

	// Channel partagé pour les lignes de sortie
	ch := make(chan string, 64)
	m.logLines = nil

	// Lancer l'opération dans une goroutine
	go func() {
		defer close(ch)
		var opErr error
		compose := p.DockerComposeExists()
		switch action {
		case "start":
			if compose {
				opErr = mgr.StartProjectStream(&p, ch)
			} else {
				opErr = mgr.StartContainersStream(&p, ch)
			}
		case "stop":
			if compose {
				opErr = mgr.StopProjectStream(&p, ch)
			} else {
				opErr = mgr.StopContainersStream(&p, ch)
			}
		case "restart":
			if compose {
				opErr = mgr.RestartServiceStream(&p, ch, "")
			} else {
				opErr = mgr.RestartContainersStream(&p, ch)
			}
		case "update":
			opErr = mgr.UpdateProjectStream(&p, ch)
		}
		// Envoyer le résultat final dans le channel
		if opErr != nil {
			ch <- "\x00ERR:" + opErr.Error()
		}
		_ = idx
	}()

	// Retourner une commande qui écoute le channel
	return listenForOutput(ch, idx, action)
}

// streamMsg reçoit les lignes de sortie en temps réel
type streamMsg struct {
	line   string
	idx    int
	action string
	ch     chan string
	done   bool
	err    string
}

// listenForOutput crée une commande Bubble Tea qui lit le channel
func listenForOutput(ch chan string, idx int, action string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return streamMsg{done: true, idx: idx, action: action}
		}
		// Vérifier si c'est une erreur finale
		if strings.HasPrefix(line, "\x00ERR:") {
			return streamMsg{done: true, idx: idx, action: action, err: strings.TrimPrefix(line, "\x00ERR:")}
		}
		return streamMsg{line: line, ch: ch, idx: idx, action: action}
	}
}

// Update gère les mises à jour du modèle
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case streamMsg:
		if msg.done {
			m.loading = false
			if msg.err != "" {
				m.lastError = msg.err
			} else {
				name := m.projects[msg.idx].Name
				switch msg.action {
				case "start":
					m.message = fmt.Sprintf("✅ Projet %s démarré", name)
					m.projects[msg.idx].Running = true
					if m.projects[msg.idx].TotalCount > 0 {
						m.projects[msg.idx].RunningCount = m.projects[msg.idx].TotalCount
					}
				case "stop":
					m.message = fmt.Sprintf("✅ Projet %s arrêté", name)
					m.projects[msg.idx].Running = false
					m.projects[msg.idx].RunningCount = 0
				case "restart":
					m.message = fmt.Sprintf("✅ Projet %s redémarré", name)
				case "update":
					m.message = fmt.Sprintf("✅ Images de %s à jour", name)
				}
				m.reload()
			}
			return m, nil
		}
		// Ajouter la ligne au log
		m.logLines = append(m.logLines, msg.line)
		if len(m.logLines) > maxLogLines {
			m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		}
		// Continuer à écouter
		return m, tea.Batch(m.spinner.Tick, listenForOutput(msg.ch, msg.idx, msg.action))

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		// Ignorer les touches pendant le chargement (sauf quit)
		if m.loading {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.projects)-1 {
				m.selected++
			}

		case "s":
			if m.selected < len(m.projects) {
				p := m.projects[m.selected]
				if !p.Manageable() {
					m.lastError = "rien à démarrer : ni fichier compose ni container existant"
					break
				}
				m.loading = true
				m.lastError = ""
				m.logLines = nil
				m.message = fmt.Sprintf("⏳ Démarrage de %s...", p.Name)
				return m, tea.Batch(m.spinner.Tick, m.runStreamingOperation("start"))
			}

		case "d":
			if m.selected < len(m.projects) {
				p := m.projects[m.selected]
				m.loading = true
				m.lastError = ""
				m.logLines = nil
				m.message = fmt.Sprintf("⏳ Arrêt de %s...", p.Name)
				return m, tea.Batch(m.spinner.Tick, m.runStreamingOperation("stop"))
			}

		case "r":
			if m.selected < len(m.projects) {
				p := m.projects[m.selected]
				if !p.Manageable() {
					m.lastError = "rien à redémarrer : ni fichier compose ni container existant"
					break
				}
				m.loading = true
				m.lastError = ""
				m.logLines = nil
				m.message = fmt.Sprintf("⏳ Redémarrage de %s...", p.Name)
				return m, tea.Batch(m.spinner.Tick, m.runStreamingOperation("restart"))
			}

		case "u":
			if m.selected < len(m.projects) {
				p := m.projects[m.selected]
				if !p.Manageable() {
					m.lastError = "rien à mettre à jour pour ce projet"
					break
				}
				m.loading = true
				m.lastError = ""
				m.logLines = nil
				m.message = fmt.Sprintf("⏳ Mise à jour des images de %s...", p.Name)
				return m, tea.Batch(m.spinner.Tick, m.runStreamingOperation("update"))
			}

		case "R", "f5":
			m.reload()
			m.message = "🔄 Inventaire rechargé"
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// reload relit l'inventaire de la machine en conservant la sélection courante
func (m *Model) reload() {
	if m.refresh == nil {
		return
	}
	name := ""
	if m.selected < len(m.projects) {
		name = m.projects[m.selected].Name
	}
	m.projects = m.refresh()
	m.selected = 0
	for i := range m.projects {
		if m.projects[i].Name == name {
			m.selected = i
			break
		}
	}
	if m.selected >= len(m.projects) {
		m.selected = 0
	}
}

// View affiche le dashboard
func (m Model) View() string {
	// Pas de garde sur la taille : certains terminaux (SSH, tmux, pty minimal)
	// n'envoient pas de WindowSizeMsg immédiatement, voire jamais.
	// Styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Margin(1, 0)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Bold(true).
		Margin(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("5")).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Padding(0, 1)

	// Titre
	title := titleStyle.Render("🐳 Docker Manager")

	// Affichage des projets
	projectLines := ""
	if len(m.projects) == 0 {
		projectLines = normalStyle.Render("  (aucun projet ni container détecté sur cette machine)") + "\n"
	}
	for i, p := range m.projects {
		status := p.StatusString()
		line := fmt.Sprintf("  %-24s  %s", p.Name, status)

		if i == m.selected {
			projectLines += selectedStyle.Render(line) + "\n"
		} else {
			projectLines += normalStyle.Render(line) + "\n"
		}
	}

	// Message de statut (avec spinner si loading)
	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Margin(1, 0, 0, 1)

	statusLine := m.message
	if m.loading {
		statusLine = m.spinner.View() + " " + m.message
	}
	statusText := messageStyle.Render(statusLine)

	// Zone de log (sortie docker-compose en temps réel)
	logText := ""
	if len(m.logLines) > 0 {
		logStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Margin(0, 0, 0, 3)
		logText = "\n" + logStyle.Render(strings.Join(m.logLines, "\n"))
	}

	// Erreur si présente
	errorText := ""
	if m.lastError != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Margin(1, 0, 0, 1)
		errorText = errorStyle.Render("❌ " + m.lastError)
	}

	// Commandes
	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Margin(1, 0, 0, 1)

	commands := cmdStyle.Render("[s]tart  [d]own/stop  [r]estart  [u]pdate images  [R]efresh  [q]uit")

	return fmt.Sprintf("%s\n\n%s\n%s\n%s%s%s\n%s\n", title, headerStyle.Render("Projects:"), projectLines, statusText, logText, errorText, commands)
}

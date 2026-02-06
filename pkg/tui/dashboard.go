package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phil/docker-manager/pkg/docker"
	"github.com/phil/docker-manager/pkg/project"
)

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
}

// NewModel crée un nouveau modèle de dashboard
func NewModel(projects []project.Project, manager *docker.Manager) *Model {
	return &Model{
		projects: projects,
		selected: 0,
		manager:  manager,
		message:  "Bienvenue dans Docker Manager",
	}
}

// Init initialise le modèle
func (m Model) Init() tea.Cmd {
	return nil
}

// Update gère les mises à jour du modèle
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
				m.loading = true
				if err := m.manager.StartProject(&p); err != nil {
					m.lastError = err.Error()
				} else {
					m.message = fmt.Sprintf("✅ Projet %s démarré", p.Name)
				}
				m.loading = false
			}

		case "d":
			if m.selected < len(m.projects) {
				p := m.projects[m.selected]
				m.loading = true
				if err := m.manager.StopProject(&p); err != nil {
					m.lastError = err.Error()
				} else {
					m.message = fmt.Sprintf("✅ Projet %s arrêté", p.Name)
				}
				m.loading = false
			}

		case "r":
			if m.selected < len(m.projects) {
				p := m.projects[m.selected]
				m.loading = true
				if err := m.manager.RestartService(&p, ""); err != nil {
					m.lastError = err.Error()
				} else {
					m.message = fmt.Sprintf("✅ Projet %s redémarré", p.Name)
				}
				m.loading = false
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View affiche le dashboard
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initialisation du terminal..."
	}

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
	for i, p := range m.projects {
		status := p.StatusString()
		line := fmt.Sprintf("  %-20s  %s", p.Name, status)

		if i == m.selected {
			projectLines += selectedStyle.Render(line) + "\n"
		} else {
			projectLines += normalStyle.Render(line) + "\n"
		}
	}

	// Message de statut
	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Margin(1, 0, 0, 1)

	statusText := messageStyle.Render(m.message)

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

	commands := cmdStyle.Render("[S]tart  [D]rop  [R]estart  [U]p/[D]own  [Q]uit")

	return fmt.Sprintf("%s\n\n%s%s\n%s%s\n%s\n", title, headerStyle.Render("Projects:"), projectLines, statusText, errorText, commands)
}

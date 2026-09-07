package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phil/docker-manager/pkg/docker"
	"github.com/phil/docker-manager/pkg/project"
)

func testModel() *Model {
	projects := []project.Project{
		{Name: "media", RunningCount: 1, TotalCount: 2, Running: true},
		{Name: "portainer", Standalone: true, RunningCount: 1, TotalCount: 1, Running: true},
		{Name: "fantome"}, // ni compose ni container : rien à piloter
	}
	return NewModel(projects, docker.NewManager(""))
}

func press(m tea.Model, key string) tea.Model {
	var msg tea.Msg
	if len(key) == 1 {
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	} else {
		msg = tea.KeyMsg{Type: tea.KeyDown}
	}
	next, _ := m.Update(msg)
	return next
}

// Le dashboard doit s'afficher même si le terminal n'annonce pas sa taille
// (SSH vers la Raspberry, pty minimal, tmux...).
func TestViewRendersWithoutWindowSize(t *testing.T) {
	view := testModel().View()
	for _, want := range []string{"Docker Manager", "media", "◐ Partiel (1/2)", "portainer", "⬦", "[u]pdate"} {
		if !strings.Contains(view, want) {
			t.Errorf("vue sans %q:\n%s", want, view)
		}
	}
}

func TestNavigation(t *testing.T) {
	m := tea.Model(*testModel())
	m = press(m, "j")
	m = press(m, "j")
	if got := m.(Model).selected; got != 2 {
		t.Errorf("après 2x j: attendu index 2, obtenu %d", got)
	}
	m = press(m, "k")
	if got := m.(Model).selected; got != 1 {
		t.Errorf("après k: attendu index 1, obtenu %d", got)
	}
	// Pas de débordement en bas de liste
	m = press(m, "j")
	m = press(m, "j")
	if got := m.(Model).selected; got != 2 {
		t.Errorf("débordement: attendu index 2, obtenu %d", got)
	}
}

// Un projet sans compose ni container ne doit lancer aucune commande docker.
func TestGuardOnUnmanageableProject(t *testing.T) {
	m := tea.Model(*testModel())
	m = press(m, "j")
	m = press(m, "j") // "fantome"

	for _, key := range []string{"s", "r", "u"} {
		next := press(m, key)
		model := next.(Model)
		if model.loading {
			t.Errorf("touche %q: une opération a été lancée sur un projet non pilotable", key)
		}
		if model.lastError == "" {
			t.Errorf("touche %q: aucune erreur expliquée à l'utilisateur", key)
		}
	}
}

func TestRefreshKeyReloadsInventory(t *testing.T) {
	base := testModel()
	called := 0
	base.SetRefresh(func() []project.Project {
		called++
		return []project.Project{{Name: "portainer", Standalone: true, RunningCount: 1, TotalCount: 1}}
	})

	m := press(tea.Model(*base), "j") // sélection sur "portainer"
	next := press(m, "R").(Model)

	if called != 1 {
		t.Fatalf("refresh appelé %d fois, attendu 1", called)
	}
	if len(next.projects) != 1 {
		t.Fatalf("inventaire non rechargé: %d projets", len(next.projects))
	}
	// La sélection doit suivre le projet, pas l'index
	if next.projects[next.selected].Name != "portainer" {
		t.Errorf("sélection perdue après rechargement: %q", next.projects[next.selected].Name)
	}
}

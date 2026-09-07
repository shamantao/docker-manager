package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phil/docker-manager/pkg/project"
)

func writeCompose(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("services: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverFindsNestedProjects(t *testing.T) {
	root := t.TempDir()
	writeCompose(t, filepath.Join(root, "docker-pbwww"), "docker-compose.yml")
	writeCompose(t, filepath.Join(root, "media"), "compose.yaml")
	writeCompose(t, filepath.Join(root, "stacks", "pihole"), "docker-compose.yaml")
	// Ne doit pas être trouvé : trop profond et dossiers ignorés
	writeCompose(t, filepath.Join(root, "a", "b", "c"), "docker-compose.yml")
	writeCompose(t, filepath.Join(root, "node_modules", "x"), "docker-compose.yml")
	writeCompose(t, filepath.Join(root, ".cache", "y"), "docker-compose.yml")

	found, err := NewDiscoverer(root).Discover()
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, p := range found {
		names[p.Name] = true
	}

	for _, want := range []string{"pbwww", "media", "pihole"} {
		if !names[want] {
			t.Errorf("projet %q non découvert (trouvés: %v)", want, names)
		}
	}
	if names["c"] || names["x"] || names["y"] {
		t.Errorf("dossiers hors périmètre découverts: %v", names)
	}
}

func TestMatchProjectByPathThenName(t *testing.T) {
	projects := []project.Project{
		{Name: "pbwww", Path: "/srv/docker/docker-pbwww"},
		{Name: "media", Path: "/srv/docker/media"},
	}

	// Le projet compose s'appelle "docker-pbwww" (nom du dossier) mais l'outil
	// le connaît sous "pbwww" : le chemin doit permettre de les rapprocher.
	inv := project.Project{Name: "docker-pbwww", Path: "/srv/docker/docker-pbwww/"}
	if got := matchProject(projects, inv); got != 0 {
		t.Errorf("rapprochement par chemin: attendu 0, obtenu %d", got)
	}

	// Sans chemin, le préfixe "docker-" ne doit pas empêcher le rapprochement
	if got := matchProject(projects, project.Project{Name: "docker-pbwww"}); got != 0 {
		t.Errorf("rapprochement par nom normalisé: attendu 0, obtenu %d", got)
	}

	// Projet réellement inconnu
	if got := matchProject(projects, project.Project{Name: "portainer"}); got != -1 {
		t.Errorf("projet inconnu: attendu -1, obtenu %d", got)
	}
}

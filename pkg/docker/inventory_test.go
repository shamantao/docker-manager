package docker

import (
	"os"
	"path/filepath"
	"testing"
)

// Le container standalone est volontairement en première ligne : ses labels
// compose vides commencent par des tabulations, qu'un trim trop large effacerait.
const psOutput = "\t\t\tportainer\trunning\tportainer/portainer-ce\n" +
	"media\t/srv/docker/media\t/srv/docker/media/docker-compose.yml\tmedia-jellyfin-1\trunning\tjellyfin/jellyfin:latest\n" +
	"media\t/srv/docker/media\t/srv/docker/media/docker-compose.yml\tmedia-transmission-1\texited\tlinuxserver/transmission\n" +
	"pihole\t/home/pi/docker/pihole\t/home/pi/docker/pihole/compose.yaml\tpihole\trunning\tpihole/pihole:latest\n" +
	"\t\t\tvieux-test\tcreated\talpine\n"

func TestParseContainerRows(t *testing.T) {
	rows := parseContainerRows(psOutput)
	if len(rows) != 5 {
		t.Fatalf("attendu 5 lignes, obtenu %d", len(rows))
	}
	if rows[1].Project != "media" || rows[1].WorkingDir != "/srv/docker/media" {
		t.Errorf("labels compose mal parsés: %+v", rows[1])
	}
	// Première ligne : labels vides, ne doit pas être décalée
	if rows[0].Project != "" || rows[0].Name != "portainer" || rows[0].State != "running" {
		t.Errorf("container standalone en tête mal parsé: %+v", rows[0])
	}
}

func TestBuildInventory(t *testing.T) {
	projects := buildInventory(parseContainerRows(psOutput))
	if len(projects) != 4 {
		t.Fatalf("attendu 4 projets (2 compose + 2 standalone), obtenu %d", len(projects))
	}

	byName := map[string]int{}
	for i, p := range projects {
		byName[p.Name] = i
	}

	// Projet compose partiellement démarré : les containers arrêtés comptent
	media := projects[byName["media"]]
	if media.TotalCount != 2 || media.RunningCount != 1 {
		t.Errorf("media: attendu 2 containers dont 1 actif, obtenu %d/%d", media.RunningCount, media.TotalCount)
	}
	if media.Path != "/srv/docker/media" {
		t.Errorf("media: chemin non retrouvé depuis les labels: %q", media.Path)
	}
	if media.Standalone {
		t.Error("media ne devrait pas être marqué standalone")
	}
	if got := media.StatusString(); got != "◐ Partiel (1/2)" {
		t.Errorf("media: statut inattendu %q", got)
	}

	// Container standalone arrêté : visible, donc gérable
	vieux := projects[byName["vieux-test"]]
	if !vieux.Standalone || vieux.TotalCount != 1 || vieux.RunningCount != 0 {
		t.Errorf("vieux-test: %+v", vieux)
	}
	if !vieux.Manageable() {
		t.Error("un container arrêté doit rester gérable (start possible)")
	}
	if got := vieux.StatusString(); got != "⏸ Installé, arrêté (1) ⬦" {
		t.Errorf("vieux-test: statut inattendu %q", got)
	}
}

func TestResolveComposePath(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Chemin absolu fourni par le label
	if got := resolveComposePath(dir, composeFile); got != composeFile {
		t.Errorf("label absolu: attendu %q, obtenu %q", composeFile, got)
	}
	// Chemin relatif au working_dir (compose plus ancien)
	if got := resolveComposePath(dir, "compose.yml"); got != composeFile {
		t.Errorf("label relatif: attendu %q, obtenu %q", composeFile, got)
	}
	// Plusieurs fichiers : on prend le premier
	if got := resolveComposePath(dir, composeFile+",override.yml"); got != composeFile {
		t.Errorf("labels multiples: attendu %q, obtenu %q", composeFile, got)
	}
	// Label absent : on retombe sur la détection dans le dossier
	if got := resolveComposePath(dir, ""); got != composeFile {
		t.Errorf("sans label: attendu %q, obtenu %q", composeFile, got)
	}
	// Dossier disparu (projet déplacé) : aucun chemin
	if got := resolveComposePath("/chemin/inexistant", "/chemin/inexistant/docker-compose.yml"); got != "" {
		t.Errorf("chemin inexistant: attendu \"\", obtenu %q", got)
	}
}

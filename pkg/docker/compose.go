package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// composeFileNames liste les noms de fichiers compose reconnus, par ordre de priorité.
var composeFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

var (
	composeOnce sync.Once
	composeBin  []string
)

// ComposeCommand retourne la commande compose disponible sur la machine.
// Docker Compose v2 (plugin "docker compose") est préféré ; on retombe sur
// le binaire v1 "docker-compose" s'il est seul présent.
// Le résultat est détecté une fois puis mis en cache.
func ComposeCommand() []string {
	composeOnce.Do(func() {
		if err := exec.Command("docker", "compose", "version").Run(); err == nil {
			composeBin = []string{"docker", "compose"}
			return
		}
		if _, err := exec.LookPath("docker-compose"); err == nil {
			composeBin = []string{"docker-compose"}
			return
		}
		// Aucun des deux détecté : on garde v2, l'erreur remontera à l'exécution.
		composeBin = []string{"docker", "compose"}
	})
	return composeBin
}

// ComposeCommandString retourne la commande compose sous forme de chaîne shell.
func ComposeCommandString() string {
	return shelljoin(ComposeCommand())
}

// FindComposeFile retourne le chemin du fichier compose présent dans dir,
// ou "" si aucun n'est trouvé.
func FindComposeFile(dir string) string {
	for _, name := range composeFileNames {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

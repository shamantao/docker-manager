#!/usr/bin/env bash
# Installation portable de docker-manager (macOS / Linux / Raspberry Pi).
#
# Détecte l'OS et l'architecture, utilise un binaire pré-compilé s'il est
# présent à côté du script, sinon compile depuis les sources (Go requis).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFIX="${PREFIX:-/usr/local/bin}"
TARGET="$PREFIX/docker-manager"

# --- Détection OS / architecture ---
case "$(uname -s)" in
  Darwin) GOOS=darwin ;;
  Linux)  GOOS=linux ;;
  *) echo "❌ OS non supporté: $(uname -s)"; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) GOARCH=arm64 ;;
  x86_64|amd64)  GOARCH=amd64 ;;
  armv7l|armv6l) GOARCH=arm ;;   # Raspberry Pi 32 bits
  *) echo "❌ Architecture non supportée: $(uname -m)"; exit 1 ;;
esac

echo "🖥️  Cible détectée : ${GOOS}/${GOARCH}"

BINARY="$SCRIPT_DIR/docker-manager-${GOOS}-${GOARCH}"

# --- Binaire pré-compilé, sinon compilation ---
if [ -f "$BINARY" ]; then
  echo "📦 Binaire pré-compilé trouvé : $(basename "$BINARY")"
elif command -v go >/dev/null 2>&1; then
  echo "🔨 Compilation depuis les sources..."
  (cd "$SCRIPT_DIR" && GOOS=$GOOS GOARCH=$GOARCH go build -o "$BINARY" .)
else
  echo "❌ Ni binaire pré-compilé pour ${GOOS}/${GOARCH}, ni Go installé."
  echo "   Sur votre machine de développement : make release"
  echo "   puis copiez docker-manager-${GOOS}-${GOARCH} à côté de ce script."
  exit 1
fi

# --- Installation ---
SUDO=""
if [ -d "$PREFIX" ] && [ -w "$PREFIX" ]; then
  echo "📦 Installation dans $PREFIX..."
elif [ ! -d "$PREFIX" ] && mkdir -p "$PREFIX" 2>/dev/null; then
  echo "📦 Répertoire $PREFIX créé."
else
  SUDO="sudo"
  echo "📦 Installation dans $PREFIX (mot de passe demandé)..."
  $SUDO mkdir -p "$PREFIX"
fi

$SUDO install -m 0755 "$BINARY" "$TARGET"

case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *) echo "⚠️  $PREFIX n'est pas dans votre PATH."
     echo "   Ajoutez : export PATH=\"$PREFIX:\$PATH\"" ;;
esac

echo "✅ docker-manager installé : $TARGET"

# --- Vérifications d'environnement ---
if ! command -v docker >/dev/null 2>&1; then
  echo "⚠️  Docker n'est pas installé sur cette machine."
elif ! docker info >/dev/null 2>&1; then
  echo "⚠️  Le daemon Docker n'est pas joignable."
  if [ "$GOOS" = "linux" ]; then
    echo "   Daemon arrêté   : sudo systemctl start docker"
    echo "   Accès refusé    : sudo usermod -aG docker \$USER (puis reconnexion)"
  fi
fi

echo "🚀 Testez avec : docker-manager status"

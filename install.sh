#!/bin/bash

# Script d'installation pour docker-manager
set -e

echo "🔨 Compilation de docker-manager pour macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -o docker-manager-arm64

echo "📦 Installation dans /usr/local/bin (peut demander votre mot de passe)..."
sudo cp docker-manager-arm64 /usr/local/bin/docker-manager
sudo chmod +x /usr/local/bin/docker-manager

echo "✅ docker-manager a été installé avec succès!"
echo "🚀 Testez avec: docker-manager --help"

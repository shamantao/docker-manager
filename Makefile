# Docker Manager - Makefile

.PHONY: help build run test clean install deps release \
        darwin-arm64 darwin-amd64 linux-arm64 linux-arm linux-amd64

help:
	@echo "Docker Manager Build Commands"
	@echo "=============================="
	@echo "make build          - Build pour l'OS/arch courant"
	@echo "make install        - Compile et installe (via install.sh)"
	@echo "make release        - Build tous les binaires (macOS + Linux/Pi)"
	@echo "make linux-arm64    - Build Raspberry Pi 4/5 (64 bits)"
	@echo "make linux-arm      - Build Raspberry Pi 32 bits"
	@echo "make run            - Exécute le dashboard"
	@echo "make test           - Lance les tests"
	@echo "make clean          - Nettoie les fichiers compilés"
	@echo "make deps           - Télécharge les dépendances"

deps:
	go mod download
	go mod tidy

build: deps
	go build -o docker-manager -v .

darwin-arm64: deps
	GOOS=darwin GOARCH=arm64 go build -o docker-manager-darwin-arm64 .

darwin-amd64: deps
	GOOS=darwin GOARCH=amd64 go build -o docker-manager-darwin-amd64 .

# Raspberry Pi 4/5 sous OS 64 bits (aarch64)
linux-arm64: deps
	GOOS=linux GOARCH=arm64 go build -o docker-manager-linux-arm64 .

# Raspberry Pi sous OS 32 bits (armv7)
linux-arm: deps
	GOOS=linux GOARCH=arm GOARM=7 go build -o docker-manager-linux-arm .

linux-amd64: deps
	GOOS=linux GOARCH=amd64 go build -o docker-manager-linux-amd64 .

# Tous les binaires distribuables
release: darwin-arm64 darwin-amd64 linux-arm64 linux-arm linux-amd64
	@echo "✅ Binaires prêts :"
	@ls -1 docker-manager-*

# Installe sur la machine courante (détecte OS/arch)
install:
	@./install.sh

run: build
	./docker-manager dashboard

test:
	go test -v ./...

clean:
	rm -f docker-manager docker-manager-* 
	go clean

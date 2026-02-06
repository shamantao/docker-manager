# Docker Manager - Makefile

.PHONY: help build run test clean install darwin-arm64

help:
	@echo "Docker Manager Build Commands"
	@echo "=============================="
	@echo "make build          - Build pour macOS current arch"
	@echo "make darwin-arm64   - Build pour M1 (ARM64)"
	@echo "make install        - Compile et installe dans /usr/local/bin"
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
	GOOS=darwin GOARCH=arm64 go build -o docker-manager-arm64 -v .

install: darwin-arm64
	cp docker-manager-arm64 /usr/local/bin/docker-manager
	chmod +x /usr/local/bin/docker-manager
	@echo "✅ docker-manager installé dans /usr/local/bin"

run: build
	./docker-manager dashboard

test:
	go test -v ./...

clean:
	rm -f docker-manager docker-manager-arm64
	go clean

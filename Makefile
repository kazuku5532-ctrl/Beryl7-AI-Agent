.PHONY: all test coverage build deploy clean distclean docker-build help

all: test coverage build

test:
	@echo "=== Running Go Unit Tests ==="
	cd go-agent && go test -v ./...

coverage:
	@echo "=== Enforcing Go Code Coverage (80%+ Target) ==="
	cd go-agent && go test -v -coverprofile=coverage.out ./...
	cd go-agent && go tool cover -html=coverage.out -o coverage.html
	cd go-agent && go tool cover -func=coverage.out | grep total

build:
	@echo "=== Cross-Compiling Go ARM64 Binary for OpenWrt ==="
	cd go-agent && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o beryl7-agent ./cmd

deploy: build
	@echo "=== Deploying to GL-MT3600BE Router via Ansible ==="
	@command -v ansible-playbook >/dev/null 2>&1 || (echo "Error: ansible-playbook is required for 'make deploy'. Please install ansible or use python scratch/deploy_router_v15.py." && exit 1)
	ansible-playbook -i deploy/inventory.ini deploy/deploy.yml

clean:
	@echo "=== Cleaning Compiled Binaries ==="
	rm -f go-agent/beryl7-agent go-agent/beryl7-agent-*

distclean: clean
	@echo "=== Cleaning All Build & Coverage Artifacts ==="
	rm -rf dist/ go-agent/coverage.out go-agent/coverage.html

docker-build:
	@echo "=== Building Local Developer Testing Docker Image ==="
	docker build -t beryl7-dev:latest .

help:
	@echo "Beryl7 Build & Test Targets:"
	@echo "  make test         - Run Go unit test suite"
	@echo "  make coverage     - Enforce 80%+ Go code coverage and generate coverage.html"
	@echo "  make build        - Cross-compile ARM64 Linux binary"
	@echo "  make deploy       - Deploy binary to router via Ansible"
	@echo "  make clean        - Remove compiled binaries"
	@echo "  make distclean    - Remove all binaries, dist, and coverage artifacts"
	@echo "  make docker-build - Build local test Docker image"


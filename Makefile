.PHONY: all test coverage build deploy help

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
	cd go-agent && GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd

deploy: build
	@echo "=== Deploying to GL-MT3600BE Router via Ansible ==="
	ansible-playbook -i deploy/inventory.ini deploy/deploy.yml

help:
	@echo "Beryl7 Build & Test Targets:"
	@echo "  make test      - Run Go unit test suite"
	@echo "  make coverage  - Enforce 80%+ Go code coverage and generate coverage.html"
	@echo "  make build     - Cross-compile ARM64 Linux binary"
	@echo "  make deploy    - Deploy binary to router via Ansible"

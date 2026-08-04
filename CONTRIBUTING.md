# Contributing Guidelines 🤝

Thank you for considering contributing to the Beryl 7 AI Agent project!

---

## 🛠️ Development Setup & Guidelines

### 1. Prerequisites
- **Go 1.21+** installed on your workstation.
- **Git** configured with your developer name and email.
- **OpenWrt GL-MT3600BE Router** (or local test suite for development).

### 2. Branch Naming Conventions
Always create feature branches off the `main` branch:
- `feature/description` — For new functionality or architectural additions.
- `fix/description` — For bug fixes or vulnerability remediations.
- `docs/description` — For documentation updates or guide improvements.
- `refactor/description` — For code cleanup without functional changes.

---

## 📝 Commit Message Format (Conventional Commits)

Commit messages must follow the [Conventional Commits specification](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary in present tense>

[optional body providing technical rationale]
```

### Supported Types:
- `feat`: A new feature added to the daemon or tools.
- `fix`: A bug fix or error handling remediation.
- `docs`: Documentation changes only.
- `test`: Adding or updating unit/integration/stress tests.
- `refactor`: Code changes that neither fix a bug nor add a feature.
- `chore`: Maintenance tasks, dependencies, or build scripts.

---

## 🧪 Testing & Quality Gate Requirements

Before opening a Pull Request:
1. Format all Go code: `gofmt -s -w ./...`
2. Run the complete test suite:
   ```bash
   cd go-agent
   go test -v ./...
   ```
3. Verify test coverage meets the **80% Quality Gate**:
   ```bash
   go test -coverpkg=./... -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out
   ```
4. Verify cross-compilation builds cleanly:
   ```bash
   GOOS=linux GOARCH=arm64 go build -o beryl7-agent ./cmd
   ```

---

## 📬 Pull Request Workflow

1. Fork the repository and create your branch from `main`.
2. Ensure all tests pass and documentation is updated.
3. Open a Pull Request targeting `main`.
4. Fill in the PR template describing the changes and test results.
5. All PRs require passing GitHub Actions CI checks before merging.

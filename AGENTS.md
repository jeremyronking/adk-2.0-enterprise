# Enterprise AI Agent Rules & Directives

All AI coding assistants working in this codebase MUST strictly follow project conventions:

1. **Architecture Standards**:
   - Refer to [docs/architecture.md](docs/architecture.md) for agent composition and service boundaries.
   - Maintain decoupling between `cmd/` entry points and `internal/` domain logic.
   - The Command Center UI exists as three byte-identical copies. Edit
     `web/index.html`, then copy it to `cmd/router/web/index.html` (embedded
     into the binary) and `docs/command_center_demo.html`. Never let them drift.
2. **Coding Standards**:
   - Refer to [docs/coding_conventions.md](docs/coding_conventions.md) before refactoring.
   - Struct input fields must specify `json` and `jsonschema` tags.
3. **Deployment**:
   - Refer to [docs/deployment.md](docs/deployment.md) for Cloud Run containerization and GCP Agent Registry standards.
4. **Quality Gate**:
   - Run `make check` (`gofmt`, `go vet`, `go test ./...`) after modifications.

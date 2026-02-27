# Project Overview

`terraclaw` is a Go-based interactive terminal UI (TUI) tool that converts existing cloud infrastructure into Terraform/OpenTofu configuration using AI. It connects to a running [Steampipe](https://steampipe.io/) instance to discover live cloud resources, lets the user select resources via a [BubbleTea](https://github.com/charmbracelet/bubbletea)-powered interface, calls a chosen LLM (OpenAI, Anthropic Claude, Google Gemini, or Azure OpenAI via Azure AI Foundry) to generate valid Terraform HCL, writes the result to disk, and optionally runs `terraform import` to create the corresponding state—all without leaving the terminal.

---

## Repository Structure

```
terraclaw/
├── main.go                          Entry point — delegates directly to cmd.Execute()
├── go.mod / go.sum                  Go module definition and lockfile
├── config/
│   ├── config.go                    Config struct, Load(), Validate(), SteampipeConnStr()
│   └── config_test.go               Unit tests for config loading and validation
├── cmd/
│   ├── root.go                      Cobra root command; wires config → Steampipe → TUI
│   └── doctor.go                    `terraclaw doctor` subcommand; delegates to internal/doctor
└── internal/
    ├── doctor/                      Dependency-check logic (binaries, API keys, Steampipe reachability)
    ├── steampipe/                   PostgreSQL client wrapping Steampipe; ListSchemas, ListTables, ListResources
    ├── llm/
    │   ├── provider.go              Provider interface + factory (New) + shared prompt builder
    │   ├── openai.go                OpenAI (GPT-4o) implementation
    │   ├── azure_openai.go          Azure OpenAI / Azure AI Foundry implementation
    │   ├── claude.go                Anthropic Claude implementation
    │   └── gemini.go                Google Gemini implementation
    ├── terraform/
    │   ├── generator.go             WriteConfig (HCL file writer) + GuessResourceAddress
    │   └── importer.go              Runs `terraform import` as a subprocess
    └── tui/
        ├── model.go                 BubbleTea Model + all View/Update logic; wizard step machine
        ├── commands.go              Async tea.Cmd definitions (fetch tables, generate code, run import)
        └── styles.go                Lipgloss style constants (titleStyle, codeStyle, errorStyle, …)
```

**Key design principles:**
- `internal/` packages are not importable outside the module — keep all domain logic there.
- `config` is the only package imported by both `cmd` and `internal/*`; do not import `cmd` from `internal`.
- The TUI (`internal/tui`) is wired up in `cmd/root.go` via `tui.SetConfig` and `tui.SetSteampipeClient`; avoid adding more global setters.

---

## Dependencies and Installation

**Required Go version:** Go 1.21 or later (module declares `go 1.24.12`).

**Key direct dependencies (managed via Go modules):**

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/bubbles` | List, spinner, and other TUI components |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/spf13/cobra` | CLI command parsing |
| `github.com/joho/godotenv` | `.env` file loading |
| `github.com/lib/pq` | PostgreSQL driver for Steampipe |
| `github.com/sashabaranov/go-openai` | OpenAI API client |
| `github.com/anthropics/anthropic-sdk-go` | Anthropic Claude API client |
| `google.golang.org/genai` | Google Gemini API client |

**Install / build:**

```bash
# Install to $GOPATH/bin
go install .

# Or build a local binary
go build -o terraclaw .

# Fetch / tidy dependencies
go mod tidy
```

**External runtime dependencies** (not Go packages — must be on PATH):
- [`steampipe`](https://steampipe.io/downloads) with at least one cloud plugin (e.g. `steampipe plugin install aws`)
- [`terraform`](https://developer.hashicorp.com/terraform/downloads) (or OpenTofu; set `TERRAFORM_BIN` env var to override)

---

## Testing Instructions

Tests live alongside the packages they test, in `_test.go` files using the `package foo_test` external test pattern.

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests in a specific package
go test ./config/...

# Run a specific test function
go test -run TestValidate ./config/...
```

**Notes:**
- Tests use only the standard `testing` package — no test framework like `testify`.
- Unit tests manipulate environment variables via `t.Setenv` and `os.Unsetenv`; never use `os.Setenv` directly in tests (it won't be cleaned up automatically).
- There are currently no integration tests requiring a live Steampipe or Terraform binary. Do not add tests that require external services without a clear mock or skip guard (`t.Skip`).
- Run `terraclaw doctor` (not a Go test) to validate the runtime environment before manual testing.

---

## Code Style and Conventions

Follow standard Go idioms and the conventions already present in the codebase:

### General
- **`gofmt` / `goimports`**: All code must be formatted. Run `gofmt -w .` or `goimports -w .` before committing.
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` — never discard or silent-swallow errors.
- **Named return values**: Avoid them; prefer explicit `return` statements for clarity.
- **Comments**: Every exported symbol must have a Go doc comment (`// TypeName ...`). Unexported helpers only need comments when behaviour is non-obvious.
- **Avoid `init()` side-effects**: The only acceptable `init()` use is Cobra command registration (see `cmd/doctor.go`).

### Packages
- Keep `internal/` packages focused on a single responsibility (one provider per file in `internal/llm/`).
- New LLM providers: implement the `llm.Provider` interface (`GenerateTerraform`, `Name`) and add a case to `llm.New`. Supported provider keys: `openai`, `claude`, `gemini`, `azure-openai`.
- New CLI subcommands: add a new file in `cmd/` and register via `rootCmd.AddCommand` inside `init()`.

### TUI (BubbleTea)
- The `Model` struct is the single source of truth for UI state; do not use global variables for state.
- Async work (network calls, subprocess execution) must be performed in `tea.Cmd` functions (see `internal/tui/commands.go`), never inside `Update`.
- Style constants belong in `internal/tui/styles.go`; do not embed raw `lipgloss` calls in `model.go`.

### Configuration
- All configuration is loaded from environment variables or a `.env` file via `config.Load()`.
- Always call `cfg.Validate()` after loading when an LLM API key is required.
- Do not hardcode provider defaults or API endpoints outside `config/config.go`.

---

## Development Commands

```bash
# Build
go build -o terraclaw .

# Run in-place (requires Steampipe running)
go run . 

# Run doctor checks
go run . doctor

# Run all tests
go test ./...

# Lint (requires golangci-lint)
golangci-lint run ./...

# Format code
gofmt -w .

# Tidy module dependencies
go mod tidy

# View dependency graph
go mod graph
```

---

## Security Concerns and Guardrails

### API Keys
- **Never hardcode** API keys (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `AZURE_OPENAI_API_KEY`) in source code or test fixtures.
- Keys are loaded exclusively via environment variables or `.env` files — the `.gitignore` already excludes `.env`. Ensure it stays excluded.
- Azure AI Foundry also requires `AZURE_OPENAI_ENDPOINT` (resource URL) and `AZURE_OPENAI_DEPLOYMENT` (deployment name). Never hardcode these.
- When writing tests that require a key value, use a clearly fake sentinel like `"sk-test"` or `"key"`.

### Subprocess Execution
- `terraform import` is the only subprocess spawned by the application (`internal/terraform/importer.go`). The binary path comes from `cfg.TerraformBin` (user-controlled via env/flag) — validate or sanitize this path before use; do not interpolate user input directly into shell strings.
- Do **not** use `os/exec` with `sh -c` or `bash -c`; always pass arguments as a slice to avoid shell injection.

### File System
- Generated `.tf` files are written to `cfg.OutputDir`. The directory is created with permissions `0o750` and files with `0o600` — maintain these restrictive permissions. Do not loosen them.
- Do not write files outside the configured `OutputDir` without explicit user confirmation.

### Database (Steampipe)
- Steampipe queries are constructed from user-selected schema/table names fetched from the database itself. Always parameterize or allowlist table/schema identifiers; never interpolate user-provided strings directly into SQL query strings.
- The Steampipe connection uses `sslmode=disable` (local loopback only). If remote Steampipe support is ever added, enforce TLS.

### LLM Output
- Treat all LLM-generated HCL as **untrusted input**. Display it for user review before writing to disk or running `terraform import`. Do not auto-execute LLM output without explicit user confirmation (the existing confirm step satisfies this — do not bypass it).
- Do not log or print raw LLM responses that might echo back secrets from resource properties.

# terraclaw

Go-based interactive CLI for converting existing cloud resources to Terraform/OpenTofu configuration using AI.

## Overview

`terraclaw` is a [BubbleTea](https://github.com/charmbracelet/bubbletea)-powered terminal UI tool that:

1. **Discovers** your existing cloud resources via [Steampipe](https://steampipe.io/)
2. **Lets you select** the resources you want to import interactively
3. **Generates Terraform HCL** using your preferred LLM (ChatGPT, Claude, or Gemini)
4. **Runs `terraform import`** to create state files for the selected resources

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Steampipe](https://steampipe.io/downloads) with at least one cloud plugin installed
- [Terraform](https://developer.hashicorp.com/terraform/downloads) (for the import step)
- An API key for at least one of: OpenAI, Anthropic, or Google Gemini

## Installation

```bash
git clone https://github.com/arunim2405/terraclaw.git
cd terraclaw
go install .
```

Or build directly:

```bash
go build -o terraclaw .
```

## Configuration

`terraclaw` reads configuration from environment variables or a `.env` file in the working directory.

| Variable             | Default       | Description                                      |
|----------------------|---------------|--------------------------------------------------|
| `STEAMPIPE_HOST`     | `localhost`   | Steampipe PostgreSQL host                        |
| `STEAMPIPE_PORT`     | `9193`        | Steampipe PostgreSQL port                        |
| `STEAMPIPE_DB`       | `steampipe`   | Steampipe database name                          |
| `STEAMPIPE_USER`     | `steampipe`   | Steampipe database user                          |
| `STEAMPIPE_PASSWORD` | _(empty)_     | Steampipe database password                      |
| `LLM_PROVIDER`       | `openai`      | LLM provider: `openai`, `claude`, or `gemini`    |
| `OPENAI_API_KEY`     | _(required for openai)_  | OpenAI API key                      |
| `ANTHROPIC_API_KEY`  | _(required for claude)_  | Anthropic API key                   |
| `GEMINI_API_KEY`     | _(required for gemini)_  | Google Gemini API key               |
| `TERRAFORM_BIN`      | `terraform`   | Path to the `terraform` binary                   |
| `OUTPUT_DIR`         | `.`           | Directory to write generated `.tf` files         |

Example `.env` file:

```env
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
```

## Usage

1. Start Steampipe with your desired cloud plugin:

```bash
steampipe plugin install aws
steampipe service start
```

2. Run `terraclaw`:

```bash
terraclaw
```

To verify local dependencies and configuration before running the TUI:

```bash
terraclaw doctor
```

3. Follow the interactive prompts:
   - Select your LLM provider
   - Select a cloud provider (Steampipe schema)
   - Select a resource type (table)
   - Toggle individual resources with **Space**, confirm with **Enter**
   - Review the generated Terraform code
   - Confirm running `terraform import`

### Flags

```
--output-dir string    Directory to write generated Terraform files (default ".")
--terraform-bin string Path to the terraform binary (default "terraform")
```

### Doctor checks

`terraclaw doctor` validates:
- `steampipe` and `terraform` binaries are available on `PATH` (or via `--terraform-bin`)
- output directory exists and is writable
- selected `LLM_PROVIDER` has the required API key
- Steampipe is reachable and has at least one plugin schema installed

## Project Structure

```
terraclaw/
├── main.go                       Entry point
├── cmd/root.go                   Cobra CLI setup
├── config/config.go              Configuration loading
├── internal/
│   ├── steampipe/client.go       Steampipe PostgreSQL client
│   ├── llm/
│   │   ├── provider.go           LLM provider interface & prompt builder
│   │   ├── openai.go             OpenAI (ChatGPT) implementation
│   │   ├── claude.go             Anthropic Claude implementation
│   │   └── gemini.go             Google Gemini implementation
│   ├── terraform/
│   │   ├── generator.go          HCL file writer & address helpers
│   │   └── importer.go           terraform import runner
│   └── tui/
│       ├── model.go              BubbleTea model & views
│       ├── commands.go           Async tea.Cmd definitions
│       └── styles.go             Lipgloss styles
```

## Keyboard Shortcuts

| Key           | Action                        |
|---------------|-------------------------------|
| `Enter`       | Select / confirm              |
| `Space`       | Toggle resource selection     |
| `↑` / `↓`     | Navigate list / scroll code   |
| `Esc`         | Go back to previous step      |
| `/`           | Filter list                   |
| `q` / `Ctrl+C`| Quit                         |

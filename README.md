# terraclaw

Go-based interactive CLI for converting existing cloud resources to Terraform/OpenTofu configuration using AI.

## Overview

`terraclaw` is a [BubbleTea](https://github.com/charmbracelet/bubbletea)-powered terminal UI tool that:

1. **Discovers** your existing cloud resources via [Steampipe](https://steampipe.io/)
2. **Lets you select** the resources you want to import interactively
3. **Generates Terraform HCL** using [OpenCode](https://opencode.ai/) (AI coding agent) in a two-stage pipeline (blueprint then code)
4. **Runs `terraform import`** to create state files for the selected resources

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Steampipe](https://steampipe.io/downloads) with at least one cloud plugin installed
- [Terraform](https://developer.hashicorp.com/terraform/downloads) (for the import step)
- [OpenCode](https://opencode.ai/) (`brew install opencode` or `npm install -g opencode-ai`)
- An AI provider configured in OpenCode (e.g. GitHub Copilot, OpenAI) via `opencode providers login`

## Installation

```bash
git clone https://github.com/arunim2405/terraclaw.git
cd terraclaw
make build
```

Or install directly:

```bash
go install .
```

## Configuration

`terraclaw` reads configuration from environment variables or a `.env` file in the working directory. The AI provider is configured through OpenCode's own auth system, not via terraclaw env vars.

| Variable             | Default       | Description                                      |
|----------------------|---------------|--------------------------------------------------|
| `STEAMPIPE_HOST`     | `localhost`   | Steampipe PostgreSQL host                        |
| `STEAMPIPE_PORT`     | `9193`        | Steampipe PostgreSQL port                        |
| `STEAMPIPE_DB`       | `steampipe`   | Steampipe database name                          |
| `STEAMPIPE_USER`     | `steampipe`   | Steampipe database user                          |
| `STEAMPIPE_PASSWORD` | _(empty)_     | Steampipe database password                      |
| `OPENCODE_PORT`      | `4096`        | Port for the OpenCode headless server            |
| `TERRAFORM_BIN`      | `terraform`   | Path to the `terraform` binary                   |
| `OUTPUT_DIR`         | `./output`    | Directory to write generated `.tf` files         |
| `CACHE_DIR`          | `~/.cache/terraclaw` | Directory for SQLite scan cache           |
| `CACHE_TTL`          | `1h`          | How long cached scans remain valid               |
| `NO_CACHE`           | `false`       | Skip the resource cache entirely                 |
| `DEBUG`              | `false`       | Enable debug logging to file                     |
| `DEBUG_LOG_FILE`     | `terraclaw.log` | Path for debug log output                      |

## OpenCode Setup

terraclaw delegates all LLM interaction to [OpenCode](https://opencode.ai/), which manages its own provider authentication. To get started:

```bash
# Log in to a provider (e.g. GitHub Copilot, OpenAI)
opencode providers login

# Verify your credentials
opencode providers list
```

The project includes an `opencode.json` that configures the model, permissions, and Terraform skills for headless operation. OpenCode will automatically pick it up from the project directory.

## Usage

### Interactive mode (TUI)

1. Start Steampipe with your desired cloud plugin:

```bash
steampipe plugin install aws
steampipe service start
```

2. Run `terraclaw`:

```bash
terraclaw
```

3. Follow the interactive prompts:
   - Select a cloud provider (Steampipe schema)
   - Select a resource type (table)
   - Toggle individual resources with **Space**, confirm with **Enter**
   - Review the generated Terraform code
   - Confirm running `terraform import`

### Non-interactive mode

Generate Terraform directly by specifying resource ARNs:

```bash
terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws
terraclaw generate --resources arn:aws:s3:::bucket-1,arn:aws:lambda:us-east-1:123456:function:my-func
```

### Doctor checks

Validate local dependencies and configuration:

```bash
terraclaw doctor
```

This checks that `steampipe`, `terraform`, and `opencode` are available on `PATH`, the output directory is writable, and Steampipe is reachable.

### Flags

```
--output-dir string      Directory to write generated Terraform files (default "./output")
--terraform-bin string   Path to the terraform binary (default "terraform")
--debug                  Enable debug logging to file (see DEBUG_LOG_FILE)
--no-cache               Skip the resource cache and always rescan from Steampipe
```

## Docker

A Docker image bundles all dependencies (Steampipe, Terraform, OpenCode, AWS CLI) for self-contained runs.

### Build

```bash
make docker-build
```

### Run

1. Create your env file from the sample:

```bash
make env-sample
# Edit .docker.env with your credentials
```

2. Run the container:

```bash
make docker-run
```

Or extract output artifacts to a local directory:

```bash
make docker-run-artifacts
```

The `.sample.docker.env` documents all required and optional environment variables for Docker runs. Key variables:

| Variable             | Required | Description                                      |
|----------------------|----------|--------------------------------------------------|
| `TERRACLAW_CMD_B64`  | Yes      | Base64-encoded terraclaw CLI command              |
| `OPENCLAW_CREDS_B64` | Yes      | Base64-encoded OpenCode `auth.json` content       |
| `AWS_ACCESS_KEY_ID`  | No       | AWS credentials (auto-detected by Steampipe)      |
| `AWS_SECRET_ACCESS_KEY` | No    | AWS secret key                                    |
| `AWS_REGION`         | No       | AWS region                                        |
| `LOCAL_ARTIFACTS_DIR`| No       | Bind-mount path to copy the output zip into       |
| `STEAMPIPE_PLUGINS`  | No       | Comma-separated extra plugins (e.g. `azure,gcp`)  |

### Generating `OPENCLAW_CREDS_B64`

This variable contains your OpenCode auth credentials (provider tokens) base64-encoded so they can be injected into the container at runtime. The credentials live in OpenCode's local auth file on your machine.

1. First, make sure you have at least one provider logged in locally:

```bash
opencode providers login   # follow the OAuth flow for GitHub Copilot, OpenAI, etc.
opencode providers list    # verify credentials are saved
```

2. Base64-encode the auth file:

```bash
# macOS
cat ~/.local/share/opencode/auth.json | base64

# Linux
cat ~/.local/share/opencode/auth.json | base64 -w 0
```

3. Paste the output into your `.docker.env`:

```env
OPENCLAW_CREDS_B64=eyJnaXRodWItY29waWxvdCI6ey...
```

Similarly, for `TERRACLAW_CMD_B64`:

```bash
echo "terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws" | base64
```

### What happens inside the container

The entrypoint (`docker/entrypoint.sh`) runs these steps in order:

1. Writes OpenCode auth credentials from `OPENCLAW_CREDS_B64`
2. Installs Steampipe plugins and starts the Steampipe service
3. Starts `opencode serve` in the background
4. Runs the decoded terraclaw command
5. Zips the output and optionally copies it to `LOCAL_ARTIFACTS_DIR`
6. Cleans up OpenCode and Steampipe processes

## Makefile

Run `make help` to see all available targets:

```
build                  Build the binary for the current platform
build-linux            Cross-compile for linux/amd64
run                    Build and run with sample args (override with ARGS=)
test                   Run all tests
lint                   Run go vet
clean                  Remove build artifacts
docker-build           Build the Docker image
docker-run             Run the Docker container with .docker.env
docker-run-artifacts   Run and extract output to ./artifacts
docker-shell           Open a shell in the Docker container
env-sample             Copy sample env to .docker.env
help                   Show this help
```

## Project Structure

```
terraclaw/
├── main.go                         Entry point
├── cmd/
│   ├── root.go                     Cobra CLI setup & TUI entry
│   ├── generate.go                 Non-interactive generate command
│   ├── doctor.go                   Dependency checker command
│   └── debug.go                    Debug utilities
├── config/config.go                Configuration loading
├── opencode.json                   OpenCode project config (model, permissions, plugins)
├── internal/
│   ├── opencode/opencode.go        OpenCode server lifecycle & REST client
│   ├── llm/provider.go             Two-stage Terraform generation pipeline
│   ├── steampipe/client.go         Steampipe PostgreSQL client
│   ├── terraform/
│   │   ├── generator.go            HCL file helpers
│   │   └── importer.go             terraform import runner
│   ├── tui/
│   │   ├── model.go                BubbleTea model & views
│   │   ├── commands.go             Async tea.Cmd definitions
│   │   └── styles.go               Lipgloss styles
│   ├── cache/                      SQLite scan cache
│   ├── debuglog/                   File-based debug logger
│   ├── doctor/                     Dependency validation
│   └── graph/                      Resource dependency graph
├── .agents/skills/                 OpenCode Terraform skills (style guide, import, etc.)
├── docker/
│   └── entrypoint.sh               Docker container entrypoint
├── Dockerfile                       Multi-stage Docker build
├── Makefile                         Build & run targets
└── .sample.docker.env               Sample env file for Docker runs
```

## Keyboard Shortcuts (TUI)

| Key           | Action                        |
|---------------|-------------------------------|
| `Enter`       | Select / confirm              |
| `Space`       | Toggle resource selection     |
| `↑` / `↓`     | Navigate list / scroll code   |
| `Esc`         | Go back to previous step      |
| `/`           | Filter list                   |
| `q` / `Ctrl+C`| Quit                         |

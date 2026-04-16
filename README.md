# terraclaw

Go-based interactive CLI for converting existing cloud resources to Terraform/OpenTofu configuration using AI.

## Overview

`terraclaw` is a [BubbleTea](https://github.com/charmbracelet/bubbletea)-powered terminal UI tool that:

1. **Discovers** your existing cloud resources via [Steampipe](https://steampipe.io/) (AWS and Azure)
2. **Lets you select** the resources you want to import interactively
3. **Matches your own Terraform modules** from git repos or local paths, scoring them by fit
4. **Generates Terraform HCL** using [OpenCode](https://opencode.ai/) (AI coding agent), preferring your modules over public registry modules
5. **Runs `terraform import`** to create state files for the selected resources

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

Generate Terraform directly by specifying resource ARNs or Azure resource IDs:

```bash
# AWS
terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws

# Azure
terraclaw generate --resources /subscriptions/xxx/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1 --schema azure

# With user modules (auto-select all matching modules)
terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws --auto-modules
```

### User Modules

Register your organization's Terraform modules so terraclaw uses them instead of public registry modules during code generation.

#### Adding a module

```bash
# From a git repository (Terraform source format)
terraclaw add-module "git::https://github.com/acme/tf-modules.git//modules/vpc?ref=v2.0"

# From a git repo via SSH
terraclaw add-module "git@github.com:acme/tf-modules.git//modules/vpc?ref=v2.0"

# From a local directory
terraclaw add-module ./my-modules/vpc

# With a custom name (overrides auto-derived name)
terraclaw add-module --name "custom-vpc" "git::https://github.com/acme/tf-modules.git//modules/vpc?ref=v2.0"
```

When you add a module, terraclaw automatically:
- Clones the repository (for git sources) or reads the local directory
- Parses all `.tf` files to extract resource types, variables, and outputs
- Detects the cloud provider (AWS, Azure) from resource type prefixes
- Reads the README for a description
- Stores the metadata in a local SQLite database (`~/.cache/terraclaw/modules.db`)

#### Managing modules

```bash
# List all registered modules
terraclaw list-modules

# Show full metadata (variables, outputs, resource types)
terraclaw inspect-module vpc

# Remove a module
terraclaw remove-module vpc
```

#### How modules are used during generation

**Interactive mode (TUI):** After you select resources and confirm generation, terraclaw checks your registered modules for matches. If any modules manage the same resource types you selected, a module selection screen appears showing each module's **fit score** (0-100%). Modules scoring 60%+ are pre-selected. Toggle with **Space**, confirm with **Enter**.

**Non-interactive mode:** Use `--use-modules` or `--auto-modules`:

```bash
# Auto-select all matching modules
terraclaw generate --resources arn:aws:... --schema aws --auto-modules

# Use modules (pre-selects those with >= 60% fit)
terraclaw generate --resources arn:aws:... --schema aws --use-modules
```

#### Fit score

The fit score tells you how well a module matches your selected resources:

| Component | Weight | What it measures |
|-----------|--------|------------------|
| Coverage | 50% | Fraction of your selected resource types the module handles |
| Specificity | 30% | How focused the module is (penalizes modules that drag in many unrelated resources) |
| Variable Readiness | 20% | Fraction of required variables that have defaults or match resource properties |

Selected modules are injected into the AI prompt as hard constraints, taking priority over public registry modules (e.g. `terraform-aws-modules`).

### Doctor checks

Validate local dependencies and configuration:

```bash
terraclaw doctor
```

This checks that `steampipe`, `terraform`, and `opencode` are available on `PATH`, the output directory is writable, and Steampipe is reachable.

### Flags

**Global flags:**
```
--output-dir string      Directory to write generated Terraform files (default "./output")
--terraform-bin string   Path to the terraform binary (default "terraform")
--debug                  Enable debug logging to file (see DEBUG_LOG_FILE)
--no-cache               Skip the resource cache and always rescan from Steampipe
```

**Generate flags:**
```
-r, --resources string   Comma-separated resource ARNs or Azure resource IDs (required)
    --schema string      Steampipe schema (auto-detected from resource IDs if omitted)
    --use-modules        Use registered user modules (matched by resource type)
    --auto-modules       Auto-select all matching user modules (implies --use-modules)
```

**Add-module flags:**
```
    --name string        Override the auto-derived module name
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
│   ├── module.go                   Module management commands (add, list, remove, inspect)
│   ├── doctor.go                   Dependency checker command
│   └── debug.go                    Debug utilities
├── config/config.go                Configuration loading
├── opencode.json                   OpenCode project config (model, permissions, plugins)
├── internal/
│   ├── opencode/opencode.go        OpenCode server lifecycle & REST client
│   ├── llm/
│   │   ├── provider.go             Two-stage Terraform generation pipeline
│   │   ├── prompts_aws.go          AWS-specific prompt content
│   │   └── prompts_azure.go        Azure-specific prompt content
│   ├── modules/
│   │   ├── types.go                ModuleMetadata, FitResult types
│   │   ├── store.go                SQLite CRUD for module metadata
│   │   ├── scanner.go              Git clone + HCL parsing (terraform-config-inspect)
│   │   ├── matcher.go              Fit score algorithm (coverage, specificity, var readiness)
│   │   └── prompt.go               User module prompt section builder
│   ├── provider/provider.go        Cloud provider detection (AWS, Azure)
│   ├── steampipe/
│   │   ├── client.go               Steampipe PostgreSQL client
│   │   └── resource_mapping.go     AWS ARN + Azure resource ID → table mapping
│   ├── terraform/
│   │   ├── generator.go            HCL file helpers
│   │   └── importer.go             terraform import runner
│   ├── tui/
│   │   ├── model.go                BubbleTea model & views (incl. module selection step)
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

| Key           | Action                                      |
|---------------|---------------------------------------------|
| `Enter`       | Select / confirm                            |
| `Space`       | Toggle resource or module selection         |
| `r`           | Expand related resources (resource step)    |
| `↑` / `↓`     | Navigate list / scroll code                 |
| `Esc`         | Go back to previous step                    |
| `/`           | Filter list                                 |
| `q` / `Ctrl+C`| Quit                                       |

---

## Codify at Scale

<a href="https://www.stackguardian.io/platform/code">
  <img src="https://cdn.prod.website-files.com/624e5850e5f4590e498e0ce2/67bcb8cf3697a7ff4be3e14d_SG%20logo%20Dark-p-500.png" alt="StackGuardian" width="200" />
</a>

If you're looking to codify cloud resources **at scale** with minimal setup — and manage, govern, and drift-detect the resulting Terraform across your organization — check out [StackGuardian Code](https://www.stackguardian.io/platform/code).

StackGuardian provides a managed platform that goes beyond one-off imports: continuous codification, policy guardrails, module governance, and team collaboration out of the box.

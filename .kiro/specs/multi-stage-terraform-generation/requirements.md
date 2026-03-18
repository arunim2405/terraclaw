# Requirements Document

## Introduction

Refactor terraclaw's Terraform code generation from a single monolithic OpenCode prompt into a two-stage pipeline within a single OpenCode session. Stage 1 produces a structured YAML blueprint (resource grouping, modules, variables, outputs, import IDs). Stage 2 consumes that blueprint and generates production-ready Terraform HCL files plus an import script. Both stages execute as sequential prompts within one session, preserving conversational context. This separation improves reliability, debuggability, and output quality by giving each stage a focused prompt while maintaining shared context.

## Glossary

- **Pipeline**: The two-stage sequence of prompts within a single OpenCode session that replaces the current single-shot generation flow.
- **Blueprint**: A structured YAML document produced by the Stage 1 prompt containing module definitions, variables, outputs, root wiring, and import IDs for all selected resources.
- **Session**: A single OpenCode session created via `CreateSession`, within which both Stage 1 and Stage 2 prompts are sent sequentially. The session maintains conversational context across prompts.
- **Stage_1_Prompt**: The first user prompt sent within the Session, containing the Resource_JSON. The agent responds with the Blueprint as text.
- **Stage_2_Prompt**: The second user prompt sent within the same Session, containing the persisted Blueprint and instructions to generate Terraform HCL files.
- **OpenCode_Server**: The background OpenCode process that exposes a REST API for session management and prompting (managed by `internal/opencode/opencode.go`).
- **LLM_Provider**: The `internal/llm/provider.go` module that orchestrates the OpenCode session and prompt construction.
- **TUI**: The BubbleTea terminal UI in `internal/tui/` that drives the interactive wizard flow.
- **Import_Script**: A bash script (`import.sh`) containing `terraform init` and `terraform import` commands for all generated resources.
- **System_Prompt**: The instruction text injected into the Session via `InjectSystemPrompt` before the Stage_1_Prompt, defining the agent's role and output format for blueprint generation.
- **Resource_JSON**: The JSON representation of AWS resources discovered by Steampipe, used as input to the Stage_1_Prompt.

## Requirements

### Requirement 1: Single-Session Two-Stage Pipeline Orchestration

**User Story:** As a developer, I want the Terraform generation to run as a two-stage pipeline within a single OpenCode session, so that resource analysis and code generation share conversational context while being handled by separate, focused prompts.

#### Acceptance Criteria

1. WHEN the user confirms code generation, THE Pipeline SHALL create a single Session via `CreateSession("terraclaw-terraform-generation")`.
2. WHEN the Session is created, THE Pipeline SHALL inject the Stage 1 System_Prompt via `InjectSystemPrompt` to set the blueprint generator identity and output format.
3. WHEN the System_Prompt is injected, THE Pipeline SHALL send the Stage_1_Prompt containing the Resource_JSON via `Prompt` or `PromptAsync` and wait for the Blueprint response.
4. WHEN the Stage_1_Prompt returns a valid Blueprint response, THE Pipeline SHALL extract the Blueprint text from the response, persist it to disk, and then send the Stage_2_Prompt containing the Blueprint and Terraform generation instructions within the same Session.
5. IF the Stage_1_Prompt returns an error or empty response, THEN THE Pipeline SHALL report the error to the TUI and halt generation without sending the Stage_2_Prompt.
6. IF the Stage_2_Prompt returns an error or produces zero output files, THEN THE Pipeline SHALL report the error to the TUI.
7. THE Pipeline SHALL reuse the existing OpenCode_Server instance rather than starting a new server process.

### Requirement 2: Stage 1 — YAML Blueprint Generation

**User Story:** As a developer, I want the first stage to analyze my selected AWS resources and produce a structured YAML blueprint, so that the code generation stage has a clear, deterministic specification to work from.

#### Acceptance Criteria

1. THE Session SHALL receive a dedicated System_Prompt via `InjectSystemPrompt` that instructs the agent to analyze Resource_JSON and produce a YAML Blueprint.
2. THE System_Prompt SHALL include rules for grouping related resources into logical modules (e.g., aws_iam_role + aws_iam_role_policy + aws_iam_role_policy_attachment into a single IAM role module).
3. THE System_Prompt SHALL instruct the agent to use for_each at root level only when two or more instances of the same logical module exist.
4. THE System_Prompt SHALL instruct the agent to preserve all import IDs from the Resource_JSON in the Blueprint output.
5. THE System_Prompt SHALL include prompt injection defense instructions.
6. THE System_Prompt SHALL include rules for variable naming, cross-module wiring, and security guardrails (credential redaction, zero-drift integrity).
7. WHEN the Resource_JSON is missing information needed for the Blueprint, THE System_Prompt SHALL instruct the agent to fetch missing data via AWS CLI commands.
8. THE LLM_Provider SHALL expose a dedicated function to build the Stage 1 System_Prompt, separate from the Stage 2 prompt builder.

### Requirement 3: Stage 2 — Terraform Code Generation from Blueprint

**User Story:** As a developer, I want the second stage to consume the YAML blueprint and generate production-ready Terraform files, so that I get modular, importable infrastructure code.

#### Acceptance Criteria

1. THE Stage_2_Prompt SHALL contain the persisted Blueprint content along with instructions for the agent to generate Terraform HCL files.
2. THE Stage_2_Prompt SHALL instruct the agent to generate module directories (modules/*/main.tf, variables.tf, outputs.tf), a root main.tf, terraform.tfvars, versions.tf, and providers.tf.
3. THE Stage_2_Prompt SHALL instruct the agent to generate an Import_Script (import.sh) containing static `terraform import` commands for all resources.
4. THE Stage_2_Prompt SHALL include rules for cross-module references, root wiring, and for_each usage matching the Blueprint specification.
5. WHEN the Blueprint references information the agent cannot resolve from context alone, THE Stage_2_Prompt SHALL instruct the agent to fetch missing data via AWS CLI commands.
6. THE LLM_Provider SHALL expose a dedicated function to build the Stage_2_Prompt, separate from the Stage 1 System_Prompt builder.

### Requirement 4: TUI Progress Reporting for Two Stages

**User Story:** As a user, I want to see which stage of the pipeline is currently running, so that I understand the progress of code generation.

#### Acceptance Criteria

1. WHILE the Stage_1_Prompt is awaiting a response, THE TUI SHALL display a status indicator showing "Stage 1: Generating Blueprint..." along with agent activity from polling.
2. WHILE the Stage_2_Prompt is awaiting a response, THE TUI SHALL display a status indicator showing "Stage 2: Generating Terraform Code..." along with agent activity from polling.
3. WHEN the Stage_1_Prompt completes successfully, THE TUI SHALL transition the status display from Stage 1 to Stage 2 without user intervention.

### Requirement 5: Blueprint Persistence

**User Story:** As a developer, I want the YAML blueprint to be saved to disk, so that I can inspect it, debug issues, or re-run Stage 2 independently.

#### Acceptance Criteria

1. WHEN the Stage_1_Prompt completes successfully, THE Pipeline SHALL extract the Blueprint from the response text and write it to a file named `blueprint.yaml` in the configured output directory.
2. THE Pipeline SHALL write the Blueprint file with permissions 0o600.
3. WHEN the Stage_2_Prompt is constructed, THE Pipeline SHALL read the Blueprint from the persisted file rather than passing it only in memory, ensuring the file on disk is the source of truth.

### Requirement 6: Import Script Execution

**User Story:** As a developer, I want the generated import.sh script to be automatically executed after Terraform file generation, so that my state is initialized without manual steps.

#### Acceptance Criteria

1. WHEN the Stage_2_Prompt completes and the user confirms import, THE TUI SHALL execute the Import_Script using the existing `RunImportScript` function.
2. THE Import_Script SHALL contain `terraform init` followed by `terraform import` commands for each resource defined in the Blueprint.
3. IF the Import_Script does not exist after the Stage_2_Prompt completes, THEN THE TUI SHALL fall back to per-resource import using `GuessResourceAddress`.

### Requirement 7: Non-Interactive (CLI) Pipeline Support

**User Story:** As a developer, I want the `terraclaw generate` CLI command to also use the two-stage pipeline, so that non-interactive usage produces the same quality output as the TUI.

#### Acceptance Criteria

1. WHEN the `terraclaw generate` command is invoked with `--resources` flag, THE Pipeline SHALL execute both Stage_1_Prompt and Stage_2_Prompt sequentially within a single Session.
2. THE `terraclaw generate` command SHALL print progress messages indicating which stage is currently running.
3. WHEN both stages complete, THE `terraclaw generate` command SHALL list all generated files and the Blueprint file path.

### Requirement 8: Backward-Compatible File Scanning

**User Story:** As a developer, I want the file scanning after generation to support the new modular directory structure, so that all generated files (including those in subdirectories) are discovered.

#### Acceptance Criteria

1. THE LLM_Provider SHALL scan for generated files recursively, including files in module subdirectories (e.g., modules/iam_role/main.tf).
2. THE LLM_Provider SHALL scan for .tf files, .sh files (import.sh), and .yaml files (blueprint.yaml) in the output directory tree.
3. THE TUI file viewer SHALL display files from subdirectories with their relative path (e.g., "modules/vpc/main.tf") rather than just the basename.

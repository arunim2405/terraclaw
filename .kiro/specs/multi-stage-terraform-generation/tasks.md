# Implementation Plan: Multi-Stage Terraform Generation

## Overview

Refactor terraclaw's single-shot OpenCode prompt into a two-stage pipeline within a single OpenCode session. Stage 1 generates a YAML blueprint; Stage 2 consumes it to produce Terraform HCL files. Implementation proceeds bottom-up: pure functions first, then orchestration, then TUI/CLI integration.

## Tasks

- [x] 1. Implement Stage 1 and Stage 2 prompt builders in `internal/llm/provider.go`
  - [x] 1.1 Add `BuildStage1SystemPrompt()` function
    - Create a new exported function returning the Stage 1 system prompt string constant
    - The prompt must include: blueprint generator identity, XML-tagged structure, YAML output format between `<<YAML>>` / `<<END_YAML>>` markers, module grouping rules, for_each rules, import ID preservation, variable naming, cross-module wiring, security guardrails (credential redaction, zero-drift), prompt injection defense, and AWS CLI fallback instructions
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_

  - [x] 1.2 Add `BuildStage1UserPrompt(resources []steampipe.Resource) string` function
    - Serialize resources to JSON (replacing the current plain-text listing)
    - Include all resource fields: Provider, Type, Name, ID, Region, Properties
    - Wrap with instructions to analyze and produce the YAML blueprint
    - _Requirements: 1.3, 2.4_

  - [x] 1.3 Add `BuildStage2Prompt(blueprint string, outputDir string) string` function
    - Embed the full blueprint content and the output directory path in the prompt
    - Include instructions for: module directory structure (`modules/*/main.tf`, `variables.tf`, `outputs.tf`), root `main.tf`, `terraform.tfvars`, `versions.tf`, `providers.tf`, `import.sh` generation, cross-module references, for_each usage, and AWS CLI fallback
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

  - [x] 1.4 Write property test: Stage 1 user prompt contains all resource identifiers
    - **Property 6: Stage 1 user prompt contains all resource identifiers**
    - Generate random `[]steampipe.Resource` values with gopter; verify every resource's `ID` and `Type` appear as substrings in `BuildStage1UserPrompt` output
    - **Validates: Requirements 1.3, 2.4**

  - [x] 1.5 Write property test: Stage 2 prompt embeds blueprint content
    - **Property 3: Stage 2 prompt embeds blueprint content**
    - Generate random non-empty blueprint strings and output directory paths with gopter; verify both appear as substrings in `BuildStage2Prompt` output
    - **Validates: Requirements 3.1**

  - [x] 1.6 Write unit tests for prompt builder key instructions
    - `TestBuildStage1SystemPrompt_ContainsKeyInstructions`: verify prompt contains module grouping, for_each, import ID, prompt injection defense, and AWS CLI fallback instructions
    - `TestBuildStage2Prompt_ContainsKeyInstructions`: verify prompt contains module directory, import.sh, cross-module reference, and AWS CLI instructions
    - _Requirements: 2.2–2.7, 3.2–3.5_

- [x] 2. Implement blueprint extraction and persistence in `internal/llm/provider.go`
  - [x] 2.1 Add `ExtractBlueprint(response string) (string, error)` function
    - Parse the response text to find content between `<<YAML>>` and `<<END_YAML>>` markers
    - Return error if markers are not found or content is empty
    - _Requirements: 5.1, 1.4_

  - [x] 2.2 Add `PersistBlueprint(blueprint string, outputDir string) error` function
    - Write `blueprint.yaml` to `outputDir` with `0o600` permissions
    - _Requirements: 5.1, 5.2_

  - [x] 2.3 Add `ReadBlueprint(outputDir string) (string, error)` function
    - Read `blueprint.yaml` from `outputDir`
    - _Requirements: 5.3_

  - [x] 2.4 Write property test: Blueprint extraction round trip
    - **Property 1: Blueprint extraction round trip**
    - Generate random YAML strings (excluding marker sequences) with gopter; wrap in `<<YAML>>\n` ... `\n<<END_YAML>>` with optional surrounding prose; call `ExtractBlueprint` and verify exact match
    - **Validates: Requirements 5.1**

  - [x] 2.5 Write property test: Blueprint persistence round trip
    - **Property 7: Blueprint persistence round trip**
    - Generate random YAML strings with gopter; call `PersistBlueprint` then `ReadBlueprint` in a temp directory; verify exact match
    - **Validates: Requirements 5.1, 5.3**

  - [x] 2.6 Write unit tests for extraction edge cases
    - `TestExtractBlueprint_NoMarkers`: verify error when markers are missing
    - `TestExtractBlueprint_EmptyYAML`: verify behavior when markers exist but content is empty
    - `TestPersistBlueprint_Permissions`: verify file is written with `0o600` permissions
    - _Requirements: 1.5, 5.2_

- [x] 3. Implement recursive file scanning in `internal/llm/provider.go`
  - [x] 3.1 Add `RecursiveListGeneratedFiles(dir string) ([]GeneratedFile, error)` function
    - Walk `dir` recursively using `filepath.WalkDir`
    - Include files with extensions `.tf`, `.sh`, `.yaml`
    - Set `Name` field to the relative path from `dir` (e.g., `modules/vpc/main.tf`)
    - Set `Path` field to the absolute path
    - Read file content into `Content`
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 3.2 Update `GeneratedFile.Name` field semantics
    - Change the `Name` field comment from "basename" to "relative path from outputDir"
    - Ensure all consumers of `GeneratedFile.Name` handle relative paths (TUI file viewer tabs)
    - _Requirements: 8.3_

  - [x] 3.3 Write property test: Recursive file scanning discovers all matching files
    - **Property 4: Recursive file scanning discovers all matching files**
    - Generate random directory trees with gopter containing `.tf`, `.sh`, `.yaml`, and non-matching files at arbitrary depths; verify `RecursiveListGeneratedFiles` returns exactly the matching set
    - **Validates: Requirements 8.1, 8.2**

  - [x] 3.4 Write property test: Scanned file names use relative paths
    - **Property 5: Scanned file names use relative paths**
    - Generate files in random subdirectory structures with gopter; verify each `GeneratedFile.Name` equals the relative path from the scanned root
    - **Validates: Requirements 8.3**

  - [x] 3.5 Write unit tests for scanner edge cases
    - `TestRecursiveListGeneratedFiles_IgnoresNonMatchingExtensions`: verify `.txt`, `.json`, etc. are excluded
    - Place these tests in `internal/llm/scanner_test.go`
    - _Requirements: 8.1, 8.2_

- [x] 4. Checkpoint — Ensure all tests pass
  - Ensure all tests pass with `go test ./internal/llm/...`, ask the user if questions arise.

- [x] 5. Refactor `GenerateTerraform` for two-stage pipeline in `internal/llm/provider.go`
  - [x] 5.1 Refactor `OpencodeProvider.GenerateTerraform()` method
    - Replace the single-prompt flow with the two-stage pipeline:
      1. `CreateSession("terraclaw-terraform-generation")`
      2. `InjectSystemPrompt(sessionID, BuildStage1SystemPrompt())`
      3. `Prompt(sessionID, BuildStage1UserPrompt(resources))`
      4. `ExtractBlueprint(response)` → `PersistBlueprint(blueprint, outputDir)`
      5. `ReadBlueprint(outputDir)` → `Prompt(sessionID, BuildStage2Prompt(blueprintFromDisk, outputDir))`
      6. `RecursiveListGeneratedFiles(outputDir)`
    - Return error and halt if Stage 1 fails (no Stage 2 call)
    - Return error if Stage 2 produces zero `.tf` files
    - Remove old `BuildSystemPrompt` and `buildPrompt` functions (replaced by stage-specific builders)
    - Remove old `ListGeneratedFiles` function (replaced by `RecursiveListGeneratedFiles`)
    - Remove `BuildUserPrompt` method on `OpencodeProvider` (no longer needed)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7_

  - [x] 5.2 Write property test: Stage 1 error halts pipeline
    - **Property 2: Stage 1 error halts pipeline**
    - Generate random resource lists with gopter; create a mock `opencode.Server` that returns an error on the first `Prompt` call; verify `GenerateTerraform` returns non-nil error and mock records zero subsequent `Prompt` calls
    - **Validates: Requirements 1.5**

  - [x] 5.3 Write unit tests for pipeline happy path and Stage 2 error
    - `TestGenerateTerraform_HappyPath`: mock server returning valid Stage 1 (with YAML markers) and Stage 2 responses; verify full pipeline produces files
    - `TestGenerateTerraform_Stage2Error`: mock server returning success for Stage 1 but error for Stage 2; verify error is returned
    - _Requirements: 1.4, 1.5, 1.6_

- [x] 6. Update TUI for two-stage progress display
  - [x] 6.1 Add `generationStage` field to `Model` in `internal/tui/model.go`
    - Add `generationStage int` field (1 = blueprint, 2 = terraform)
    - Add `stageTransitionMsg` handling in `Update` to set `generationStage`
    - _Requirements: 4.1, 4.2, 4.3_

  - [x] 6.2 Update `generatingView()` in `internal/tui/model.go`
    - When `generationStage == 1`: display "Stage 1: Generating Blueprint..."
    - When `generationStage == 2`: display "Stage 2: Generating Terraform Code..."
    - _Requirements: 4.1, 4.2_

  - [x] 6.3 Refactor `generateCodeCmd` in `internal/tui/commands.go`
    - Add `stageTransitionMsg` message type
    - Refactor to create session, inject Stage 1 system prompt, send Stage 1 prompt async with `generatingStartedMsg` carrying stage=1
    - On Stage 1 completion in `pollAgentStatusCmd`: extract blueprint, persist to disk, send Stage 2 prompt async, emit `stageTransitionMsg{stage: 2}`
    - On Stage 2 completion: call `RecursiveListGeneratedFiles` and emit `generationDoneMsg`
    - Replace `llm.BuildSystemPrompt` with `llm.BuildStage1SystemPrompt`
    - Replace `llm.ListGeneratedFiles` with `llm.RecursiveListGeneratedFiles`
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 4.1, 4.2, 4.3_

  - [x] 6.4 Write unit tests for TUI stage display
    - `TestTUI_GeneratingView_Stage1`: verify view output contains "Stage 1: Generating Blueprint..." when `generationStage == 1`
    - `TestTUI_GeneratingView_Stage2`: verify view output contains "Stage 2: Generating Terraform Code..." when `generationStage == 2`
    - _Requirements: 4.1, 4.2_

- [x] 7. Update CLI `generate` command for two-stage pipeline
  - [x] 7.1 Refactor `runGenerate` in `cmd/generate.go`
    - Replace single-prompt flow with two-stage pipeline:
      1. Print "Stage 1: Generating Blueprint..."
      2. Inject Stage 1 system prompt, send Stage 1 prompt async, poll for status
      3. On Stage 1 completion: extract and persist blueprint, print "Stage 2: Generating Terraform Code..."
      4. Send Stage 2 prompt async, poll for status
      5. On completion: call `RecursiveListGeneratedFiles`, list all files including `blueprint.yaml` path
    - Replace `llm.BuildSystemPrompt` with `llm.BuildStage1SystemPrompt`
    - Replace `llm.ListGeneratedFiles` with `llm.RecursiveListGeneratedFiles`
    - Print stage-specific error prefixes (e.g., `"stage 1 (blueprint) failed: ..."`)
    - _Requirements: 7.1, 7.2, 7.3_

- [x] 8. Update import fallback logic
  - [x] 8.1 Update `runImportCmd` in `internal/tui/commands.go`
    - Ensure `import.sh` detection works with the new output directory structure
    - Verify fallback to `GuessResourceAddress` when `import.sh` is absent
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 8.2 Write unit test for import fallback
    - `TestImportFallback_NoImportScript`: verify fallback to `GuessResourceAddress` when `import.sh` is absent
    - _Requirements: 6.3_

- [x] 9. Final checkpoint — Ensure all tests pass
  - Run `go test ./...` to verify all tests pass across all packages. Ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests use the `gopter` library and validate universal correctness properties from the design
- Unit tests validate specific examples and edge cases
- All new test files: `internal/llm/provider_test.go`, `internal/llm/scanner_test.go`, `internal/tui/model_test.go`, `internal/tui/commands_test.go`

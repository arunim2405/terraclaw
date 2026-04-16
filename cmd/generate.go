// Package cmd provides the CLI commands for terraclaw.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/modules"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/provider"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Terraform code for specified resources (non-interactive)",
	Long: `Generate Terraform code directly by specifying resource ARNs or Azure resource IDs.
This is the non-interactive equivalent of the TUI flow.

If you have registered user modules (via 'terraclaw add-module'), use --use-modules
or --auto-modules to inject them as hard constraints during code generation.
Matched modules take priority over public registry modules.

Examples:
  # AWS resources
  terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws

  # Azure resources
  terraclaw generate --resources /subscriptions/xxx/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1 --schema azure

  # With user modules (auto-select all matching)
  terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws --auto-modules

  # Schema auto-detected from resource ID format
  terraclaw generate --resources arn:aws:s3:::my-bucket --auto-modules`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().StringP("resources", "r", "", "Comma-separated list of resource ARNs or Azure resource IDs (required)")
	generateCmd.Flags().String("schema", "", "Steampipe schema to query (auto-detected from resource IDs if omitted)")
	generateCmd.Flags().Bool("use-modules", false, "Use registered user modules (matched by resource type)")
	generateCmd.Flags().Bool("auto-modules", false, "Auto-select all matching user modules (implies --use-modules)")
	_ = generateCmd.MarkFlagRequired("resources")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	resourcesFlag, _ := cmd.Flags().GetString("resources")
	schema, _ := cmd.Flags().GetString("schema")

	if resourcesFlag == "" {
		return fmt.Errorf("--resources flag is required")
	}

	// Parse resource IDs from comma-separated list.
	var resourceIDs []string
	for _, a := range strings.Split(resourcesFlag, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			resourceIDs = append(resourceIDs, a)
		}
	}
	if len(resourceIDs) == 0 {
		return fmt.Errorf("no valid resource IDs provided")
	}

	// Auto-detect cloud provider from the first resource ID if schema not specified.
	if schema == "" {
		cloud := provider.DetectFromResourceID(resourceIDs[0])
		schema = cloud.String()
		if schema == "unknown" {
			schema = "aws" // fallback
		}
		fmt.Printf("Auto-detected schema: %s\n", schema)
	}
	cloud := provider.DetectFromSchema(schema)

	// Load and configure.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if v, _ := cmd.Flags().GetString("output-dir"); v != "" {
		cfg.OutputDir = v
	}
	if v, _ := cmd.Flags().GetString("terraform-bin"); v != "" {
		cfg.TerraformBin = v
	}
	if v, _ := cmd.Flags().GetBool("debug"); v {
		cfg.Debug = true
	}

	// Initialise debug logger.
	if cfg.Debug {
		if initErr := debuglog.Init(cfg.DebugLogFile); initErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not init debug log: %v\n", initErr)
		}
		defer debuglog.Close()
		debuglog.Log("[generate] starting — resources=%v schema=%s cloud=%s outputDir=%s", resourceIDs, schema, cloud, cfg.OutputDir)
	}

	// Ensure output dir exists and resolve to absolute path.
	if err := os.MkdirAll(cfg.OutputDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	outputAbs, _ := filepath.Abs(cfg.OutputDir)
	cfg.OutputDir = outputAbs

	// Copy skills.
	cwd, _ := os.Getwd()
	skillsSrc := filepath.Join(cwd, ".agents", "skills")
	skillsDst := filepath.Join(outputAbs, ".agents", "skills")
	if err := copyDir(skillsSrc, skillsDst); err != nil {
		debuglog.Log("[generate] warning: could not copy skills: %v", err)
	} else {
		debuglog.Log("[generate] copied skills from %s to %s", skillsSrc, skillsDst)
	}

	// Connect to Steampipe.
	fmt.Println("Connecting to Steampipe...")
	spClient, err := steampipe.New(cfg.SteampipeConnStr())
	if err != nil {
		return fmt.Errorf("connect to steampipe: %w\n\nMake sure Steampipe is running: steampipe service start", err)
	}
	defer spClient.Close()

	// Look up resources by ID.
	fmt.Printf("Looking up %d resource(s)...\n", len(resourceIDs))
	resources, err := spClient.FetchResourcesByIDs(schema, resourceIDs)
	if err != nil {
		return fmt.Errorf("fetch resources: %w", err)
	}
	fmt.Printf("Found %d resource(s):\n", len(resources))
	for i, r := range resources {
		fmt.Printf("   %d. [%s] %s — %s\n", i+1, r.Type, r.Name, r.ID)
	}

	// Start OpenCode server.
	fmt.Println("\nStarting OpenCode server...")
	ocServer, err := opencode.StartServer(context.Background(), cfg.OpencodePort, outputAbs)
	if err != nil {
		return fmt.Errorf("start opencode server: %w\n\nMake sure OpenCode is installed: brew install anomalyco/tap/opencode", err)
	}
	defer ocServer.Stop()

	// Create session and message tracker.
	fmt.Println("Creating OpenCode session...")
	sessionID, err := ocServer.CreateSession("terraclaw-terraform-generation")
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	tracker := opencode.NewMessageTracker()

	// ---------------------------------------------------------------
	// Module Matching (optional)
	// ---------------------------------------------------------------
	useModules, _ := cmd.Flags().GetBool("use-modules")
	autoModules, _ := cmd.Flags().GetBool("auto-modules")
	if autoModules {
		useModules = true
	}

	var selectedModules []modules.FitResult
	if useModules {
		modStore, modErr := modules.Open(cfg.ModulesDBPath())
		if modErr != nil {
			fmt.Printf("Warning: could not open module store: %v\n", modErr)
		} else {
			defer modStore.Close()
			allMods, listErr := modStore.ListModules()
			if listErr == nil && len(allMods) > 0 {
				// Extract resource types from discovered resources.
				typeSet := make(map[string]bool)
				for _, r := range resources {
					typeSet[r.Type] = true
				}
				var targetTypes []string
				for t := range typeSet {
					targetTypes = append(targetTypes, t)
				}

				matched := modules.MatchModules(allMods, targetTypes)
				if autoModules {
					// Auto-select all with positive score.
					selectedModules = matched
				} else {
					// Select only those with score >= 60%.
					for _, fit := range matched {
						if fit.Selected {
							selectedModules = append(selectedModules, fit)
						}
					}
				}

				if len(selectedModules) > 0 {
					fmt.Printf("\nMatched %d user module(s):\n", len(selectedModules))
					for _, fit := range selectedModules {
						fmt.Printf("  - %s (%d%% fit) — covers: %s\n",
							fit.Module.Name, fit.ScorePercent(), strings.Join(fit.MatchedTypes, ", "))
					}
				}
			}
		}
	}

	// ---------------------------------------------------------------
	// Stage 1: Blueprint Generation
	// ---------------------------------------------------------------
	fmt.Println("\n--- Stage 1: Generating Blueprint ---")

	// Inject Stage 1 system prompt with optional user module constraints.
	systemPrompt := llm.BuildStage1SystemPrompt(cloud)
	if len(selectedModules) > 0 {
		moduleSection := modules.BuildModuleCatalogPrompt(selectedModules)
		if moduleSection != "" {
			systemPrompt = systemPrompt + "\n\n" + moduleSection
			debuglog.Log("[generate] injected %d user module constraint(s)", len(selectedModules))
		}
	}
	if err := ocServer.InjectSystemPrompt(sessionID, systemPrompt); err != nil {
		return fmt.Errorf("stage 1 (blueprint) failed: inject system prompt: %w", err)
	}

	// Build and send Stage 1 user prompt.
	stage1Prompt := llm.BuildStage1UserPrompt(resources, cloud)
	debuglog.Log("[generate] sending stage 1 prompt (%d bytes)", len(stage1Prompt))

	stage1Response, err := pollPrompt(ocServer, sessionID, stage1Prompt, tracker)
	if err != nil {
		return fmt.Errorf("stage 1 (blueprint) failed: %w", err)
	}

	// Extract and persist blueprint.
	blueprint, err := llm.ExtractBlueprint(stage1Response)
	if err != nil {
		return fmt.Errorf("stage 1 (blueprint) failed: %w", err)
	}
	if err := llm.PersistBlueprint(blueprint, cfg.OutputDir); err != nil {
		return fmt.Errorf("stage 1 (blueprint) failed: persist blueprint: %w", err)
	}
	debuglog.Log("[generate] blueprint persisted to %s/blueprint.yaml", cfg.OutputDir)

	// ---------------------------------------------------------------
	// Stage 2: Terraform Code Generation
	// ---------------------------------------------------------------
	fmt.Println("\n--- Stage 2: Generating Terraform Code ---")

	// Read blueprint back from disk (source of truth).
	blueprintFromDisk, err := llm.ReadBlueprint(cfg.OutputDir)
	if err != nil {
		return fmt.Errorf("stage 2 (terraform) failed: read blueprint: %w", err)
	}

	// Build and send Stage 2 prompt in the same session.
	stage2Prompt := llm.BuildStage2Prompt(blueprintFromDisk, outputAbs, cloud)
	debuglog.Log("[generate] sending stage 2 prompt (%d bytes)", len(stage2Prompt))

	_, err = pollPrompt(ocServer, sessionID, stage2Prompt, tracker)
	if err != nil {
		return fmt.Errorf("stage 2 (terraform) failed: %w", err)
	}

	// Scan for generated files recursively.
	files, err := llm.RecursiveListGeneratedFiles(cfg.OutputDir)
	if err != nil {
		return fmt.Errorf("list generated files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("stage 2 (terraform) failed: no files were generated")
	}

	fmt.Printf("\nGenerated %d file(s) in %s:\n", len(files), cfg.OutputDir)
	for _, f := range files {
		fmt.Printf("   %s (%d bytes)\n", f.Name, len(f.Content))
	}

	// ---------------------------------------------------------------
	// Stage 3: Import & Validation
	// ---------------------------------------------------------------
	fmt.Println("\n--- Stage 3: Import & Validation ---")

	var finalImportResult *llm.ImportStageResult
	for iteration := 1; iteration <= llm.MaxRefinementIterations; iteration++ {
		var stage3Prompt string
		if iteration == 1 {
			stage3Prompt = llm.BuildStage3Prompt(outputAbs, iteration, llm.MaxRefinementIterations, cloud)
		} else {
			stage3Prompt = llm.BuildRefinementPrompt(outputAbs, iteration, llm.MaxRefinementIterations, cloud)
		}

		fmt.Printf("\nIteration %d/%d: Running imports and refining...\n", iteration, llm.MaxRefinementIterations)
		debuglog.Log("[generate] sending stage 3 prompt (iteration %d, %d bytes)", iteration, len(stage3Prompt))

		stage3Response, stage3Err := pollPrompt(ocServer, sessionID, stage3Prompt, tracker)
		if stage3Err != nil {
			fmt.Printf("   Stage 3 prompt failed (iteration %d): %v\n", iteration, stage3Err)
			if iteration < llm.MaxRefinementIterations {
				fmt.Println("   Continuing to next iteration...")
				continue
			}
			break
		}

		importResult, parseErr := llm.ExtractImportResult(stage3Response)
		if parseErr != nil {
			fmt.Printf("   Could not parse import result: %v\n", parseErr)
			if iteration < llm.MaxRefinementIterations {
				fmt.Println("   Continuing to next iteration...")
				continue
			}
			break
		}

		finalImportResult = importResult
		fmt.Printf("   Successful: %d, Failed: %d (status: %s)\n",
			importResult.Successful, importResult.Failed, importResult.Status)

		if importResult.Status == "success" {
			fmt.Println("\nAll imports successful! Terraform state file generated.")
			break
		}

		if iteration == llm.MaxRefinementIterations {
			fmt.Printf("\nReached max iterations (%d). Some imports may have failed.\n",
				llm.MaxRefinementIterations)
		}
	}

	// Rescan files since Stage 3 may have modified .tf files.
	files, err = llm.RecursiveListGeneratedFiles(cfg.OutputDir)
	if err != nil {
		debuglog.Log("[generate] warning: rescan after stage 3 failed: %v", err)
	} else {
		fmt.Printf("\nFinal file listing (%d files in %s):\n", len(files), cfg.OutputDir)
		for _, f := range files {
			fmt.Printf("   %s (%d bytes)\n", f.Name, len(f.Content))
		}
	}

	if finalImportResult != nil && finalImportResult.Status == "success" {
		fmt.Println("\nTerraform state file generated successfully!")
	}

	// Print the equivalent CLI command for reference.
	fmt.Printf("\nTo re-run this exact generation:\n")
	fmt.Printf("   terraclaw generate --resources %s --schema %s\n", strings.Join(resourceIDs, ","), schema)
	return nil
}

// pollPrompt sends a prompt asynchronously and polls for new message parts,
// printing all content (thinking, text, tool use, tool results) to stdout
// and logging to the debug log. Returns the final response text.
func pollPrompt(server *opencode.Server, sessionID, prompt string, tracker *opencode.MessageTracker) (string, error) {
	resultCh := server.PromptAsync(sessionID, prompt)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case result := <-resultCh:
			// Drain any remaining new parts before returning.
			drainNewParts(server, sessionID, tracker)
			if result.Err != nil {
				return "", result.Err
			}
			return result.Response, nil

		case <-ticker.C:
			messages, err := server.ListMessages(sessionID)
			if err != nil {
				debuglog.Log("[generate] poll error: %v", err)
				continue
			}
			newParts := tracker.NewParts(messages)
			for _, tp := range newParts {
				printAndLogPart(tp)
			}
		}
	}
}

// drainNewParts fetches messages one last time to print anything that arrived
// between the last tick and the prompt completing.
func drainNewParts(server *opencode.Server, sessionID string, tracker *opencode.MessageTracker) {
	messages, err := server.ListMessages(sessionID)
	if err != nil {
		return
	}
	for _, tp := range tracker.NewParts(messages) {
		printAndLogPart(tp)
	}
}

// printAndLogPart prints a tracked message part to stdout and logs it.
func printAndLogPart(tp opencode.TrackedPart) {
	part := tp.Part

	switch {
	case part.IsThinking():
		text := strings.TrimSpace(part.Text)
		if text == "" {
			return
		}
		debuglog.Log("[agent:thinking] %s", text)
		// Print thinking content with a prefix for clarity.
		for _, line := range strings.Split(text, "\n") {
			fmt.Printf("   [thinking] %s\n", line)
		}

	case part.IsText():
		text := strings.TrimSpace(part.Text)
		if text == "" {
			return
		}
		debuglog.Log("[agent:text] %s", text)
		for _, line := range strings.Split(text, "\n") {
			fmt.Printf("   %s\n", line)
		}

	case part.IsToolUse():
		state := part.StateString()
		if state == "" {
			state = "running"
		}
		debuglog.Log("[agent:tool] %s (%s)", part.ToolName, state)
		fmt.Printf("   [tool] %s (%s)\n", part.ToolName, state)
		// Print tool input so the exact command is visible.
		if input := string(part.Input); input != "" && input != "null" && input != "{}" {
			debuglog.Log("[agent:tool-input] %s", input)
			fmt.Printf("   [input] %s\n", input)
		}

	case part.IsToolResult():
		output := part.OutputString()
		debuglog.Log("[agent:tool-result] %s", output)
		// Print full tool results — no truncation.
		if output != "" {
			for _, line := range strings.Split(output, "\n") {
				fmt.Printf("   [result] %s\n", line)
			}
		}

	default:
		if part.Text != "" {
			debuglog.Log("[agent:%s] %s", part.Type, part.Text)
			fmt.Printf("   [%s] %s\n", part.Type, part.Text)
		}
	}
}

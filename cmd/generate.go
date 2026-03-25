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
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Terraform code for specified resources (non-interactive)",
	Long: `Generate Terraform code directly by specifying resource ARNs.
This is the non-interactive equivalent of the TUI flow.

Example:
  terraclaw generate --resources arn:aws:cognito-idp:eu-central-1:123456:userpool/pool-id,arn:aws:s3:::my-bucket
  terraclaw generate --resources arn:aws:lambda:us-east-1:123456:function:my-func --schema aws`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().StringP("resources", "r", "", "Comma-separated list of resource ARNs to generate Terraform for (required)")
	generateCmd.Flags().String("schema", "aws", "Steampipe schema to query (default: aws)")
	_ = generateCmd.MarkFlagRequired("resources")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	resourcesFlag, _ := cmd.Flags().GetString("resources")
	schema, _ := cmd.Flags().GetString("schema")

	if resourcesFlag == "" {
		return fmt.Errorf("--resources flag is required")
	}

	// Parse ARNs from comma-separated list.
	var arns []string
	for _, a := range strings.Split(resourcesFlag, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			arns = append(arns, a)
		}
	}
	if len(arns) == 0 {
		return fmt.Errorf("no valid ARNs provided")
	}

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
		debuglog.Log("[generate] starting — arns=%v schema=%s outputDir=%s", arns, schema, cfg.OutputDir)
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
	fmt.Println("🔌 Connecting to Steampipe...")
	spClient, err := steampipe.New(cfg.SteampipeConnStr())
	if err != nil {
		return fmt.Errorf("connect to steampipe: %w\n\nMake sure Steampipe is running: steampipe service start", err)
	}
	defer spClient.Close()

	// Look up resources by ARN.
	fmt.Printf("🔍 Looking up %d resource(s) by ARN...\n", len(arns))
	resources, err := spClient.FetchResourcesByARNs(schema, arns)
	if err != nil {
		return fmt.Errorf("fetch resources: %w", err)
	}
	fmt.Printf("✅ Found %d resource(s):\n", len(resources))
	for i, r := range resources {
		fmt.Printf("   %d. [%s] %s — %s\n", i+1, r.Type, r.Name, r.ID)
	}

	// Start OpenCode server.
	fmt.Println("\n🤖 Starting OpenCode server...")
	ocServer, err := opencode.StartServer(context.Background(), cfg.OpencodePort, outputAbs)
	if err != nil {
		return fmt.Errorf("start opencode server: %w\n\nMake sure OpenCode is installed: brew install anomalyco/tap/opencode", err)
	}
	defer ocServer.Stop()

	// Create session.
	fmt.Println("📝 Creating OpenCode session...")
	sessionID, err := ocServer.CreateSession("terraclaw-terraform-generation")
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// ---------------------------------------------------------------
	// Stage 1: Blueprint Generation
	// ---------------------------------------------------------------
	fmt.Println("Stage 1: Generating Blueprint...")

	// Inject Stage 1 system prompt.
	if err := ocServer.InjectSystemPrompt(sessionID, llm.BuildStage1SystemPrompt()); err != nil {
		return fmt.Errorf("stage 1 (blueprint) failed: inject system prompt: %w", err)
	}

	// Build and send Stage 1 user prompt.
	stage1Prompt := llm.BuildStage1UserPrompt(resources)
	debuglog.Log("[generate] sending stage 1 prompt (%d bytes)", len(stage1Prompt))

	fmt.Println("⏳ Waiting for blueprint generation (this may take a few minutes)...")

	// Send Stage 1 prompt asynchronously and poll for status.
	resultCh := ocServer.PromptAsync(sessionID, stage1Prompt)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var stage1Response string
	for {
		done := false
		select {
		case result := <-resultCh:
			if result.Err != nil {
				return fmt.Errorf("stage 1 (blueprint) failed: %w", result.Err)
			}
			stage1Response = result.Response
			done = true

		case <-ticker.C:
			messages, err := ocServer.ListMessages(sessionID)
			if err != nil {
				continue
			}
			if len(messages) > 0 {
				latest := messages[len(messages)-1]
				if latest.Info.Role == "assistant" {
					for _, part := range latest.Parts {
						if part.ToolName != "" {
							state := part.StateString()
							if state == "" {
								state = "running"
							}
							fmt.Printf("   🔧 %s (%s)\n", part.ToolName, state)
						}
					}
				}
			}
		}
		if done {
			break
		}
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
	fmt.Println("Stage 2: Generating Terraform Code...")

	// Read blueprint back from disk (source of truth).
	blueprintFromDisk, err := llm.ReadBlueprint(cfg.OutputDir)
	if err != nil {
		return fmt.Errorf("stage 2 (terraform) failed: read blueprint: %w", err)
	}

	// Build and send Stage 2 prompt in the same session.
	stage2Prompt := llm.BuildStage2Prompt(blueprintFromDisk, outputAbs)
	debuglog.Log("[generate] sending stage 2 prompt (%d bytes)", len(stage2Prompt))

	fmt.Println("⏳ Waiting for Terraform code generation (this may take a few minutes)...")

	resultCh = ocServer.PromptAsync(sessionID, stage2Prompt)

	for {
		done := false
		select {
		case result := <-resultCh:
			if result.Err != nil {
				return fmt.Errorf("stage 2 (terraform) failed: %w", result.Err)
			}
			done = true

		case <-ticker.C:
			messages, err := ocServer.ListMessages(sessionID)
			if err != nil {
				continue
			}
			if len(messages) > 0 {
				latest := messages[len(messages)-1]
				if latest.Info.Role == "assistant" {
					for _, part := range latest.Parts {
						if part.ToolName != "" {
							state := part.StateString()
							if state == "" {
								state = "running"
							}
							fmt.Printf("   🔧 %s (%s)\n", part.ToolName, state)
						}
					}
				}
			}
		}
		if done {
			break
		}
	}

	// Scan for generated files recursively.
	files, err := llm.RecursiveListGeneratedFiles(cfg.OutputDir)
	if err != nil {
		return fmt.Errorf("list generated files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("stage 2 (terraform) failed: no files were generated")
	}

	fmt.Printf("\n✅ Generated %d file(s) in %s:\n", len(files), cfg.OutputDir)
	for _, f := range files {
		fmt.Printf("   📄 %s (%d bytes)\n", f.Name, len(f.Content))
	}

	// ---------------------------------------------------------------
	// Stage 3: Import & Validation
	// ---------------------------------------------------------------
	fmt.Println("\n📥 Stage 3: Import & Validation...")

	var finalImportResult *llm.ImportStageResult
	for iteration := 1; iteration <= llm.MaxRefinementIterations; iteration++ {
		var stage3Prompt string
		if iteration == 1 {
			stage3Prompt = llm.BuildStage3Prompt(outputAbs, iteration, llm.MaxRefinementIterations)
		} else {
			stage3Prompt = llm.BuildRefinementPrompt(outputAbs, iteration, llm.MaxRefinementIterations)
		}

		fmt.Printf("⏳ Iteration %d/%d: Running imports and refining...\n", iteration, llm.MaxRefinementIterations)
		debuglog.Log("[generate] sending stage 3 prompt (iteration %d, %d bytes)", iteration, len(stage3Prompt))

		resultCh = ocServer.PromptAsync(sessionID, stage3Prompt)

		var stage3Response string
		stage3Err := false
		for {
			done := false
			select {
			case result := <-resultCh:
				if result.Err != nil {
					fmt.Printf("   ❌ Stage 3 prompt failed (iteration %d): %v\n", iteration, result.Err)
					stage3Err = true
					done = true
				} else {
					stage3Response = result.Response
					done = true
				}
			case <-ticker.C:
				messages, err := ocServer.ListMessages(sessionID)
				if err != nil {
					continue
				}
				if len(messages) > 0 {
					latest := messages[len(messages)-1]
					if latest.Info.Role == "assistant" {
						for _, part := range latest.Parts {
							if part.ToolName != "" {
								state := part.StateString()
								if state == "" {
									state = "running"
								}
								fmt.Printf("   🔧 %s (%s)\n", part.ToolName, state)
							}
						}
					}
				}
			}
			if done {
				break
			}
		}

		if stage3Err {
			if iteration < llm.MaxRefinementIterations {
				fmt.Println("   Continuing to next iteration...")
				continue
			}
			break
		}

		importResult, parseErr := llm.ExtractImportResult(stage3Response)
		if parseErr != nil {
			fmt.Printf("   ⚠️  Could not parse import result: %v\n", parseErr)
			if iteration < llm.MaxRefinementIterations {
				fmt.Println("   Continuing to next iteration...")
				continue
			}
			break
		}

		finalImportResult = importResult
		fmt.Printf("   ✅ Successful: %d, ❌ Failed: %d (status: %s)\n",
			importResult.Successful, importResult.Failed, importResult.Status)

		if importResult.Status == "success" {
			fmt.Println("\n🎉 All imports successful! Terraform state file generated.")
			break
		}

		if iteration == llm.MaxRefinementIterations {
			fmt.Printf("\n⚠️  Reached max iterations (%d). Some imports may have failed.\n",
				llm.MaxRefinementIterations)
		}
	}

	// Rescan files since Stage 3 may have modified .tf files.
	files, err = llm.RecursiveListGeneratedFiles(cfg.OutputDir)
	if err != nil {
		debuglog.Log("[generate] warning: rescan after stage 3 failed: %v", err)
	} else {
		fmt.Printf("\n📂 Final file listing (%d files in %s):\n", len(files), cfg.OutputDir)
		for _, f := range files {
			fmt.Printf("   📄 %s (%d bytes)\n", f.Name, len(f.Content))
		}
	}

	if finalImportResult != nil && finalImportResult.Status == "success" {
		fmt.Println("\n✅ Terraform state file generated successfully!")
	}

	// Print the equivalent CLI command for reference.
	fmt.Printf("\n💡 To re-run this exact generation:\n")
	fmt.Printf("   terraclaw generate --resources %s --schema %s\n", strings.Join(arns, ","), schema)
	return nil
}

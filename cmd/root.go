// Package cmd provides the CLI commands for terraclaw.
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	"github.com/arunim2405/terraclaw/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "terraclaw",
	Short: "Convert existing cloud resources to Terraform configuration using AI",
	Long: `terraclaw is an interactive CLI tool that:
  • Connects to Steampipe to discover existing cloud resources
  • Lets you select resources using an interactive TUI
  • Uses your choice of LLM (ChatGPT, Claude or Gemini) to generate Terraform HCL
  • Runs terraform import to create state files for the resources`,
	RunE: runInteractive,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("output-dir", ".", "Directory to write generated Terraform files")
	rootCmd.PersistentFlags().String("terraform-bin", "terraform", "Path to the terraform binary")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging to file (see DEBUG_LOG_FILE)")
}

// runInteractive starts the BubbleTea TUI.
func runInteractive(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override config from flags if provided.
	if v, _ := cmd.Flags().GetString("output-dir"); v != "" {
		cfg.OutputDir = v
	}
	if v, _ := cmd.Flags().GetString("terraform-bin"); v != "" {
		cfg.TerraformBin = v
	}
	if v, _ := cmd.Flags().GetBool("debug"); v {
		cfg.Debug = true
	}

	// Initialise debug logger before anything else.
	if cfg.Debug {
		if initErr := debuglog.Init(cfg.DebugLogFile); initErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not init debug log: %v\n", initErr)
		}
		defer debuglog.Close()
		debuglog.Log("[startup] terraclaw starting — provider=%s outputDir=%s", cfg.LLMProvider, cfg.OutputDir)
	}
	// Connect to Steampipe.
	spClient, err := steampipe.New(cfg.SteampipeConnStr())
	if err != nil {
		return fmt.Errorf("connect to steampipe: %w\n\nMake sure Steampipe is running: steampipe service start", err)
	}
	defer spClient.Close()

	// Fetch available schemas (cloud provider plugins).
	schemas, err := spClient.ListSchemas()
	if err != nil {
		return fmt.Errorf("list steampipe schemas: %w", err)
	}
	if len(schemas) == 0 {
		return fmt.Errorf("no Steampipe plugin schemas found; install a plugin first:\n  steampipe plugin install aws")
	}

	// Wire up the TUI with the config and steampipe client.
	tui.SetConfig(cfg)
	tui.SetSteampipeClient(spClient)

	model := tui.New(schemas, string(cfg.LLMProvider))
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

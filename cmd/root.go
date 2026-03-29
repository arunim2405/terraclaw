// Package cmd provides the CLI commands for terraclaw.
package cmd

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	"github.com/arunim2405/terraclaw/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "terraclaw",
	Short: "Convert existing cloud resources to Terraform configuration using AI",
	Long: `terraclaw is an interactive CLI tool that:
  • Connects to Steampipe to discover existing cloud resources
  • Builds a dependency graph of related resources
  • Uses OpenCode (AI coding agent) to generate Terraform HCL
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
	rootCmd.PersistentFlags().String("output-dir", "./output", "Directory to write generated Terraform files")
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
		debuglog.Log("[startup] terraclaw starting — outputDir=%s opencodePort=%d", cfg.OutputDir, cfg.OpencodePort)
	}

	// Ensure the output directory exists and resolve it to an absolute path.
	if err := os.MkdirAll(cfg.OutputDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	outputAbs, _ := filepath.Abs(cfg.OutputDir)
	cfg.OutputDir = outputAbs

	// Copy .agents/skills from the project root into the output directory
	// so OpenCode can discover Terraform skills (style guide, modules, etc.).
	cwd, _ := os.Getwd()
	skillsSrc := filepath.Join(cwd, ".agents", "skills")
	skillsDst := filepath.Join(outputAbs, ".agents", "skills")
	if err := copyDir(skillsSrc, skillsDst); err != nil {
		debuglog.Log("[startup] warning: could not copy skills: %v", err)
	} else {
		debuglog.Log("[startup] copied skills from %s to %s", skillsSrc, skillsDst)
	}

	// Start the OpenCode server with cwd set to the output directory
	// so it writes .tf files directly there.
	ocServer, err := opencode.StartServer(context.Background(), cfg.OpencodePort, outputAbs)
	if err != nil {
		return fmt.Errorf("start opencode server: %w\n\nMake sure OpenCode is installed: brew install anomalyco/tap/opencode", err)
	}
	defer ocServer.Stop()

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

	// Wire up the TUI with the config, steampipe client, and opencode server.
	tui.SetConfig(cfg)
	tui.SetSteampipeClient(spClient)
	tui.SetOpencodeServer(ocServer)

	model := tui.New(schemas)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

// copyDir recursively copies src directory to dst.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute the relative path and destination path.
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o750)
		}
		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

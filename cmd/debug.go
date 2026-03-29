package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Print configuration summary and test Steampipe connectivity",
	Long: `debug prints a redacted summary of the loaded configuration (API keys
are masked) and tests the Steampipe connection without launching the TUI.

This is useful for diagnosing configuration problems and verifying that all
required environment variables are present before running terraclaw.`,
	RunE: runDebug,
}

func init() {
	rootCmd.AddCommand(debugCmd)
}

func runDebug(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Honour --debug flag.
	if v, _ := cmd.Flags().GetBool("debug"); v {
		cfg.Debug = true
	}
	if cfg.Debug {
		if initErr := debuglog.Init(cfg.DebugLogFile); initErr != nil {
			fmt.Printf("warning: could not init debug log: %v\n", initErr)
		}
		defer debuglog.Close()
	}

	debuglog.Log("[debug cmd] starting config dump")

	fmt.Println("=== terraclaw debug ===")
	fmt.Println()

	// --- Configuration summary ---
	fmt.Println("Configuration:")
	fmt.Printf("  OPENCODE_PORT     : %d\n", cfg.OpencodePort)
	fmt.Printf("  STEAMPIPE_HOST    : %s\n", cfg.SteampipeHost)
	fmt.Printf("  STEAMPIPE_PORT    : %s\n", cfg.SteampipePort)
	fmt.Printf("  STEAMPIPE_DB      : %s\n", cfg.SteampipeDB)
	fmt.Printf("  STEAMPIPE_USER    : %s\n", cfg.SteampipeUser)
	fmt.Printf("  TERRAFORM_BIN     : %s\n", cfg.TerraformBin)
	fmt.Printf("  OUTPUT_DIR        : %s\n", cfg.OutputDir)
	fmt.Printf("  SCAN_TABLES       : %s\n", cfg.ScanTables)
	fmt.Printf("  DEBUG             : %v\n", cfg.Debug)
	fmt.Printf("  DEBUG_LOG_FILE    : %s\n", cfg.DebugLogFile)
	fmt.Println()

	// --- OpenCode validation ---
	fmt.Println("OpenCode coding agent:")
	if err := cfg.Validate(); err != nil {
		fmt.Printf("  [FAIL] %v\n", err)
	} else {
		fmt.Printf("  [OK]   configuration valid (LLM provider is configured in OpenCode)\n")
	}
	fmt.Println()

	// --- Steampipe connectivity ---
	fmt.Println("Steampipe connectivity:")
	debuglog.Log("[debug cmd] connecting to Steampipe at %s", cfg.SteampipeConnStr())
	spClient, err := steampipe.New(cfg.SteampipeConnStr())
	if err != nil {
		fmt.Printf("  [FAIL] connect: %v\n", err)
		fmt.Println("         fix: run `steampipe service start`")
		return nil
	}
	defer spClient.Close()
	fmt.Println("  [OK]   connected")

	schemas, err := spClient.ListSchemas()
	if err != nil {
		fmt.Printf("  [FAIL] list schemas: %v\n", err)
		return nil
	}
	if len(schemas) == 0 {
		fmt.Println("  [WARN] no plugin schemas found — install one: steampipe plugin install aws")
	} else {
		fmt.Printf("  [OK]   %d schema(s): %s\n", len(schemas), strings.Join(schemas, ", "))
	}
	debuglog.Log("[debug cmd] schemas: %v", schemas)
	fmt.Println()

	if cfg.Debug {
		fmt.Printf("Debug log: %s\n", cfg.DebugLogFile)
	}

	return nil
}

// maskKey redacts an API key, showing only the last 4 characters.
func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

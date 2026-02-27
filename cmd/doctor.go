package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/doctor"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local dependencies and configuration required by terraclaw",
	RunE: func(cmd *cobra.Command, _ []string) error {
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

		report := doctor.Run(cfg, doctor.DefaultDeps())
		for _, check := range report.Checks {
			if check.OK {
				fmt.Printf("[OK]   %s: %s\n", check.Name, check.Details)
				continue
			}
			fmt.Printf("[FAIL] %s: %s\n", check.Name, check.Details)
			if check.Fix != "" {
				fmt.Printf("       fix: %s\n", check.Fix)
			}
		}

		if report.HasFailures() {
			return fmt.Errorf("doctor found missing or misconfigured dependencies")
		}
		fmt.Println("All dependency checks passed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

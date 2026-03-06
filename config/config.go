// Package config provides configuration management for terraclaw.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	// Steampipe
	SteampipeHost     string
	SteampipePort     string
	SteampipeDB       string
	SteampipeUser     string
	SteampipePassword string

	// OpenCode (coding agent)
	OpencodePort int // default 4096

	// Terraform
	TerraformBin string

	// Output
	OutputDir string

	// Resource scanning
	ScanTables string // comma-separated table names, or "*" for all, or empty for key resources

	// Debug
	Debug        bool
	DebugLogFile string
}

// Load loads configuration from environment variables and an optional .env file.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if file not found)
	_ = godotenv.Load()

	port := 4096
	if v := os.Getenv("OPENCODE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}

	cfg := &Config{
		SteampipeHost:     envOrDefault("STEAMPIPE_HOST", "localhost"),
		SteampipePort:     envOrDefault("STEAMPIPE_PORT", "9193"),
		SteampipeDB:       envOrDefault("STEAMPIPE_DB", "steampipe"),
		SteampipeUser:     envOrDefault("STEAMPIPE_USER", "steampipe"),
		SteampipePassword: envOrDefault("STEAMPIPE_PASSWORD", ""),
		OpencodePort:      port,
		TerraformBin:      envOrDefault("TERRAFORM_BIN", "terraform"),
		OutputDir:         envOrDefault("OUTPUT_DIR", "./output"),
		ScanTables:        os.Getenv("SCAN_TABLES"),
		Debug:             os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1",
		DebugLogFile:      envOrDefault("DEBUG_LOG_FILE", "terraclaw.log"),
	}

	return cfg, nil
}

// Validate checks that required configuration values are present.
func (c *Config) Validate() error {
	// OpenCode is validated by the doctor command (binary check).
	// No API keys needed since OpenCode manages its own provider config.
	return nil
}

// SteampipeConnStr returns the PostgreSQL connection string for Steampipe.
func (c *Config) SteampipeConnStr() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		c.SteampipeHost, c.SteampipePort, c.SteampipeDB, c.SteampipeUser, c.SteampipePassword,
	)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

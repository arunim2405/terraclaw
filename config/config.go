// Package config provides configuration management for terraclaw.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LLMProvider represents the supported LLM providers.
type LLMProvider string

const (
	ProviderOpenAI    LLMProvider = "openai"
	ProviderClaude    LLMProvider = "claude"
	ProviderGemini    LLMProvider = "gemini"
)

// Config holds the application configuration.
type Config struct {
	// Steampipe
	SteampipeHost     string
	SteampipePort     string
	SteampipeDB       string
	SteampipeUser     string
	SteampipePassword string

	// LLM
	OpenAIAPIKey    string
	AnthropicAPIKey string
	GeminiAPIKey    string
	LLMProvider     LLMProvider

	// Terraform
	TerraformBin string

	// Output
	OutputDir string
}

// Load loads configuration from environment variables and an optional .env file.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if file not found)
	_ = godotenv.Load()

	cfg := &Config{
		SteampipeHost:     envOrDefault("STEAMPIPE_HOST", "localhost"),
		SteampipePort:     envOrDefault("STEAMPIPE_PORT", "9193"),
		SteampipeDB:       envOrDefault("STEAMPIPE_DB", "steampipe"),
		SteampipeUser:     envOrDefault("STEAMPIPE_USER", "steampipe"),
		SteampipePassword: envOrDefault("STEAMPIPE_PASSWORD", ""),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		LLMProvider:       LLMProvider(envOrDefault("LLM_PROVIDER", string(ProviderOpenAI))),
		TerraformBin:      envOrDefault("TERRAFORM_BIN", "terraform"),
		OutputDir:         envOrDefault("OUTPUT_DIR", "."),
	}

	return cfg, nil
}

// Validate checks that required configuration values are present.
func (c *Config) Validate() error {
	switch c.LLMProvider {
	case ProviderOpenAI:
		if c.OpenAIAPIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY is required when using openai provider")
		}
	case ProviderClaude:
		if c.AnthropicAPIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY is required when using claude provider")
		}
	case ProviderGemini:
		if c.GeminiAPIKey == "" {
			return fmt.Errorf("GEMINI_API_KEY is required when using gemini provider")
		}
	default:
		return fmt.Errorf("unknown LLM provider %q; valid options: openai, claude, gemini", c.LLMProvider)
	}
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

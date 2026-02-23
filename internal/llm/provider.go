// Package llm provides a unified interface for LLM providers.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// GenerateTerraform asks the LLM to generate Terraform HCL code for the
	// given cloud resources.
	GenerateTerraform(ctx context.Context, resources []steampipe.Resource) (string, error)

	// Name returns the display name of the provider.
	Name() string
}

// New returns the appropriate Provider implementation for the given config.
func New(cfg *config.Config) (Provider, error) {
	switch cfg.LLMProvider {
	case config.ProviderOpenAI:
		return NewOpenAI(cfg.OpenAIAPIKey), nil
	case config.ProviderClaude:
		return NewClaude(cfg.AnthropicAPIKey), nil
	case config.ProviderGemini:
		return NewGemini(cfg.GeminiAPIKey), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.LLMProvider)
	}
}

// buildPrompt constructs the prompt sent to the LLM.
func buildPrompt(resources []steampipe.Resource) string {
	var sb strings.Builder
	sb.WriteString("You are a Terraform expert. Generate complete, valid Terraform HCL configuration code to import and manage the following existing cloud resources.\n\n")
	sb.WriteString("For each resource:\n")
	sb.WriteString("1. Create the appropriate terraform resource block\n")
	sb.WriteString("2. Include all required arguments\n")
	sb.WriteString("3. Set values based on the resource properties provided\n")
	sb.WriteString("4. Add a comment with the terraform import command needed\n\n")
	sb.WriteString("Resources:\n\n")

	for i, r := range resources {
		sb.WriteString(fmt.Sprintf("Resource %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  Provider: %s\n", r.Provider))
		sb.WriteString(fmt.Sprintf("  Type: %s\n", r.Type))
		sb.WriteString(fmt.Sprintf("  Name: %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("  ID: %s\n", r.ID))
		if r.Region != "" {
			sb.WriteString(fmt.Sprintf("  Region: %s\n", r.Region))
		}
		sb.WriteString("  Properties:\n")
		for k, v := range r.Properties {
			if v != "" && len(v) < 200 {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nReturn ONLY the Terraform HCL code. Include terraform import commands as comments above each resource block.\n")
	return sb.String()
}

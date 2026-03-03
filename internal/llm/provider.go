// Package llm provides a unified interface for LLM providers.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
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
	var p Provider
	switch cfg.LLMProvider {
	case config.ProviderOpenAI:
		p = NewOpenAI(cfg.OpenAIAPIKey)
	case config.ProviderClaude:
		p = NewClaude(cfg.AnthropicAPIKey)
	case config.ProviderGemini:
		p = NewGemini(cfg.GeminiAPIKey)
	case config.ProviderAzureOpenAI:
		p = NewAzureOpenAI(cfg.AzureOpenAIAPIKey, cfg.AzureOpenAIEndpoint, cfg.AzureOpenAIAPIVersion, cfg.AzureOpenAIDeployment, AzureModelType(cfg.AzureOpenAIModelType))
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.LLMProvider)
	}
	debuglog.Log("[llm] provider instantiated: %s", p.Name())
	return p, nil
}

// BuildSystemPrompt returns the system-level instruction sent to the LLM.
// It encodes the HashiCorp Terraform style guide so that generated code
// follows official conventions and best practices.
// Reference: https://github.com/hashicorp/agent-skills
func BuildSystemPrompt() string {
	return `You are a Terraform expert that generates production-quality HCL configuration code following HashiCorp's official style guide.

## Code Generation Strategy
1. Start with provider configuration and version constraints (terraform.tf / providers.tf)
2. Create data sources before dependent resources
3. Build resources in dependency order
4. Add outputs for key resource attributes
5. Use variables for all configurable values

## File Organization (in a single output, use comments to delineate sections)
- terraform {} block with required_version >= 1.7 and required_providers with pinned versions (~> 5.0 for AWS)
- provider configuration with region and default_tags
- locals for common tags (ManagedBy = "Terraform")
- resource blocks, ordered by dependency
- output blocks for key IDs (vpc_id, instance_id, etc.)

## Code Formatting
- Two spaces per nesting level (no tabs)
- Align equals signs for consecutive arguments
- Arguments precede blocks; meta-arguments (count, for_each, depends_on) first
- lifecycle blocks last
- Blank line between blocks

## Naming Conventions
- Lowercase with underscores for all names
- Descriptive nouns, excluding the resource type (e.g. "web" not "aws_instance_web")
- Singular, not plural
- Default to "main" when only one instance exists

## Variables & Outputs
- Every variable must have type and description
- Every output must have description
- Mark sensitive values with sensitive = true

## Dynamic Resources
- Prefer for_each over count for multiple resources
- Use count only for conditional creation (count = var.enabled ? 1 : 0)

## Security Best Practices
- Enable encryption at rest by default
- Configure private networking where applicable
- Apply least-privilege for security groups and IAM
- Enable logging and monitoring
- Never hardcode credentials or secrets
- Mark sensitive outputs with sensitive = true

## Import Support
- Add a comment above each resource block with the terraform import command
- Format: # terraform import <resource_type>.<resource_name> <id>`
}

// buildPrompt constructs the user-facing prompt (resource list) sent to the LLM.
func buildPrompt(resources []steampipe.Resource) string {
	var sb strings.Builder
	sb.WriteString("Generate complete, valid Terraform HCL configuration to import and manage the following existing cloud resources.\n\n")
	sb.WriteString("For each resource:\n")
	sb.WriteString("1. Create the appropriate terraform resource block with all required arguments\n")
	sb.WriteString("2. Set values based on the resource properties provided\n")
	sb.WriteString("3. Add a comment with the terraform import command above each block\n")
	sb.WriteString("4. Cross-reference related resources (e.g. use aws_vpc.main.id instead of hardcoding VPC IDs if both VPC and subnet are provided)\n")
	sb.WriteString("5. Follow the HashiCorp Terraform style guide as specified in your instructions\n\n")

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
			if v != "" && len(v) < 500 {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nReturn ONLY the Terraform HCL code. Include terraform import commands as comments above each resource block.\n")
	sb.WriteString("If multiple related resources are provided, use resource references (e.g. aws_vpc.main.id) instead of hardcoded IDs.\n")
	return sb.String()
}

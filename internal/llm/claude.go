package llm

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// ClaudeProvider uses the Anthropic Claude API to generate Terraform code.
type ClaudeProvider struct {
	client anthropic.Client
}

// NewClaude creates a new Claude provider.
func NewClaude(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

// Name returns the provider name.
func (p *ClaudeProvider) Name() string { return "Claude (Anthropic)" }

// GenerateTerraform calls the Claude API to generate Terraform HCL code.
func (p *ClaudeProvider) GenerateTerraform(ctx context.Context, resources []steampipe.Resource) (string, error) {
	prompt := buildPrompt(resources)
	debuglog.Log("[claude] calling API model=%s resources=%d promptLen=%d", anthropic.ModelClaudeOpus4_6, len(resources), len(prompt))

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: "You are a Terraform expert that generates valid HCL configuration code."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		debuglog.Log("[claude] API error: %v", err)
		return "", fmt.Errorf("claude message: %w", err)
	}
	if len(msg.Content) == 0 {
		debuglog.Log("[claude] API returned no content")
		return "", fmt.Errorf("claude returned no content")
	}
	result := msg.Content[0].Text
	debuglog.Log("[claude] response received: %d chars", len(result))
	return result, nil
}

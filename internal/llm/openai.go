package llm

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// OpenAIProvider uses the OpenAI API to generate Terraform code.
type OpenAIProvider struct {
	client *openai.Client
}

// NewOpenAI creates a new OpenAI provider.
func NewOpenAI(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		client: openai.NewClient(apiKey),
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string { return "ChatGPT (OpenAI)" }

// GenerateTerraform calls OpenAI to generate Terraform HCL code.
func (p *OpenAIProvider) GenerateTerraform(ctx context.Context, resources []steampipe.Resource) (string, error) {
	prompt := buildPrompt(resources)
	debuglog.Log("[openai] calling API model=%s resources=%d promptLen=%d", openai.GPT4o, len(resources), len(prompt))

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: BuildSystemPrompt(),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.2,
	})
	if err != nil {
		debuglog.Log("[openai] API error: %v", err)
		return "", fmt.Errorf("openai completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		debuglog.Log("[openai] API returned no choices")
		return "", fmt.Errorf("openai returned no choices")
	}
	result := resp.Choices[0].Message.Content
	debuglog.Log("[openai] response received: %d chars", len(result))
	return result, nil
}

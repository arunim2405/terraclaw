package llm

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// AzureOpenAIProvider uses Azure AI Foundry (Azure OpenAI Service) to generate
// Terraform code. It targets a specific deployment within the Azure resource.
type AzureOpenAIProvider struct {
	client     *openai.Client
	deployment string
}

// NewAzureOpenAI creates a new AzureOpenAIProvider.
//
// apiKey is the Azure OpenAI API key, endpoint is the resource endpoint
// (e.g. "https://<resource-name>.openai.azure.com/"), and deployment is the
// deployment name (e.g. "gpt-4o").
func NewAzureOpenAI(apiKey, endpoint, deployment string) *AzureOpenAIProvider {
	cfg := openai.DefaultAzureConfig(apiKey, endpoint)
	return &AzureOpenAIProvider{
		client:     openai.NewClientWithConfig(cfg),
		deployment: deployment,
	}
}

// Name returns the provider name.
func (p *AzureOpenAIProvider) Name() string { return "Azure OpenAI (Azure AI Foundry)" }

// GenerateTerraform calls Azure OpenAI to generate Terraform HCL code.
//
// The Azure OpenAI API uses the deployment name in place of a model identifier,
// so p.deployment is passed as the Model field.
func (p *AzureOpenAIProvider) GenerateTerraform(ctx context.Context, resources []steampipe.Resource) (string, error) {
	prompt := buildPrompt(resources)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.deployment,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a Terraform expert that generates valid HCL configuration code.",
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
		return "", fmt.Errorf("azure openai completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("azure openai returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/responses"

	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// AzureModelType selects which Azure OpenAI REST endpoint to use.
type AzureModelType string

const (
	// AzureModelTypeChat uses /chat/completions — for GPT-4o, GPT-4, GPT-3.5-turbo…
	AzureModelTypeChat AzureModelType = "chat"
	// AzureModelTypeCompletions uses /completions — for codex models (gpt-5.3-codex, davinci-002…)
	AzureModelTypeCompletions AzureModelType = "completions"
	// AzureModelTypeResponses uses /responses — for o-series reasoning models and
	// newer Azure-hosted models that only support the Responses API.
	AzureModelTypeResponses AzureModelType = "responses"
)

// AzureOpenAIProvider uses Azure AI Foundry (Azure OpenAI Service) to generate
// Terraform code. It supports both chat-completions models (GPT-4o etc.) and
// legacy completions models (codex family) via the modelType selector.
type AzureOpenAIProvider struct {
	client     openai.Client
	deployment string
	modelType  AzureModelType
}

// NewAzureOpenAI creates a new AzureOpenAIProvider.
//
// modelType should be "chat" (default) for GPT-4o/GPT-4/GPT-3.5-turbo class
// deployments, or "completions" for codex-family deployments.
func NewAzureOpenAI(apiKey, endpoint, apiVersion, deployment string, modelType AzureModelType) *AzureOpenAIProvider {
	if modelType == "" {
		modelType = AzureModelTypeChat
	}
	client := openai.NewClient(
		azure.WithEndpoint(endpoint, apiVersion),
		azure.WithAPIKey(apiKey),
	)
	return &AzureOpenAIProvider{
		client:     client,
		deployment: deployment,
		modelType:  modelType,
	}
}

// Name returns the provider name.
func (p *AzureOpenAIProvider) Name() string { return "Azure OpenAI (Azure AI Foundry)" }

// GenerateTerraform calls the Azure OpenAI deployment to generate Terraform HCL.
// It dispatches to chat-completions, legacy-completions, or the Responses API
// based on the model type configured at construction time.
func (p *AzureOpenAIProvider) GenerateTerraform(ctx context.Context, resources []steampipe.Resource) (string, error) {
	prompt := buildPrompt(resources)
	debuglog.Log("[azure-openai] calling API deployment=%s modelType=%s resources=%d prompt=%s",
		p.deployment, p.modelType, len(resources), prompt)

	switch p.modelType {
	case AzureModelTypeCompletions:
		return p.generateViaCompletions(ctx, prompt)
	case AzureModelTypeResponses:
		return p.generateViaResponses(ctx, prompt)
	default:
		return p.generateViaChatCompletions(ctx, prompt)
	}
}

// generateViaChatCompletions uses the /chat/completions endpoint.
// Works with GPT-4o, GPT-4, GPT-3.5-turbo and similar chat-capable models.
func (p *AzureOpenAIProvider) generateViaChatCompletions(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: p.deployment,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(BuildSystemPrompt()),
			openai.UserMessage(prompt),
		},
		MaxCompletionTokens: openai.Int(4096),
	})
	if err != nil {
		debuglog.Log("[azure-openai] chat API error: %v", err)
		return "", fmt.Errorf("azure openai completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		debuglog.Log("[azure-openai] chat API returned no choices")
		return "", fmt.Errorf("azure openai returned no choices")
	}
	result := resp.Choices[0].Message.Content
	debuglog.Log("[azure-openai] chat response received: %d chars", len(result))
	return result, nil
}

// generateViaCompletions uses the legacy /completions endpoint.
// Required for codex-family models (e.g. gpt-5.3-codex, davinci-002).
func (p *AzureOpenAIProvider) generateViaCompletions(ctx context.Context, prompt string) (string, error) {
	// Codex models expect a single string; prepend the system instruction inline.
	fullPrompt := BuildSystemPrompt() + "\n\n" + prompt

	resp, err := p.client.Completions.New(ctx, openai.CompletionNewParams{
		Model: openai.CompletionNewParamsModel(p.deployment),
		Prompt: openai.CompletionNewParamsPromptUnion{
			OfString: openai.String(fullPrompt),
		},
		MaxTokens: openai.Int(4096),
	})
	if err != nil {
		debuglog.Log("[azure-openai] completions API error: %v", err)
		return "", fmt.Errorf("azure openai completions: %w", err)
	}
	if len(resp.Choices) == 0 {
		debuglog.Log("[azure-openai] completions API returned no choices")
		return "", fmt.Errorf("azure openai completions returned no choices")
	}

	// Trim any leading/trailing whitespace that completions models tend to add.
	result := strings.TrimSpace(resp.Choices[0].Text)
	debuglog.Log("[azure-openai] completions response received: %d chars", len(result))
	return result, nil
}

// generateViaResponses uses the /responses endpoint.
// Required for o-series reasoning models and newer Azure-hosted models that do
// not support the chat-completions or legacy-completions endpoints.
func (p *AzureOpenAIProvider) generateViaResponses(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        p.deployment,
		Instructions: openai.String(BuildSystemPrompt()),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		MaxOutputTokens: openai.Int(4096),
	})
	if err != nil {
		debuglog.Log("[azure-openai] responses API error: %v", err)
		return "", fmt.Errorf("azure openai responses: %w", err)
	}

	result := strings.TrimSpace(resp.OutputText())
	if result == "" {
		debuglog.Log("[azure-openai] responses API returned empty output")
		return "", fmt.Errorf("azure openai responses returned empty output")
	}
	debuglog.Log("[azure-openai] responses response received: %d chars", len(result))
	return result, nil
}

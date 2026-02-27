package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

const geminiModel = "gemini-2.0-flash"

// GeminiProvider uses the Google Gemini API to generate Terraform code.
type GeminiProvider struct {
	apiKey string
}

// NewGemini creates a new Gemini provider.
func NewGemini(apiKey string) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string { return "Gemini (Google)" }

// GenerateTerraform calls the Gemini API to generate Terraform HCL code.
func (p *GeminiProvider) GenerateTerraform(ctx context.Context, resources []steampipe.Resource) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  p.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		debuglog.Log("[gemini] failed to create client: %v", err)
		return "", fmt.Errorf("create gemini client: %w", err)
	}

	prompt := buildPrompt(resources)
	debuglog.Log("[gemini] calling API model=%s resources=%d promptLen=%d", geminiModel, len(resources), len(prompt))
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}

	resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(
			"You are a Terraform expert that generates valid HCL configuration code.",
			"",
		),
	})
	if err != nil {
		debuglog.Log("[gemini] API error: %v", err)
		return "", fmt.Errorf("gemini generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		debuglog.Log("[gemini] API returned no candidates")
		return "", fmt.Errorf("gemini returned no candidates")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		result += part.Text
	}
	debuglog.Log("[gemini] response received: %d chars", len(result))
	return result, nil
}

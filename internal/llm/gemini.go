package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"

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
		return "", fmt.Errorf("create gemini client: %w", err)
	}

	prompt := buildPrompt(resources)
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
		return "", fmt.Errorf("gemini generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("gemini returned no candidates")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		result += part.Text
	}
	return result, nil
}

package config_test

import (
	"os"
	"testing"

	"github.com/arunim2405/terraclaw/config"
)

func TestLoadDefaults(t *testing.T) {
	// Clear env to test defaults.
	os.Unsetenv("STEAMPIPE_HOST")
	os.Unsetenv("STEAMPIPE_PORT")
	os.Unsetenv("LLM_PROVIDER")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.SteampipeHost != "localhost" {
		t.Errorf("SteampipeHost = %q, want %q", cfg.SteampipeHost, "localhost")
	}
	if cfg.SteampipePort != "9193" {
		t.Errorf("SteampipePort = %q, want %q", cfg.SteampipePort, "9193")
	}
	if cfg.LLMProvider != config.ProviderOpenAI {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, config.ProviderOpenAI)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("STEAMPIPE_HOST", "myhost")
	t.Setenv("STEAMPIPE_PORT", "5432")
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.SteampipeHost != "myhost" {
		t.Errorf("SteampipeHost = %q, want %q", cfg.SteampipeHost, "myhost")
	}
	if cfg.LLMProvider != config.ProviderClaude {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, config.ProviderClaude)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name:    "openai without key",
			cfg:     config.Config{LLMProvider: config.ProviderOpenAI},
			wantErr: true,
		},
		{
			name:    "openai with key",
			cfg:     config.Config{LLMProvider: config.ProviderOpenAI, OpenAIAPIKey: "sk-test"},
			wantErr: false,
		},
		{
			name:    "claude without key",
			cfg:     config.Config{LLMProvider: config.ProviderClaude},
			wantErr: true,
		},
		{
			name:    "claude with key",
			cfg:     config.Config{LLMProvider: config.ProviderClaude, AnthropicAPIKey: "key"},
			wantErr: false,
		},
		{
			name:    "gemini without key",
			cfg:     config.Config{LLMProvider: config.ProviderGemini},
			wantErr: true,
		},
		{
			name:    "gemini with key",
			cfg:     config.Config{LLMProvider: config.ProviderGemini, GeminiAPIKey: "key"},
			wantErr: false,
		},
		{
			name:    "unknown provider",
			cfg:     config.Config{LLMProvider: "unknown"},
			wantErr: true,
		},
		{
			name:    "azure-openai without key",
			cfg:     config.Config{LLMProvider: config.ProviderAzureOpenAI, AzureOpenAIEndpoint: "https://example.openai.azure.com/"},
			wantErr: true,
		},
		{
			name:    "azure-openai without endpoint",
			cfg:     config.Config{LLMProvider: config.ProviderAzureOpenAI, AzureOpenAIAPIKey: "key"},
			wantErr: true,
		},
		{
			name: "azure-openai with key and endpoint",
			cfg: config.Config{
				LLMProvider:         config.ProviderAzureOpenAI,
				AzureOpenAIAPIKey:   "key",
				AzureOpenAIEndpoint: "https://example.openai.azure.com/",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSteampipeConnStr(t *testing.T) {
	cfg := config.Config{
		SteampipeHost:     "localhost",
		SteampipePort:     "9193",
		SteampipeDB:       "steampipe",
		SteampipeUser:     "steampipe",
		SteampipePassword: "",
	}
	got := cfg.SteampipeConnStr()
	want := "host=localhost port=9193 dbname=steampipe user=steampipe password= sslmode=disable"
	if got != want {
		t.Errorf("SteampipeConnStr() = %q, want %q", got, want)
	}
}

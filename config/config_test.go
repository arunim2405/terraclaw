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
	os.Unsetenv("OPENCODE_PORT")

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
	if cfg.OpencodePort != 4096 {
		t.Errorf("OpencodePort = %d, want %d", cfg.OpencodePort, 4096)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("STEAMPIPE_HOST", "myhost")
	t.Setenv("STEAMPIPE_PORT", "5432")
	t.Setenv("OPENCODE_PORT", "8080")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.SteampipeHost != "myhost" {
		t.Errorf("SteampipeHost = %q, want %q", cfg.SteampipeHost, "myhost")
	}
	if cfg.OpencodePort != 8080 {
		t.Errorf("OpencodePort = %d, want %d", cfg.OpencodePort, 8080)
	}
}

func TestValidate(t *testing.T) {
	// With OpenCode, Validate() always returns nil since the coding agent
	// manages its own LLM provider configuration.
	cfg := config.Config{}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
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

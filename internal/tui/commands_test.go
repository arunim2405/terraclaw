package tui

import (
	"strings"
	"testing"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// TestImportFallback_NoImportScript verifies that when import.sh is absent
// from the output directory, runImportCmd falls back to per-resource imports
// using GuessResourceAddress rather than attempting to run import.sh.
//
// Validates: Requirements 6.3
func TestImportFallback_NoImportScript(t *testing.T) {
	// Create a temp directory with no import.sh.
	tmpDir := t.TempDir()

	// Set appConfig to point to the temp directory.
	appConfig = &config.Config{
		OutputDir:    tmpDir,
		TerraformBin: "terraform",
	}
	defer func() { appConfig = nil }()

	// Create resource items with a real steampipe.Resource.
	resources := []ResourceItem{
		{
			Resource: steampipe.Resource{
				Provider: "aws",
				Type:     "aws_s3_bucket",
				Name:     "test-bucket",
				ID:       "test-bucket",
				Region:   "us-east-1",
			},
			Selected: true,
		},
	}

	// Call runImportCmd and execute the returned Cmd.
	cmd := runImportCmd(resources, "")
	msg := cmd()

	result, ok := msg.(asyncResultMsg)
	if !ok {
		t.Fatalf("expected asyncResultMsg, got %T", msg)
	}

	// The fallback path runs terraform init + terraform import via
	// GuessResourceAddress. Since terraform is not installed in the test
	// environment, terraform init will fail — but the key assertion is that
	// the result does NOT contain "import.sh output" (which would indicate
	// the import.sh path was taken).
	if strings.Contains(result.imports, "import.sh output") {
		t.Errorf("expected fallback path (no import.sh), but result contains import.sh output:\n%s", result.imports)
	}

	// The fallback path should produce some output (even if terraform init fails).
	if result.imports == "" && result.err == nil {
		t.Error("expected non-empty imports or error from fallback path, got neither")
	}
}

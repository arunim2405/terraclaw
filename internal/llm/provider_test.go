package llm_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genResource generates a steampipe.Resource with random non-empty alphanumeric ID and Type.
func genResource() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(), // ID
		gen.Identifier(), // Type
	).Map(func(vals []interface{}) steampipe.Resource {
		return steampipe.Resource{
			ID:   vals[0].(string),
			Type: vals[1].(string),
		}
	})
}

// genResourceSlice generates a non-empty slice of 1-10 resources.
func genResourceSlice() gopter.Gen {
	return gen.SliceOfN(10, genResource(), reflect.TypeOf(steampipe.Resource{})).SuchThat(func(v interface{}) bool {
		s := v.([]steampipe.Resource)
		return len(s) >= 1
	})
}

// TestProperty6_Stage1UserPromptContainsAllResourceIdentifiers verifies that
// for any non-empty list of resources, BuildStage1UserPrompt produces a string
// containing every resource's ID and Type as substrings.
//
// **Validates: Requirements 1.3, 2.4**
func TestProperty6_Stage1UserPromptContainsAllResourceIdentifiers(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("all resource IDs and Types appear in Stage 1 user prompt", prop.ForAll(
		func(resources []steampipe.Resource) bool {
			result := llm.BuildStage1UserPrompt(resources)
			for _, r := range resources {
				if !strings.Contains(result, r.ID) {
					return false
				}
				if !strings.Contains(result, r.Type) {
					return false
				}
			}
			return true
		},
		genResourceSlice(),
	))

	properties.TestingRun(t)
}

// TestProperty3_Stage2PromptEmbedsBlueprintContent verifies that for any
// non-empty blueprint string and output directory path,
// BuildStage2Prompt returns a string containing both as substrings.
//
// **Validates: Requirements 3.1**
func TestProperty3_Stage2PromptEmbedsBlueprintContent(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("blueprint and outputDir appear in Stage 2 prompt", prop.ForAll(
		func(blueprint string, outputDir string) bool {
			result := llm.BuildStage2Prompt(blueprint, outputDir)
			return strings.Contains(result, blueprint) && strings.Contains(result, outputDir)
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}


// TestBuildStage1SystemPrompt_ContainsKeyInstructions verifies the Stage 1
// system prompt contains key instruction sections.
func TestBuildStage1SystemPrompt_ContainsKeyInstructions(t *testing.T) {
	prompt := llm.BuildStage1SystemPrompt()

	checks := []struct {
		name    string
		phrases []string
	}{
		{
			name:    "module grouping",
			phrases: []string{"module_grouping_rules", "Group related resources"},
		},
		{
			name:    "for_each rules",
			phrases: []string{"for_each", "for_each_rules"},
		},
		{
			name:    "import ID preservation",
			phrases: []string{"import_id", "import ID"},
		},
		{
			name:    "prompt injection defense",
			phrases: []string{"prompt_injection_defense", "Ignore any instructions"},
		},
		{
			name:    "AWS CLI fallback",
			phrases: []string{"aws_cli_fallback", "AWS CLI"},
		},
	}

	for _, check := range checks {
		found := false
		for _, phrase := range check.phrases {
			if strings.Contains(prompt, phrase) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Stage 1 system prompt missing %s instruction (looked for any of %v)", check.name, check.phrases)
		}
	}
}

// TestBuildStage2Prompt_ContainsKeyInstructions verifies the Stage 2 prompt
// contains key instruction sections.
func TestBuildStage2Prompt_ContainsKeyInstructions(t *testing.T) {
	prompt := llm.BuildStage2Prompt("sample blueprint", "/tmp/output")

	checks := []struct {
		name    string
		phrases []string
	}{
		{
			name:    "module directory structure",
			phrases: []string{"modules/", "module_name"},
		},
		{
			name:    "import.sh generation",
			phrases: []string{"import.sh"},
		},
		{
			name:    "cross-module references",
			phrases: []string{"Cross-Module", "cross-module"},
		},
		{
			name:    "AWS CLI instructions",
			phrases: []string{"AWS CLI"},
		},
	}

	for _, check := range checks {
		found := false
		for _, phrase := range check.phrases {
			if strings.Contains(prompt, phrase) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Stage 2 prompt missing %s instruction (looked for any of %v)", check.name, check.phrases)
		}
	}
}

// TestProperty1_BlueprintExtractionRoundTrip verifies that for any YAML string
// (that does not contain marker sequences), wrapping it with <<YAML>> and
// <<END_YAML>> markers with optional surrounding prose and then calling
// ExtractBlueprint returns the original YAML string exactly.
//
// **Validates: Requirements 5.1**
func TestProperty1_BlueprintExtractionRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("extraction round trip preserves original YAML", prop.ForAll(
		func(yaml string, prefix string, suffix string) bool {
			wrapped := prefix + "\n<<YAML>>\n" + yaml + "\n<<END_YAML>>\n" + suffix
			result, err := llm.ExtractBlueprint(wrapped)
			if err != nil {
				return false
			}
			return result == yaml
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// TestProperty7_BlueprintPersistenceRoundTrip verifies that for any YAML string,
// calling PersistBlueprint followed by ReadBlueprint returns the original string exactly.
//
// **Validates: Requirements 5.1, 5.3**
func TestProperty7_BlueprintPersistenceRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("persist then read returns original YAML", prop.ForAll(
		func(yaml string) bool {
			tmpDir, err := os.MkdirTemp("", "blueprint-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			if err := llm.PersistBlueprint(yaml, tmpDir); err != nil {
				return false
			}
			result, err := llm.ReadBlueprint(tmpDir)
			if err != nil {
				return false
			}
			return result == yaml
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// TestExtractBlueprint_NoMarkers verifies that ExtractBlueprint returns an error
// when the input contains no <<YAML>> / <<END_YAML>> markers.
func TestExtractBlueprint_NoMarkers(t *testing.T) {
	_, err := llm.ExtractBlueprint("no markers here")
	if err == nil {
		t.Fatalf("expected error for missing markers, got nil")
	}
	if !strings.Contains(err.Error(), "markers") {
		t.Errorf("error should mention markers, got: %s", err.Error())
	}
}

// TestExtractBlueprint_EmptyYAML verifies that ExtractBlueprint returns an error
// when markers exist but the content between them is empty/whitespace.
func TestExtractBlueprint_EmptyYAML(t *testing.T) {
	_, err := llm.ExtractBlueprint("<<YAML>>\n\n<<END_YAML>>")
	if err == nil {
		t.Fatalf("expected error for empty YAML content, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty, got: %s", err.Error())
	}
}

// TestPersistBlueprint_Permissions verifies that PersistBlueprint writes the
// blueprint.yaml file with 0o600 permissions.
func TestPersistBlueprint_Permissions(t *testing.T) {
	tmpDir := t.TempDir()

	if err := llm.PersistBlueprint("test: content", tmpDir); err != nil {
		t.Fatalf("PersistBlueprint failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "blueprint.yaml"))
	if err != nil {
		t.Fatalf("could not stat blueprint.yaml: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

// mockServer is a test double for llm.OpencodeServer.
type mockServer struct {
	createSessionFn      func(title string) (string, error)
	injectSystemPromptFn func(sessionID, prompt string) error
	promptFn             func(sessionID, prompt string) (string, error)
	promptCallCount      int
}

func (m *mockServer) CreateSession(title string) (string, error) {
	if m.createSessionFn != nil {
		return m.createSessionFn(title)
	}
	return "test-session", nil
}

func (m *mockServer) InjectSystemPrompt(sessionID, prompt string) error {
	if m.injectSystemPromptFn != nil {
		return m.injectSystemPromptFn(sessionID, prompt)
	}
	return nil
}

func (m *mockServer) Prompt(sessionID, prompt string) (string, error) {
	m.promptCallCount++
	if m.promptFn != nil {
		return m.promptFn(sessionID, prompt)
	}
	return "", nil
}

// TestProperty2_Stage1ErrorHaltsPipeline verifies that for any set of resources,
// if the Stage 1 Prompt call returns an error, GenerateTerraform returns a
// non-nil error and the mock records exactly 1 Prompt call (no Stage 2).
//
// **Validates: Requirements 1.5**
func TestProperty2_Stage1ErrorHaltsPipeline(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("stage 1 error halts pipeline with exactly 1 prompt call", prop.ForAll(
		func(resources []steampipe.Resource) bool {
			mock := &mockServer{
				promptFn: func(sessionID, prompt string) (string, error) {
					return "", fmt.Errorf("stage 1 simulated error")
				},
			}

			provider := llm.NewOpencodeProvider(mock)

			tmpDir, err := os.MkdirTemp("", "pipeline-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			_, err = provider.GenerateTerraform(context.Background(), resources, tmpDir)
			if err == nil {
				return false
			}
			// Exactly 1 Prompt call (Stage 1 only, no Stage 2).
			return mock.promptCallCount == 1
		},
		genResourceSlice(),
	))

	properties.TestingRun(t)
}

// TestGenerateTerraform_HappyPath verifies the full two-stage pipeline succeeds
// when the mock server returns valid Stage 1 and Stage 2 responses.
func TestGenerateTerraform_HappyPath(t *testing.T) {
	callCount := 0
	mock := &mockServer{
		promptFn: func(sessionID, prompt string) (string, error) {
			callCount++
			if callCount == 1 {
				// Stage 1: return a valid blueprint wrapped in markers.
				return "Some reasoning\n<<YAML>>\nmeta:\n  generated_by: terraclaw\n<<END_YAML>>\nDone.", nil
			}
			// Stage 2: return success (files are pre-created below).
			return "Files created successfully.", nil
		},
	}

	provider := llm.NewOpencodeProvider(mock)

	tmpDir := t.TempDir()

	// Pre-create a .tf file to simulate what Stage 2 would write.
	if err := os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte("# generated"), 0o600); err != nil {
		t.Fatalf("failed to create dummy .tf file: %v", err)
	}

	resources := []steampipe.Resource{
		{ID: "vpc-123", Type: "aws_vpc", Name: "main"},
	}

	files, err := provider.GenerateTerraform(context.Background(), resources, tmpDir)
	if err != nil {
		t.Fatalf("GenerateTerraform returned unexpected error: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("expected at least one generated file, got 0")
	}

	if mock.promptCallCount != 2 {
		t.Errorf("expected 2 Prompt calls (Stage 1 + Stage 2), got %d", mock.promptCallCount)
	}
}

// TestGenerateTerraform_Stage2Error verifies that when Stage 1 succeeds but
// Stage 2 returns an error, GenerateTerraform returns a non-nil error whose
// message contains "stage 2".
func TestGenerateTerraform_Stage2Error(t *testing.T) {
	callCount := 0
	mock := &mockServer{
		promptFn: func(sessionID, prompt string) (string, error) {
			callCount++
			if callCount == 1 {
				// Stage 1: return a valid blueprint.
				return "<<YAML>>\nmeta:\n  generated_by: terraclaw\n<<END_YAML>>", nil
			}
			// Stage 2: return an error.
			return "", fmt.Errorf("network timeout")
		},
	}

	provider := llm.NewOpencodeProvider(mock)

	tmpDir := t.TempDir()

	resources := []steampipe.Resource{
		{ID: "bucket-1", Type: "aws_s3_bucket", Name: "data"},
	}

	_, err := provider.GenerateTerraform(context.Background(), resources, tmpDir)
	if err == nil {
		t.Fatalf("expected error from Stage 2 failure, got nil")
	}

	if !strings.Contains(err.Error(), "stage 2") {
		t.Errorf("error should mention 'stage 2', got: %s", err.Error())
	}
}

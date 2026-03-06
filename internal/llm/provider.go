// Package llm provides Terraform code generation through the OpenCode coding agent.
package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// GeneratedFile represents a Terraform file created by OpenCode.
type GeneratedFile struct {
	Path    string // absolute path on disk
	Name    string // basename (e.g. "main.tf")
	Content string // file content
}

// Provider is the interface for Terraform code generation.
type Provider interface {
	GenerateTerraform(ctx context.Context, resources []steampipe.Resource, outputDir string) ([]GeneratedFile, error)
	Name() string
}

// OpencodeProvider generates Terraform code via the OpenCode coding agent.
type OpencodeProvider struct {
	server *opencode.Server
}

// NewOpencodeProvider creates a provider that delegates to an OpenCode server.
func NewOpencodeProvider(server *opencode.Server) *OpencodeProvider {
	return &OpencodeProvider{server: server}
}

// Name returns the provider name.
func (p *OpencodeProvider) Name() string { return "OpenCode" }

// GenerateTerraform creates a session, injects the system prompt, sends the
// resource prompt, and waits for OpenCode to write .tf files to the output
// directory. Returns the list of generated files.
func (p *OpencodeProvider) GenerateTerraform(ctx context.Context, resources []steampipe.Resource, outputDir string) ([]GeneratedFile, error) {
	debuglog.Log("[opencode-provider] generating terraform for %d resource(s) in %s", len(resources), outputDir)

	// Ensure output directory exists.
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// 1. Create a session named terraclaw-terraform-generation
	sessionID, err := p.server.CreateSession("terraclaw-terraform-generation")
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	debuglog.Log("[opencode-provider] session created: %s", sessionID)

	// 2. Inject the system prompt (Terraform generation instructions).
	if err := p.server.InjectSystemPrompt(sessionID, BuildSystemPrompt(outputDir)); err != nil {
		return nil, fmt.Errorf("inject system prompt: %w", err)
	}

	// 3. Send the resource prompt and wait for OpenCode to finish.
	userPrompt := buildPrompt(resources, outputDir)
	debuglog.Log("[opencode-provider] sending resource prompt (%d bytes)", len(userPrompt))

	response, err := p.server.Prompt(sessionID, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("prompt: %w", err)
	}

	debuglog.Log("[opencode-provider] received response (%d bytes)", len(response))

	// 4. Scan the output directory for generated .tf files.
	files, err := ListGeneratedFiles(outputDir)
	if err != nil {
		return nil, fmt.Errorf("list generated files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("opencode did not create any .tf files in %s", outputDir)
	}

	debuglog.Log("[opencode-provider] found %d generated file(s)", len(files))
	return files, nil
}

// ListGeneratedFiles scans a directory for .tf files and reads their content.
func ListGeneratedFiles(dir string) ([]GeneratedFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []GeneratedFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".tf") {
			continue
		}
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			debuglog.Log("[opencode-provider] warning: could not read %s: %v", path, err)
			continue
		}
		files = append(files, GeneratedFile{
			Path:    path,
			Name:    name,
			Content: string(content),
		})
	}
	return files, nil
}

// BuildSystemPrompt returns the system-level instruction for OpenCode.
// It tells OpenCode to create Terraform files directly in the output directory
// following best practices with modules, variables, and proper file structure.
func BuildSystemPrompt(outputDir string) string {
	return fmt.Sprintf(`You are a Terraform expert and senior DevOps engineer. Your job is to generate production-quality, modular Terraform code and WRITE IT TO FILES in the directory: %s

## CRITICAL: You MUST create the files yourself using your file-writing tools. Do NOT just return code as text.

## CRITICAL: Do NOT ask the user any follow-up questions. Make all decisions autonomously as an expert DevOps and Terraform engineer. If you are unsure about a value, use the best default based on the resource configuration provided. Proceed immediately with file creation.

## CRITICAL: Match Existing Infrastructure EXACTLY
The resources provided include their FULL current configuration from the cloud. You MUST:
- Reproduce ALL nested configuration blocks exactly as they exist (e.g. schema blocks, lambda_config, password_policy, account_recovery_setting, etc.)
- Include ALL attributes that are explicitly set on the resource, not just required ones
- For JSON-structured properties, convert them to the corresponding Terraform block syntax
- Pay special attention to: lists, maps, nested blocks, and boolean values
- Do NOT add attributes that aren't in the existing config (this would cause drift)
- When both username_attributes and alias_attributes appear, use ONLY the one that has actual values. Never set both, as they conflict.

## File Structure — Create separate files, take files below as suggestion but not a hard requirement:

1. **providers.tf** — terraform {}, required_providers, provider config with region and default_tags
2. **variables.tf** — All input variables with type, description, and sensible defaults
3. **main.tf** — Primary resource definitions. Use public Terraform modules where appropriate (e.g. terraform-aws-modules/vpc/aws, terraform-aws-modules/ec2-instance/aws, etc.)
4. **outputs.tf** — Output blocks for key resource attributes (IDs, ARNs, endpoints)
5. **data.tf** — Data sources (if needed for lookups like AMIs, availability zones, etc.)
6. **locals.tf** — Local values for computed values, common tags, naming conventions

Only create files that are needed. If no data sources are required, skip data.tf.

## Code Quality Requirements

### Use Public Terraform Modules
- Prefer official HashiCorp and community modules from the Terraform Registry
- Examples: terraform-aws-modules/vpc/aws, terraform-aws-modules/s3-bucket/aws, terraform-aws-modules/iam/aws
- Only create raw resources when no suitable module exists

### Variables & Reusability
- Every configurable value should be a variable (region, instance type, CIDR blocks, names, etc.)
- Variables must have: type, description, and a sensible default where appropriate
- Use variable validation blocks where it makes sense

### Naming & Tagging
- Lowercase with underscores for all Terraform names
- Descriptive names excluding the resource type prefix

### Security Best Practices
- Enable encryption at rest by default
- Configure private networking where applicable
- Apply least-privilege for security groups and IAM
- Never hardcode credentials or secrets
- Mark sensitive outputs with sensitive = true

### Code Formatting
- Two spaces per nesting level (no tabs)
- Align equals signs for consecutive arguments
- Meta-arguments (count, for_each, depends_on, lifecycle) first
- Blank line between blocks

### Import Support
- Add a comment above each resource block with the terraform import command
- Format: # terraform import <resource_type>.<resource_name> <id>

## After Creating Files
After writing all the .tf files, reply with a brief summary listing the files you created and what each contains. Code should be formatted with Code formatted with terraform fmt and 
`, outputDir)
}

// buildPrompt constructs the user-facing prompt with resource details.
func buildPrompt(resources []steampipe.Resource, outputDir string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Generate Terraform configuration to manage the following %d existing cloud resources.\n\n", len(resources)))
	sb.WriteString(fmt.Sprintf("IMPORTANT: Write the .tf files directly to the directory: %s\n", outputDir))
	sb.WriteString("Use public Terraform modules where appropriate. Create proper variables for all configurable values.\n")
	sb.WriteString("Cross-reference related resources using Terraform references (e.g. module.vpc.vpc_id) instead of hardcoding IDs.\n\n")

	sb.WriteString("Resources to manage:\n\n")

	for i, r := range resources {
		sb.WriteString(fmt.Sprintf("Resource %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  Provider: %s\n", r.Provider))
		sb.WriteString(fmt.Sprintf("  Type: %s\n", r.Type))
		sb.WriteString(fmt.Sprintf("  Name: %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("  ID: %s\n", r.ID))
		if r.Region != "" {
			sb.WriteString(fmt.Sprintf("  Region: %s\n", r.Region))
		}
		sb.WriteString("  Configuration:\n")
		writeResourceProperties(&sb, r.Properties)
		sb.WriteString("\n")
	}

	sb.WriteString("\nCreate the following files:\n")
	sb.WriteString("- providers.tf (terraform block + provider config)\n")
	sb.WriteString("- variables.tf (all input variables)\n")
	sb.WriteString("- main.tf (resources, using modules where appropriate)\n")
	sb.WriteString("- outputs.tf (key outputs)\n")
	sb.WriteString("- Additional files only if needed (data.tf, locals.tf)\n\n")
	sb.WriteString("Include terraform import commands as comments above each resource/module block.\n")
	sb.WriteString("Write the files now.\n")
	return sb.String()
}

// steampipeMetaColumns are Steampipe-internal columns that are useless for Terraform generation.
var steampipeMetaColumns = map[string]bool{
	"sp_ctx":             true,
	"_ctx":               true,
	"sp_connection_name": true,
	"akas":               true,
	"partition":          true,
	"account_id":         true,
	"title":              true, // duplicate of name
}

// writeResourceProperties writes sorted, filtered resource properties to the builder.
// No character limit is applied — all property values are included in full.
func writeResourceProperties(sb *strings.Builder, props map[string]string) {
	// Sort keys for consistent output.
	keys := make([]string, 0, len(props))
	for k := range props {
		if steampipeMetaColumns[k] {
			continue
		}
		keys = append(keys, k)
	}
	sortStrings(keys)

	for _, k := range keys {
		v := props[k]
		if v == "" || v == "[]" || v == "{}" || v == "<nil>" || v == "null" {
			continue
		}
		sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
	}
}

// sortStrings sorts a slice of strings in place (simple insertion sort to avoid importing sort).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

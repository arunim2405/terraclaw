package terraform

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// ImportResult holds the result of a terraform import command.
type ImportResult struct {
	Resource steampipe.Resource
	Address  string
	Output   string
	Error    error
}

// RunImport executes `terraform import` for a single resource.
func RunImport(terraformBin, workDir string, resource steampipe.Resource, address string) ImportResult {
	result := ImportResult{Resource: resource, Address: address}

	cmd := exec.Command(terraformBin, "import", address, resource.ID) // #nosec G204
	cmd.Dir = workDir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		result.Error = fmt.Errorf("terraform import failed: %w\n%s", err, out.String())
	}
	result.Output = out.String()
	return result
}

// RunInit executes `terraform init` in the working directory.
// This must be called before running terraform import.
func RunInit(terraformBin, workDir string) error {
	cmd := exec.Command(terraformBin, "init") // #nosec G204
	cmd.Dir = workDir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform init failed: %w\n%s", err, out.String())
	}
	return nil
}

// RunImports runs `terraform init` first, then executes `terraform import`
// for each resource in the list.
func RunImports(terraformBin, workDir string, resources []steampipe.Resource) []ImportResult {
	results := make([]ImportResult, 0, len(resources))

	// Run terraform init before importing.
	if err := RunInit(terraformBin, workDir); err != nil {
		// Return a single error result if init fails.
		return []ImportResult{{
			Error: fmt.Errorf("terraform init failed (imports skipped): %w", err),
		}}
	}

	for _, r := range resources {
		addr := GuessResourceAddress(r)
		results = append(results, RunImport(terraformBin, workDir, r, addr))
	}
	return results
}

// SummaryText returns a human-readable summary of import results.
func SummaryText(results []ImportResult) string {
	var sb strings.Builder
	passed, failed := 0, 0
	for _, r := range results {
		if r.Error != nil {
			failed++
			sb.WriteString(fmt.Sprintf("✗ %s: %v\n", r.Address, r.Error))
		} else {
			passed++
			sb.WriteString(fmt.Sprintf("✓ %s\n", r.Address))
		}
	}
	sb.WriteString(fmt.Sprintf("\n%d imported, %d failed\n", passed, failed))
	return sb.String()
}

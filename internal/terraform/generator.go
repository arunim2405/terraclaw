// Package terraform provides utilities for generating and importing Terraform configurations.
package terraform

import (
	"fmt"
	"strings"

	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// NOTE: WriteConfig removed — OpenCode now creates .tf files directly in the output directory.
// Use llm.ListGeneratedFiles(dir) to scan for generated files after OpenCode completes.

// ImportCommand returns the terraform import command for a given resource.
func ImportCommand(terraformBin string, resource steampipe.Resource, resourceAddr string) string {
	return fmt.Sprintf("%s import %s %s", terraformBin, resourceAddr, resource.ID)
}

// GuessResourceAddress attempts to derive a Terraform resource address from the
// Steampipe resource metadata. This is a best-effort helper; users may need to
// adjust the address manually.
func GuessResourceAddress(r steampipe.Resource) string {
	// Clean the resource type name: remove provider prefix if present.
	resourceType := r.Type
	if idx := strings.Index(resourceType, "_"); idx != -1 {
		// e.g. "aws_s3_bucket" stays as-is, but "s3_bucket" becomes "aws_s3_bucket" if provider is aws
		if !strings.HasPrefix(resourceType, r.Provider+"_") {
			resourceType = r.Provider + "_" + resourceType
		}
	}

	// Sanitize the resource name.
	name := r.Name
	if name == "" {
		name = r.ID
	}
	name = sanitizeIdentifier(name)

	return fmt.Sprintf("%s.%s", resourceType, name)
}

// sanitizeIdentifier makes a string safe to use as a Terraform identifier.
func sanitizeIdentifier(s string) string {
	var b strings.Builder
	for i, ch := range s {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch == '_':
			b.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			if i == 0 {
				b.WriteRune('_')
			}
			b.WriteRune(ch)
		default:
			b.WriteRune('_')
		}
	}
	result := b.String()
	if result == "" {
		return "resource"
	}
	return result
}

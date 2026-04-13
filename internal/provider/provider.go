// Package provider defines cloud provider detection and provider-specific constants.
package provider

import "strings"

// Cloud represents a supported cloud provider.
type Cloud string

const (
	AWS     Cloud = "aws"
	Azure   Cloud = "azure"
	Unknown Cloud = "unknown"
)

// DetectFromSchema returns the Cloud provider for a Steampipe schema name.
// Steampipe schemas are named after the plugin: "aws", "azure", "azuread", etc.
func DetectFromSchema(schema string) Cloud {
	s := strings.ToLower(schema)
	switch {
	case s == "aws":
		return AWS
	case s == "azure" || s == "azuread" || strings.HasPrefix(s, "azure"):
		return Azure
	default:
		return Unknown
	}
}

// DetectFromResourceID inspects a resource identifier and returns the likely provider.
// AWS ARNs start with "arn:", Azure resource IDs start with "/subscriptions/".
func DetectFromResourceID(id string) Cloud {
	switch {
	case strings.HasPrefix(id, "arn:"):
		return AWS
	case strings.HasPrefix(id, "/subscriptions/"):
		return Azure
	default:
		return Unknown
	}
}

// CLIName returns the CLI tool name used for this provider (aws, az).
func (c Cloud) CLIName() string {
	switch c {
	case AWS:
		return "aws"
	case Azure:
		return "az"
	default:
		return ""
	}
}

// TerraformProviderSource returns the HashiCorp provider source string.
func (c Cloud) TerraformProviderSource() string {
	switch c {
	case AWS:
		return "hashicorp/aws"
	case Azure:
		return "hashicorp/azurerm"
	default:
		return ""
	}
}

// String returns the lowercase provider name.
func (c Cloud) String() string {
	return string(c)
}

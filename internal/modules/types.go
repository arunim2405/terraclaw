// Package modules provides user-registered Terraform module management:
// scanning, storage, matching, and fit scoring.
package modules

import "time"

// ModuleMetadata holds the parsed metadata for a user-registered Terraform module.
type ModuleMetadata struct {
	ID            int64
	Name          string         // derived from directory basename or git path
	Source        string         // original source (git URL or local path)
	Description   string         // from module README first paragraph
	ProviderType  string         // "aws", "azure", or "" if multi/unknown
	ResourceTypes []string       // terraform resource types managed (e.g. "aws_vpc")
	DataSources   []string       // data sources used
	Variables     []VariableMeta // parsed from variables.tf
	Outputs       []OutputMeta   // parsed from outputs.tf
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// RequiredInputs returns the variable names that have no default value.
func (m ModuleMetadata) RequiredInputs() []string {
	var required []string
	for _, v := range m.Variables {
		if v.Required {
			required = append(required, v.Name)
		}
	}
	return required
}

// VariableMeta describes a single input variable of a module.
type VariableMeta struct {
	Name        string `json:"name"`
	Type        string `json:"type"`        // HCL type expression as string
	Description string `json:"description"`
	Default     string `json:"default"`     // JSON-encoded default value, empty if required
	Required    bool   `json:"required"`
}

// OutputMeta describes a single output of a module.
type OutputMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// FitResult represents how well a module matches a set of target resources.
type FitResult struct {
	Module           ModuleMetadata
	Score            float64  // 0.0 to 1.0 weighted total
	CoverageScore    float64  // fraction of target resources the module covers
	SpecificityScore float64  // fraction of module resources in the target set
	VarReadiness     float64  // fraction of vars with defaults or property matches
	MatchedTypes     []string // which target resource types this module covers
	UnmatchedModule  []string // module resource types NOT in the target set
	MissingInputs    []string // required vars we cannot auto-populate
	Selected         bool     // whether the user has toggled this module on
}

// ScorePercent returns the fit score as a rounded integer percentage.
func (f FitResult) ScorePercent() int {
	return int(f.Score*100 + 0.5)
}

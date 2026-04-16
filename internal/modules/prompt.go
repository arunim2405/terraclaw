package modules

import (
	"fmt"
	"strings"
)

// BuildModuleCatalogPrompt constructs a prompt section that constrains the LLM
// to use the user's selected modules as hard constraints during blueprint
// generation. Returns empty string if no modules are provided.
func BuildModuleCatalogPrompt(selected []FitResult) string {
	if len(selected) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(`<user_module_constraints>
The user has registered the following Terraform modules as HARD CONSTRAINTS.
When the target resources include types managed by these modules, you MUST use
the user's module instead of any registry module from the catalog.

Use the user module's source path in the blueprint's "source" field and map
the resource properties to the module's input variables.

`)

	for _, fit := range selected {
		m := fit.Module
		sb.WriteString(fmt.Sprintf("Module: %s\n", m.Name))
		sb.WriteString(fmt.Sprintf("  Source: %s\n", m.Source))
		sb.WriteString(fmt.Sprintf("  Provider: %s\n", m.ProviderType))
		sb.WriteString(fmt.Sprintf("  Manages: %s\n", strings.Join(m.ResourceTypes, ", ")))
		sb.WriteString(fmt.Sprintf("  Fit score: %d%%\n", fit.ScorePercent()))

		if len(m.Variables) > 0 {
			sb.WriteString("  Variables:\n")
			for _, v := range m.Variables {
				req := ""
				if v.Required {
					req = ", required"
				}
				def := ""
				if v.Default != "" {
					def = fmt.Sprintf(", default: %s", v.Default)
				}
				desc := ""
				if v.Description != "" {
					desc = fmt.Sprintf(" — %s", v.Description)
				}
				sb.WriteString(fmt.Sprintf("    - %s (%s%s%s)%s\n", v.Name, v.Type, req, def, desc))
			}
		}

		if len(m.Outputs) > 0 {
			sb.WriteString(fmt.Sprintf("  Outputs: %s\n", joinOutputNames(m.Outputs)))
		}

		sb.WriteString("\n")
	}

	sb.WriteString(`IMPORTANT: User modules take ABSOLUTE PRIORITY over registry modules.
If a user module covers a resource type, do NOT use the registry module for
that type. Instead, use the user module's source path and map inputs from
the resource properties to the module's variables.

For resources NOT covered by any user module, fall back to the registry
module catalog as usual.
</user_module_constraints>`)

	return sb.String()
}

func joinOutputNames(outputs []OutputMeta) string {
	names := make([]string, len(outputs))
	for i, o := range outputs {
		names[i] = o.Name
	}
	return strings.Join(names, ", ")
}

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/graph"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	tf "github.com/arunim2405/terraclaw/internal/terraform"
)

// appConfig is set once at startup so commands can access it.
var appConfig *config.Config

// steampipeClient is the shared Steampipe client used by commands.
var steampipeClient *steampipe.Client

// SetConfig stores the application config for use by TUI commands.
func SetConfig(cfg *config.Config) { appConfig = cfg }

// SetSteampipeClient stores the Steampipe client for use by TUI commands.
func SetSteampipeClient(c *steampipe.Client) { steampipeClient = c }

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// spinnerTickMsg triggers a spinner frame update.
type spinnerTickMsg struct{}

// scanProgressMsg reports scan progress.
type scanProgressMsg struct {
	message string
}

// graphBuiltMsg carries the completed resource graph.
type graphBuiltMsg struct {
	graph *graph.Graph
	err   error
}

// asyncResultMsg carries the result of an async code generation or import.
type asyncResultMsg asyncResult

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// tickCmd returns a command that sends a spinnerTickMsg after a short delay.
func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// scanResourcesCmd scans cloud resources and builds the dependency graph.
func scanResourcesCmd(schema string, scanMode string) tea.Cmd {
	return func() tea.Msg {
		if steampipeClient == nil {
			return graphBuiltMsg{err: fmt.Errorf("steampipe client not initialized")}
		}

		// Determine which tables to scan.
		var tables []string
		switch scanMode {
		case "all":
			debuglog.Log("[graph] scanning ALL tables for schema=%s", schema)
			var err error
			tables, err = steampipeClient.ListTables(schema)
			if err != nil {
				return graphBuiltMsg{err: fmt.Errorf("list tables: %w", err)}
			}
		default: // "key"
			// Use configured tables if set, else default AWS tables.
			if appConfig != nil && appConfig.ScanTables != "" && appConfig.ScanTables != "*" {
				tables = strings.Split(appConfig.ScanTables, ",")
				for i := range tables {
					tables[i] = strings.TrimSpace(tables[i])
				}
			} else {
				tables = graph.DefaultAWSTables
			}
			debuglog.Log("[graph] scanning %d key tables for schema=%s", len(tables), schema)
		}

		g := graph.New()
		err := g.Build(steampipeClient, schema, tables, func(scanned, total int, table string) {
			debuglog.Log("[graph] progress: %d/%d — %s", scanned, total, table)
			// Note: we can't send tea.Msg from inside a callback directly,
			// but the spinner tick will keep the UI responsive.
		})
		if err != nil {
			return graphBuiltMsg{err: err}
		}

		// Detect relationships between resources.
		g.DetectRelationships()
		debuglog.Log("[graph] build complete: %d nodes, %d edges", g.Stats.ResourceCount, g.Stats.EdgeCount)

		return graphBuiltMsg{graph: g}
	}
}

// generateCodeCmd calls the selected LLM to generate Terraform code.
func generateCodeCmd(providerName string, resources []ResourceItem) tea.Cmd {
	return func() tea.Msg {
		debuglog.Log("[llm] generateCode called: providerName=%q resources=%d", providerName, len(resources))
		if appConfig == nil {
			debuglog.Log("[llm] ERROR: config not initialized")
			return asyncResultMsg{err: fmt.Errorf("config not initialized")}
		}

		// Map provider display name back to config provider.
		switch {
		case strings.Contains(providerName, "Azure"):
			appConfig.LLMProvider = config.ProviderAzureOpenAI
		case strings.Contains(providerName, "OpenAI"):
			appConfig.LLMProvider = config.ProviderOpenAI
		case strings.Contains(providerName, "Anthropic"):
			appConfig.LLMProvider = config.ProviderClaude
		case strings.Contains(providerName, "Google"):
			appConfig.LLMProvider = config.ProviderGemini
		}
		debuglog.Log("[llm] resolved provider: %s", appConfig.LLMProvider)

		provider, err := llm.New(appConfig)
		if err != nil {
			debuglog.Log("[llm] ERROR creating provider: %v", err)
			return asyncResultMsg{err: err}
		}

		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		debuglog.Log("[llm] calling %s API with %d resource(s)", provider.Name(), len(raw))
		code, err := provider.GenerateTerraform(context.Background(), raw)
		if err != nil {
			debuglog.Log("[llm] ERROR from %s: %v", provider.Name(), err)
			return asyncResultMsg{err: err}
		}
		debuglog.Log("[llm] %s response received: %d chars", provider.Name(), len(code))

		// Also write the code to disk.
		if appConfig != nil {
			outPath, writeErr := tf.WriteConfig(appConfig.OutputDir, code)
			if writeErr != nil {
				debuglog.Log("[terraform] ERROR writing config: %v", writeErr)
			} else {
				debuglog.Log("[terraform] config written to %s", outPath)
			}
		}

		return asyncResultMsg{code: code}
	}
}

// runImportCmd runs terraform import for the selected resources.
func runImportCmd(resources []ResourceItem, _ string) tea.Cmd {
	return func() tea.Msg {
		debuglog.Log("[terraform] runImport called for %d resource(s)", len(resources))
		if appConfig == nil {
			debuglog.Log("[terraform] ERROR: config not initialized")
			return asyncResultMsg{err: fmt.Errorf("config not initialized")}
		}

		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		debuglog.Log("[terraform] running import with bin=%s outputDir=%s", appConfig.TerraformBin, appConfig.OutputDir)
		results := tf.RunImports(appConfig.TerraformBin, appConfig.OutputDir, raw)
		summary := tf.SummaryText(results)
		debuglog.Log("[terraform] import complete: %s", summary)
		return asyncResultMsg{imports: summary}
	}
}

// ---------------------------------------------------------------------------
// Model update additions for loaded data
// ---------------------------------------------------------------------------

// updateForMessages extends Model.Update to handle data-load messages.
func (m Model) updateForMessages(msg tea.Msg) (Model, tea.Cmd, bool) {
	// No table/resource loading messages needed anymore — the graph handles it.
	_ = msg
	return m, nil, false
}

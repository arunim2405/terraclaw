package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/graph"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	tf "github.com/arunim2405/terraclaw/internal/terraform"
)

// appConfig is set once at startup so commands can access it.
var appConfig *config.Config

// steampipeClient is the shared Steampipe client used by commands.
var steampipeClient *steampipe.Client

// opencodeServer is the shared OpenCode server used by commands.
var opencodeServer *opencode.Server

// SetConfig stores the application config for use by TUI commands.
func SetConfig(cfg *config.Config) { appConfig = cfg }

// SetSteampipeClient stores the Steampipe client for use by TUI commands.
func SetSteampipeClient(c *steampipe.Client) { steampipeClient = c }

// SetOpencodeServer stores the OpenCode server for use by TUI commands.
func SetOpencodeServer(s *opencode.Server) { opencodeServer = s }

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
				tables = splitAndTrim(appConfig.ScanTables, ",")
			} else {
				tables = graph.DefaultAWSTables
			}
			debuglog.Log("[graph] scanning %d key tables for schema=%s", len(tables), schema)
		}

		g := graph.New()
		err := g.Build(steampipeClient, schema, tables, func(scanned, total int, table string) {
			debuglog.Log("[graph] progress: %d/%d — %s", scanned, total, table)
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

// generateCodeCmd calls OpenCode to generate Terraform files directly.
func generateCodeCmd(resources []ResourceItem) tea.Cmd {
	return func() tea.Msg {
		debuglog.Log("[opencode] generateCode called: resources=%d", len(resources))
		if opencodeServer == nil {
			debuglog.Log("[opencode] ERROR: server not initialized")
			return asyncResultMsg{err: fmt.Errorf("opencode server not initialized")}
		}
		if appConfig == nil {
			return asyncResultMsg{err: fmt.Errorf("config not initialized")}
		}

		provider := llm.NewOpencodeProvider(opencodeServer)

		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		debuglog.Log("[opencode] calling OpenCode with %d resource(s), outputDir=%s", len(raw), appConfig.OutputDir)
		files, err := provider.GenerateTerraform(context.Background(), raw, appConfig.OutputDir)
		if err != nil {
			debuglog.Log("[opencode] ERROR: %v", err)
			return asyncResultMsg{err: err}
		}
		debuglog.Log("[opencode] generated %d file(s)", len(files))

		return asyncResultMsg{files: files}
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
// Helpers
// ---------------------------------------------------------------------------

// splitAndTrim splits a string by sep and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	for {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// updateForMessages extends Model.Update to handle data-load messages.
func (m Model) updateForMessages(msg tea.Msg) (Model, tea.Cmd, bool) {
	_ = msg
	return m, nil, false
}
